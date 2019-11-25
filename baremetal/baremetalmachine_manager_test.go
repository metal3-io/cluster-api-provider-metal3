package baremetal

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	bmoapis "github.com/metal3-io/baremetal-operator/pkg/apis"
	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientfake "k8s.io/client-go/kubernetes/fake"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/klogr"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testImageURL           = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2"
	testImageChecksumURL   = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum"
	testUserDataSecretName = "worker-user-data"
)

var ProviderID = "metal3://12345ID6789"
var bmmSpec = &capbm.BareMetalMachineSpec{
	ProviderID: &ProviderID,
}

var bmmSecret = &capbm.BareMetalMachineSpec{
	UserData: &corev1.SecretReference{
		Name:      "mybmmachine",
		Namespace: "myns",
	},
}

var consumerRef = &corev1.ObjectReference{
	Name:       "mybmmachine",
	Namespace:  "myns",
	Kind:       "BMMachine",
	APIVersion: capbm.GroupVersion.String(),
}

var consumerRefSome = &corev1.ObjectReference{
	Name:       "someoneelsesmachine",
	Namespace:  "myns",
	Kind:       "BMMachine",
	APIVersion: capi.GroupVersion.String(),
}

var bmhSpec = &bmh.BareMetalHostSpec{
	ConsumerRef: consumerRef,
	Image: &bmh.Image{
		URL: "myimage",
	},
}

var bmhSpecSomeImg = &bmh.BareMetalHostSpec{
	ConsumerRef: consumerRefSome,
	Image: &bmh.Image{
		URL: "someoneelsesimage",
	},
}

var bmhSpecNoImg = &bmh.BareMetalHostSpec{
	ConsumerRef: consumerRef,
}

var bmmObjectMetaWithValidAnnotations = &metav1.ObjectMeta{
	Name:            "mybmmachine",
	Namespace:       "myns",
	OwnerReferences: []metav1.OwnerReference{},
	Annotations: map[string]string{
		HostAnnotation: "myns/myhost",
	},
}

var bmmObjectMetaWithInvalidAnnotations = &metav1.ObjectMeta{
	Name:            "bmmachine",
	Namespace:       "myns",
	OwnerReferences: []metav1.OwnerReference{},
	Annotations: map[string]string{
		HostAnnotation: "myns/wrongvalue",
	},
}

var bmmObjectMetaWithSomeAnnotations = &metav1.ObjectMeta{
	Name:            "bmmachine",
	Namespace:       "myns",
	OwnerReferences: []metav1.OwnerReference{},
	Annotations: map[string]string{
		HostAnnotation: "myns/somehost",
	},
}

var bmmObjectMetaEmptyAnnotations = &metav1.ObjectMeta{
	Name:            "bmmachine",
	Namespace:       "myns",
	OwnerReferences: []metav1.OwnerReference{},
	Annotations:     map[string]string{},
}

var bmmObjectMetaNoAnnotations = &metav1.ObjectMeta{
	Name:            "bmmachine",
	Namespace:       "myns",
	OwnerReferences: []metav1.OwnerReference{},
}

var bmhPowerStatus = &bmh.BareMetalHostStatus{
	Provisioning: bmh.ProvisionStatus{
		State: bmh.StateNone,
	},
	PoweredOn: true,
}

var bmhStatus = &bmh.BareMetalHostStatus{
	Provisioning: bmh.ProvisionStatus{
		State: bmh.StateNone,
	},
}

func TestChooseHost(t *testing.T) {
	scheme := runtime.NewScheme()
	err := bmoapis.AddToScheme(scheme)

	if err != nil {
		log.Printf("AddToScheme failed: %v", err)
	}

	host1 := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host1",
			Namespace: "myns",
		},
		Spec: bmh.BareMetalHostSpec{
			ConsumerRef: &corev1.ObjectReference{
				Name:       "someothermachine",
				Namespace:  "myns",
				Kind:       "BMMachine",
				APIVersion: capbm.GroupVersion.String(),
			},
		},
	}
	host2 := *newBareMetalHost("myhost", nil, bmh.StateNone, nil, false)

	host3 := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host3",
			Namespace: "myns",
		},
		Spec: bmh.BareMetalHostSpec{
			ConsumerRef: &corev1.ObjectReference{
				Name:       "machine1",
				Namespace:  "myns",
				Kind:       "BMMachine",
				APIVersion: capbm.GroupVersion.String(),
			},
		},
	}
	host4 := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host4",
			Namespace: "someotherns",
		},
	}
	discoveredHost := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "discoveredHost",
			Namespace: "myns",
		},
		Status: bmh.BareMetalHostStatus{
			ErrorMessage: "this host is discovered and not usable",
		},
	}
	hostWithLabel := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithLabel",
			Namespace: "myns",
			Labels:    map[string]string{"key1": "value1"},
		},
	}

	bmmconfig, infrastructureRef := newConfig(t, "", map[string]string{},
		[]capbm.HostSelectorRequirement{})
	bmmconfig2, infrastructureRef2 := newConfig(t, "", map[string]string{"key1": "value1"},
		[]capbm.HostSelectorRequirement{})
	bmmconfig3, infrastructureRef3 := newConfig(t, "", map[string]string{"boguskey": "value"},
		[]capbm.HostSelectorRequirement{})
	bmmconfig4, infrastructureRef4 := newConfig(t, "", map[string]string{},
		[]capbm.HostSelectorRequirement{
			capbm.HostSelectorRequirement{
				Key:      "key1",
				Operator: "in",
				Values:   []string{"abc", "value1", "123"},
			},
		})
	bmmconfig5, infrastructureRef5 := newConfig(t, "", map[string]string{},
		[]capbm.HostSelectorRequirement{
			capbm.HostSelectorRequirement{
				Key:      "key1",
				Operator: "pancakes",
				Values:   []string{"abc", "value1", "123"},
			},
		})

	testCases := map[string]struct {
		Machine          *capi.Machine
		Hosts            []runtime.Object
		BMMachine        *capbm.BareMetalMachine
		ExpectedHostName string
	}{
		"Pick host2 which lacks ConsumerRef": {
			Machine:          newMachine("machine1", "", infrastructureRef2),
			Hosts:            []runtime.Object{&host2, &host1},
			BMMachine:        bmmconfig2,
			ExpectedHostName: host2.Name,
		},
		"Ignore discoveredHost and pick host2, which lacks a ConsumerRef": {
			Machine:          newMachine("machine1", "", infrastructureRef2),
			Hosts:            []runtime.Object{&discoveredHost, &host2, &host1},
			BMMachine:        bmmconfig2,
			ExpectedHostName: host2.Name,
		},
		"Pick host3, which has a matching ConsumerRef": {
			Machine:          newMachine("machine1", "", infrastructureRef3),
			Hosts:            []runtime.Object{&host1, &host3, &host2},
			BMMachine:        bmmconfig3,
			ExpectedHostName: host3.Name,
		},
		"Two hosts already taken, third is in another namespace": {
			Machine:          newMachine("machine2", "", infrastructureRef),
			Hosts:            []runtime.Object{&host1, &host3, &host4},
			BMMachine:        bmmconfig,
			ExpectedHostName: "",
		},
		"Choose hosts with a label, even without a label selector": {
			Machine:          newMachine("machine1", "", infrastructureRef),
			Hosts:            []runtime.Object{&hostWithLabel},
			BMMachine:        bmmconfig,
			ExpectedHostName: hostWithLabel.Name,
		},
		"Choose the host with the right label": {
			Machine:          newMachine("machine1", "", infrastructureRef2),
			Hosts:            []runtime.Object{&hostWithLabel, &host2},
			BMMachine:        bmmconfig2,
			ExpectedHostName: hostWithLabel.Name,
		},
		"No host that matches required label": {
			Machine:          newMachine("machine1", "", infrastructureRef3),
			Hosts:            []runtime.Object{&host2, &hostWithLabel},
			BMMachine:        bmmconfig3,
			ExpectedHostName: "",
		},
		"Host that matches a matchExpression": {
			Machine:          newMachine("machine1", "", infrastructureRef4),
			Hosts:            []runtime.Object{&host2, &hostWithLabel},
			BMMachine:        bmmconfig4,
			ExpectedHostName: hostWithLabel.Name,
		},
		"No Host available that matches a matchExpression": {
			Machine:          newMachine("machine1", "", infrastructureRef4),
			Hosts:            []runtime.Object{&host2},
			BMMachine:        bmmconfig4,
			ExpectedHostName: "",
		},
		"No host chosen, invalid match expression": {
			Machine:          newMachine("machine1", "", infrastructureRef5),
			Hosts:            []runtime.Object{&host2, &hostWithLabel, &host1},
			BMMachine:        bmmconfig5,
			ExpectedHostName: "",
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)
		c := fakeclient.NewFakeClientWithScheme(scheme, tc.Hosts...)

		machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine, tc.BMMachine, klogr.New())
		if err != nil {
			t.Errorf("%v", err)
		}

		result, err := machineMgr.chooseHost(context.TODO())

		if tc.ExpectedHostName == "" {
			if result != nil {
				t.Error("found host when none should have been available")
			}
			continue
		}
		if err != nil {
			t.Errorf("%v", err)
			return
		}
		if result != nil {
			if result.Name != tc.ExpectedHostName {
				t.Errorf("host %s chosen instead of %s", result.Name, tc.ExpectedHostName)
			}
		}
	}
}

/* TODO, commented case to be rewritten
func TestSetHostSpec(t *testing.T) {
	for _, tc := range []struct {
		Scenario                  string
		UserDataNamespace         string
		ExpectedUserDataNamespace string
		Host                      bmh.BareMetalHost
		BMMachine                 capbm.BareMetalMachine
		Machine                   capi.Machine
		ExpectedImage             *bmh.Image
		ExpectUserData            bool
	}{
		{
			Scenario:                  "User data has explicit alternate namespace",
			UserDataNamespace:         "otherns",
			ExpectedUserDataNamespace: "otherns",
			Host: bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "host2",
					Namespace: "myns",
				},
			},
			ExpectedImage: &bmh.Image{
				URL:      testImageURL,
				Checksum: testImageChecksumURL,
			},
			ExpectUserData: true,
		},

		{
			Scenario:                  "User data has no namespace",
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: "myns",
			Host: bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "host2",
					Namespace: "myns",
				},
			},
			ExpectedImage: &bmh.Image{
				URL:      testImageURL,
				Checksum: testImageChecksumURL,
			},
			ExpectUserData: true,
		},

		{
			Scenario:                  "Externally provisioned, same machine",
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: "myns",
			Host: bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "host2",
					Namespace: "myns",
				},
				Spec: bmh.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       "machine1",
						Namespace:  "myns",
						Kind:       "Machine",
						APIVersion: capi.GroupVersion.String(),
					},
				},
			},
			ExpectedImage: &bmh.Image{
				URL:      testImageURL,
				Checksum: testImageChecksumURL,
			},
			ExpectUserData: true,
		},

		{
			Scenario:                  "Previously provisioned, different image, unchanged",
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: "myns",
			Host: bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "host2",
					Namespace: "myns",
				},
				Spec: bmh.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       "mybmmachine",
						Namespace:  "myns",
						Kind:       "BMMachine",
						APIVersion: capi.GroupVersion.String(),
					},
					Image: &bmh.Image{
						URL:      testImageURL + "test",
						Checksum: testImageChecksumURL + "test",
					},
				},
			},
			BMMachine: capbm.BareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mybmmachine",
					Namespace: "myns",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "BMMachine",
					APIVersion: capi.GroupVersion.String(),
				},
			},
			Machine: capi.Machine{},
			ExpectedImage: &bmh.Image{
				URL:      testImageURL + "test",
				Checksum: testImageChecksumURL + "test",
			},
			ExpectUserData: false,
		},
	} {

		t.Run(tc.Scenario, func(t *testing.T) {
			// test setup
			scheme := runtime.NewScheme()
			err := bmoapis.AddToScheme(scheme)

			if err != nil {
				log.Printf("AddToScheme failed: %v", err)
			}

			c := fakeclient.NewFakeClientWithScheme(scheme, &tc.Host)

			//machineMgr, err := NewMachineManager(c, nil, nil, &machine, bmmconfig)
			machineMgr, err := NewMachineManager(c, nil, nil, &tc.Machine, &tc.BMMachine)
			if err != nil {
				t.Errorf("%v", err)
				return
			}
			t.Logf("ACTUATOR %v, NAME %s, NAMESPACE %s", machineMgr, machineMgr.Name(), machineMgr.Namespace())

			// run the function
			err = machineMgr.setHostSpec(context.TODO(), &tc.Host)
			if err != nil {
				t.Errorf("%v", err)
				return
			}

			// get the saved result
			key := client.ObjectKey{
				Name:      tc.Host.Name,
				Namespace: tc.Host.Namespace,
			}
			host := bmh.BareMetalHost{}

			err = c.Get(context.TODO(), key, &host)
			if err != nil {
				t.Errorf("Get failed, %v", err)
				return
			}
			t.Logf("HOSTDATA %v ", host)

			// validate the result
			if host.Spec.ConsumerRef == nil {
				t.Errorf("ConsumerRef not set")
				return
			}
			t.Logf("CONSUMERREF %v", host.Spec.ConsumerRef.Name)
			if host.Spec.ConsumerRef.Name != tc.BMMachine.Name {
				t.Errorf("#### found consumer ref %v --- expected %v", host.Spec.ConsumerRef, machineMgr.Name())
			}
			if host.Spec.ConsumerRef.Namespace != machineMgr.Namespace() {
				t.Errorf("found consumer ref %v", host.Spec.ConsumerRef)
			}
			if host.Spec.ConsumerRef.Kind != "BMMachine" {
				t.Errorf("found consumer ref %v", host.Spec.ConsumerRef)
			}
			if host.Spec.Online != true {
				t.Errorf("host not set to Online")
			}
			if tc.ExpectedImage == nil {
				if host.Spec.Image != nil {
					t.Errorf("Expected image %v but got %v", tc.ExpectedImage, host.Spec.Image)
					return
				}
			} else {
				if *(host.Spec.Image) != *(tc.ExpectedImage) {
					t.Errorf("Expected image %v but got %v", tc.ExpectedImage, host.Spec.Image)
					return
				}
			}
			if tc.ExpectUserData {
				if host.Spec.UserData == nil {
					t.Errorf("UserData not set")
					return
				}
				if host.Spec.UserData.Namespace != tc.ExpectedUserDataNamespace {
					t.Errorf("expected Userdata.Namespace %s, got %s", tc.ExpectedUserDataNamespace, host.Spec.UserData.Namespace)
				}
				if host.Spec.UserData.Name != testUserDataSecretName {
					t.Errorf("expected Userdata.Name %s, got %s", testUserDataSecretName, host.Spec.UserData.Name)
				}
			} else {
				if host.Spec.UserData != nil {
					t.Errorf("did not expect user data, got %v", host.Spec.UserData)
				}
			}
		})
	}
}*/

func TestExists(t *testing.T) {
	scheme := runtime.NewScheme()
	err := bmoapis.AddToScheme(scheme)

	if err != nil {
		log.Printf("AddToScheme failed: %v", err)
	}

	host := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "somehost",
			Namespace: "myns",
		},
	}
	c := fakeclient.NewFakeClientWithScheme(scheme, &host)

	testCases := map[string]struct {
		Client      client.Client
		Machine     capi.Machine
		BMMachine   *capbm.BareMetalMachine
		FailMessage string
		Expected    bool
	}{
		"Failed to find the existing host": {
			Client:      c,
			Machine:     capi.Machine{},
			BMMachine:   newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithSomeAnnotations),
			Expected:    true,
			FailMessage: "failed to find the existing host",
		},
		"Found host even though annotation value incorrect": {
			Client:      c,
			Machine:     capi.Machine{},
			BMMachine:   newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithInvalidAnnotations),
			Expected:    false,
			FailMessage: "found host even though annotation value incorrect",
		},
		"Found host even though annotation not present": {
			Client:      c,
			Machine:     capi.Machine{},
			BMMachine:   newBareMetalMachine("", nil, nil, nil, bmmObjectMetaEmptyAnnotations),
			Expected:    false,
			FailMessage: "found host even though annotation not present",
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)
		machineMgr, err := NewMachineManager(tc.Client, nil, nil, &tc.Machine, tc.BMMachine, klogr.New())
		if err != nil {
			t.Error(err)
		}

		result, err := machineMgr.Exists(context.TODO())
		if err != nil {
			t.Error(err)
		}
		if result != tc.Expected {
			t.Error(tc.FailMessage)
		}
	}
}

func TestGetHost(t *testing.T) {
	scheme := runtime.NewScheme()
	err := bmoapis.AddToScheme(scheme)

	if err != nil {
		log.Printf("AddToScheme failed: %v", err)
	}

	host := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myhost",
			Namespace: "myns",
		},
	}
	c := fakeclient.NewFakeClientWithScheme(scheme, &host)

	testCases := map[string]struct {
		Client        client.Client
		Machine       capi.Machine
		BMMachine     *capbm.BareMetalMachine
		FailMessage   string
		ExpectPresent bool
	}{
		"Did not find expected host": {
			Client:        c,
			Machine:       capi.Machine{},
			BMMachine:     newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithValidAnnotations),
			ExpectPresent: true,
			FailMessage:   "did not find expected host",
		},
		"Found host even though annotation value incorrect": {
			Client:        c,
			Machine:       capi.Machine{},
			BMMachine:     newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithInvalidAnnotations),
			ExpectPresent: false,
			FailMessage:   "found host even though annotation value incorrect",
		},
		"Found host even though annotation not present": {
			Client:        c,
			Machine:       capi.Machine{},
			BMMachine:     newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaEmptyAnnotations),
			ExpectPresent: false,
			FailMessage:   "found host even though annotation not present",
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)
		machineMgr, err := NewMachineManager(tc.Client, nil, nil, &tc.Machine, tc.BMMachine, klogr.New())
		if err != nil {
			t.Error(err)
		}

		result, err := machineMgr.getHost(context.TODO())
		if err != nil {
			t.Error(err)
		}
		if (result != nil) != tc.ExpectPresent {
			t.Error(tc.FailMessage)
		}
	}
}

func TestGetSetProviderID(t *testing.T) {
	scheme := runtime.NewScheme()
	err := bmoapis.AddToScheme(scheme)

	if err != nil {
		log.Printf("AddToScheme failed: %v", err)
	}
	testCases := map[string]struct {
		Machine       *capi.Machine
		BMMachine     *capbm.BareMetalMachine
		Host          *bmh.BareMetalHost
		ExpectPresent bool
		ExpectError   bool
	}{
		"Set ProviderID, empty annotations": {
			Machine:   newMachine("", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, bmmSpec, nil, bmmObjectMetaEmptyAnnotations),
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
				},
			},
			ExpectPresent: false,
			ExpectError:   true,
		},
		"Set ProviderID": {
			Machine:   newMachine("", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, bmmSpec, nil, bmmObjectMetaWithValidAnnotations),
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
					UID:       "12345ID6789",
				},
				Status: bmh.BareMetalHostStatus{
					Provisioning: bmh.ProvisionStatus{
						State: bmh.StateProvisioned,
					},
				},
			},
			ExpectPresent: true,
			ExpectError:   false,
		},
		"Set ProviderID, wrong state": {
			Machine:   newMachine("", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, bmmSpec, nil, bmmObjectMetaWithValidAnnotations),
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
					UID:       "12345ID6789",
				},
				Status: bmh.BareMetalHostStatus{
					Provisioning: bmh.ProvisionStatus{
						State: bmh.StateProvisioning,
					},
				},
			},
			ExpectPresent: false,
			ExpectError:   true,
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)
		c := fakeclient.NewFakeClientWithScheme(scheme, tc.Host)
		machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine, tc.BMMachine, klogr.New())
		if err != nil {
			t.Error(err)
		}

		bmhID, err := machineMgr.GetBaremetalHostID(context.TODO())
		if err != nil {
			if !tc.ExpectError {
				t.Error(err)
			}
		} else {
			if tc.ExpectError {
				t.Error("Expected an error")
			}
		}

		if bmhID == nil {
			if tc.ExpectPresent {
				t.Error("No BaremetalHost UID found")
			}
			continue
		}

		providerID := fmt.Sprintf("metal3://%s", *bmhID)

		if providerID != *tc.BMMachine.Spec.ProviderID {
			t.Errorf("ProviderID not matching!! expected %s, got %s", providerID,
				*tc.BMMachine.Spec.ProviderID)
		}
	}
}

func TestEnsureAnnotation(t *testing.T) {
	scheme := runtime.NewScheme()
	err := capbm.AddToScheme(scheme)

	if err != nil {
		log.Printf("AddToScheme failed: %v", err)
	}

	testCases := map[string]struct {
		Machine   capi.Machine
		Host      *bmh.BareMetalHost
		BMMachine *capbm.BareMetalMachine
	}{
		"Annotation exists and is correct": {
			Machine:   capi.Machine{},
			BMMachine: newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithValidAnnotations),
			Host:      newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
		},
		"Annotation exists but is wrong": {
			Machine:   capi.Machine{},
			BMMachine: newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithInvalidAnnotations),
			Host:      newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
		},
		"Annotations are empty": {
			Machine:   capi.Machine{},
			BMMachine: newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaEmptyAnnotations),
			Host:      newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
		},
		"Annotations are nil": {
			Machine:   capi.Machine{},
			BMMachine: newBareMetalMachine("", nil, nil, nil, bmmObjectMetaNoAnnotations),
			Host:      newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)
		c := fakeclient.NewFakeClientWithScheme(scheme, tc.BMMachine)

		machineMgr, err := NewMachineManager(c, nil, nil, &tc.Machine, tc.BMMachine, klogr.New())
		if err != nil {
			t.Error(err)
		}

		err = machineMgr.ensureAnnotation(context.TODO(), tc.Host)
		if err != nil {
			err := capbm.AddToScheme(scheme)
			t.Errorf("unexpected error %v", err)
		}

		key := client.ObjectKey{
			Name:      machineMgr.Name(),
			Namespace: machineMgr.Namespace(),
		}

		bmmachine := capbm.BareMetalMachine{}
		err = c.Get(context.TODO(), key, &bmmachine)
		if err != nil {
			t.Error(err)
		}

		annotations := bmmachine.ObjectMeta.GetAnnotations()
		if annotations == nil {
			if !(strings.Contains(name, "Annotations are empty") || strings.Contains(name, "Annotations are nil")) {
				t.Error("no annotations found")
			}
		}
		result, ok := annotations[HostAnnotation]
		if !ok {
			if !(strings.Contains(name, "Annotations are empty") || strings.Contains(name, "Annotations are nil")) {
				t.Error("host annotation not found")
			}
		}

		if strings.Contains(name, "Annotation exists and is correct") {
			if result != tc.BMMachine.Annotations[HostAnnotation] {
				t.Errorf("host annotation has value %s, expected \"%s\"",
					result, tc.BMMachine.Annotations[HostAnnotation])
			}
		} else {
			if result == tc.BMMachine.Annotations[HostAnnotation] {
				t.Error("host annotation value should not match")
			}
		}
	}
}

func TestDelete(t *testing.T) {
	scheme := runtime.NewScheme()
	err := bmoapis.AddToScheme(scheme)

	if err != nil {
		log.Printf("AddToScheme failed: %v", err)
	}

	testCases := map[string]struct {
		Host                *bmh.BareMetalHost
		Machine             *capi.Machine
		BMMachine           *capbm.BareMetalMachine
		ExpectedConsumerRef *corev1.ObjectReference
		ExpectedResult      error
	}{
		"Deprovisioning required": {
			Host:                newBareMetalHost("myhost", bmhSpec, bmh.StateProvisioned, bmhStatus, false),
			Machine:             newMachine("mymachine", "", nil),
			BMMachine:           newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithValidAnnotations),
			ExpectedConsumerRef: consumerRef,
			ExpectedResult:      &RequeueAfterError{},
		},
		"Deprovisioning in progress": {
			Host:                newBareMetalHost("myhost", bmhSpecNoImg, bmh.StateDeprovisioning, bmhStatus, false),
			Machine:             newMachine("mymachine", "", nil),
			BMMachine:           newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithValidAnnotations),
			ExpectedConsumerRef: consumerRef,
			ExpectedResult:      &RequeueAfterError{RequeueAfter: time.Second * 30},
		},
		"Externally provisioned host should be powered down": {
			Host:                newBareMetalHost("myhost", bmhSpecNoImg, bmh.StateExternallyProvisioned, bmhPowerStatus, true),
			Machine:             newMachine("mymachine", "", nil),
			BMMachine:           newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithValidAnnotations),
			ExpectedConsumerRef: consumerRef,
			ExpectedResult:      &RequeueAfterError{RequeueAfter: time.Second * 30},
		},
		"Consumer ref should be removed from externally provisioned host": {
			Host:      newBareMetalHost("myhost", bmhSpecNoImg, bmh.StateExternallyProvisioned, bmhPowerStatus, false),
			Machine:   newMachine("mymachine", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, bmmSecret, nil, bmmObjectMetaWithValidAnnotations),
		},
		"Consumer ref should be removed": {
			Host:      newBareMetalHost("myhost", bmhSpecNoImg, bmh.StateReady, bmhStatus, false),
			Machine:   newMachine("mymachine", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, bmmSecret, nil, bmmObjectMetaWithValidAnnotations),
		},
		"Consumer ref does not match, so it should not be removed": {
			Host:                newBareMetalHost("myhost", bmhSpecSomeImg, bmh.StateProvisioned, bmhStatus, false),
			Machine:             newMachine("", "", nil),
			BMMachine:           newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithValidAnnotations),
			ExpectedConsumerRef: consumerRefSome,
		},
		"No consumer ref, so this is a no-op": {
			Host:      newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
			Machine:   newMachine("", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithValidAnnotations),
		},
		"No host at all, so this is a no-op": {
			Host:      nil,
			Machine:   newMachine("", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithValidAnnotations),
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)
		err := capbm.AddToScheme(scheme)
		if err != nil {
			t.Error(err)
		}
		err = corev1.AddToScheme(scheme)
		if err != nil {
			t.Error(err)
		}

		objects := []runtime.Object{
			tc.BMMachine,
		}
		c := fakeclient.NewFakeClientWithScheme(scheme, objects...)

		if tc.Host != nil {
			err := c.Create(context.TODO(), tc.Host)
			if err != nil {
				t.Errorf("Create failed, %v", err)
			}
		}

		machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine, tc.BMMachine, klogr.New())

		if err != nil {
			t.Error(err)
		}

		err = machineMgr.Delete(context.TODO())

		var expectedResult bool
		switch e := tc.ExpectedResult.(type) {
		case nil:
			expectedResult = (err == nil)
		case *RequeueAfterError:
			var perr *RequeueAfterError
			if perr, expectedResult = err.(*RequeueAfterError); expectedResult {
				expectedResult = (*e == *perr)
			}
		}
		if !expectedResult {
			t.Errorf("unexpected error \"%v\" (expected \"%v\")",
				err, tc.ExpectedResult)
		}
		if tc.Host != nil {
			key := client.ObjectKey{
				Name:      tc.Host.Name,
				Namespace: tc.Host.Namespace,
			}
			host := bmh.BareMetalHost{}

			if tc.Host != nil {
				err := c.Get(context.TODO(), key, &host)
				if err != nil {
					t.Errorf("Get failed, %v", err)
				}
			}

			name := ""
			expectedName := ""
			if host.Spec.ConsumerRef != nil {
				name = host.Spec.ConsumerRef.Name
			}
			if tc.ExpectedConsumerRef != nil {
				expectedName = tc.ExpectedConsumerRef.Name
			}
			if name != expectedName {
				t.Errorf("expected ConsumerRef %v, found %v",
					tc.ExpectedConsumerRef, host.Spec.ConsumerRef)
			}
		}
	}
}

func TestUpdateMachineStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	err := capbm.AddToScheme(scheme)

	if err != nil {
		log.Printf("AddToScheme failed: %v", err)
	}

	nic1 := bmh.NIC{
		IP: "192.168.1.1",
	}

	nic2 := bmh.NIC{
		IP: "172.0.20.2",
	}

	testCases := map[string]struct {
		Host            *bmh.BareMetalHost
		Machine         *capi.Machine
		ExpectedMachine capi.Machine
		BMMachine       capbm.BareMetalMachine
	}{
		"Machine status updated": {
			Host: &bmh.BareMetalHost{
				Status: bmh.BareMetalHostStatus{
					HardwareDetails: &bmh.HardwareDetails{
						NIC: []bmh.NIC{nic1, nic2},
					},
				},
			},
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mymachine",
					Namespace: "myns",
				},
				Status: capi.MachineStatus{
					Addresses: []capi.MachineAddress{
						capi.MachineAddress{
							Address: "192.168.1.255",
							Type:    "InternalIP",
						},
						capi.MachineAddress{
							Address: "172.0.20.255",
							Type:    "InternalIP",
						},
					},
				},
			},
			BMMachine: capbm.BareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mybmmachine",
					Namespace: "myns",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "BMMachine",
					APIVersion: capi.GroupVersion.String(),
				},
				Status: capbm.BareMetalMachineStatus{
					Addresses: []capi.MachineAddress{
						capi.MachineAddress{
							Address: "192.168.1.1",
							Type:    "InternalIP",
						},
						capi.MachineAddress{
							Address: "172.0.20.2",
							Type:    "InternalIP",
						},
					},
					Ready: true,
				},
			},
			ExpectedMachine: capi.Machine{
				Status: capi.MachineStatus{
					Addresses: []capi.MachineAddress{
						capi.MachineAddress{
							Address: "192.168.1.1",
							Type:    "InternalIP",
						},
						capi.MachineAddress{
							Address: "172.0.20.2",
							Type:    "InternalIP",
						},
					},
				},
			},
		},
		"Machine status unchanged": {
			Host: &bmh.BareMetalHost{
				Status: bmh.BareMetalHostStatus{
					HardwareDetails: &bmh.HardwareDetails{
						NIC: []bmh.NIC{nic1, nic2},
					},
				},
			},
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mymachine",
					Namespace: "myns",
				},
				Status: capi.MachineStatus{
					Addresses: []capi.MachineAddress{
						capi.MachineAddress{
							Address: "192.168.1.1",
							Type:    "InternalIP",
						},
						capi.MachineAddress{
							Address: "172.0.20.2",
							Type:    "InternalIP",
						},
					},
				},
			},
			BMMachine: capbm.BareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mybmmachine",
					Namespace: "myns",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "BMMachine",
					APIVersion: capi.GroupVersion.String(),
				},
				Status: capbm.BareMetalMachineStatus{
					Addresses: []capi.MachineAddress{
						capi.MachineAddress{
							Address: "192.168.1.1",
							Type:    "InternalIP",
						},
						capi.MachineAddress{
							Address: "172.0.20.2",
							Type:    "InternalIP",
						},
					},
					Ready: true,
				},
			},
			ExpectedMachine: capi.Machine{
				Status: capi.MachineStatus{
					Addresses: []capi.MachineAddress{
						capi.MachineAddress{
							Address: "192.168.1.1",
							Type:    "InternalIP",
						},
						capi.MachineAddress{
							Address: "172.0.20.2",
							Type:    "InternalIP",
						},
					},
				},
			},
		},
		"Machine status unchanged, status set empty": {
			Host: &bmh.BareMetalHost{},
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mymachine",
					Namespace: "myns",
				},
				Status: capi.MachineStatus{},
			},
			BMMachine: capbm.BareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mybmmachine",
					Namespace: "myns",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "BMMachine",
					APIVersion: capi.GroupVersion.String(),
				},
				Status: capbm.BareMetalMachineStatus{
					Addresses: []capi.MachineAddress{},
					Ready:     true,
				},
			},
			ExpectedMachine: capi.Machine{
				Status: capi.MachineStatus{},
			},
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)
		c := fakeclient.NewFakeClientWithScheme(scheme, &tc.BMMachine)

		machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine, &tc.BMMachine, klogr.New())
		if err != nil {
			t.Error(err)
		}

		err = machineMgr.updateMachineStatus(context.TODO(), tc.Host)
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}

		key := client.ObjectKey{
			Name:      machineMgr.Name(),
			Namespace: machineMgr.Namespace(),
		}

		bmmachine := capbm.BareMetalMachine{}
		err = c.Get(context.TODO(), key, &bmmachine)
		if err != nil {
			t.Error(err)
		}

		if tc.BMMachine.Status.Addresses != nil {
			for i, address := range tc.ExpectedMachine.Status.Addresses {
				if address != bmmachine.Status.Addresses[i] {
					t.Errorf("expected Address %v, found %v", address, bmmachine.Status.Addresses[i])
				}
			}
		}
	}
}

func TestNodeAddresses(t *testing.T) {
	scheme := runtime.NewScheme()
	err := capi.AddToScheme(scheme)

	if err != nil {
		log.Printf("AddToScheme failed: %v", err)
	}

	nic1 := bmh.NIC{
		IP: "192.168.1.1",
	}

	nic2 := bmh.NIC{
		IP: "172.0.20.2",
	}

	addr1 := capi.MachineAddress{
		Type:    capi.MachineInternalIP,
		Address: "192.168.1.1",
	}

	addr2 := capi.MachineAddress{
		Type:    capi.MachineInternalIP,
		Address: "172.0.20.2",
	}

	addr3 := capi.MachineAddress{
		Type:    capi.MachineHostName,
		Address: "mygreathost",
	}

	testCases := map[string]struct {
		Machine               capi.Machine
		BMMachine             capbm.BareMetalMachine
		Host                  *bmh.BareMetalHost
		ExpectedNodeAddresses []capi.MachineAddress
	}{
		"One NIC": {
			Host: &bmh.BareMetalHost{
				Status: bmh.BareMetalHostStatus{
					HardwareDetails: &bmh.HardwareDetails{
						NIC: []bmh.NIC{nic1},
					},
				},
			},
			ExpectedNodeAddresses: []capi.MachineAddress{addr1},
		},
		"Two NICs": {
			Host: &bmh.BareMetalHost{
				Status: bmh.BareMetalHostStatus{
					HardwareDetails: &bmh.HardwareDetails{
						NIC: []bmh.NIC{nic1, nic2},
					},
				},
			},
			ExpectedNodeAddresses: []capi.MachineAddress{addr1, addr2},
		},
		"Hostname is set": {
			Host: &bmh.BareMetalHost{
				Status: bmh.BareMetalHostStatus{
					HardwareDetails: &bmh.HardwareDetails{
						Hostname: "mygreathost",
					},
				},
			},
			ExpectedNodeAddresses: []capi.MachineAddress{addr3},
		},
		"Empty Hostname": {
			Host: &bmh.BareMetalHost{
				Status: bmh.BareMetalHostStatus{
					HardwareDetails: &bmh.HardwareDetails{
						Hostname: "",
					},
				},
			},
			ExpectedNodeAddresses: []capi.MachineAddress{},
		},
		"No host at all, so this is a no-op": {
			Host:                  nil,
			ExpectedNodeAddresses: nil,
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)
		var nodeAddresses []capi.MachineAddress

		c := fakeclient.NewFakeClientWithScheme(scheme)
		machineMgr, err := NewMachineManager(c, nil, nil, &tc.Machine, &tc.BMMachine, klogr.New())

		if err != nil {
			t.Error(err)
		}

		if tc.Host != nil {
			nodeAddresses = machineMgr.nodeAddresses(tc.Host)
			if err != nil {
				t.Errorf("unexpected error %v", err)
			}
		}
		for i, address := range tc.ExpectedNodeAddresses {
			if address != nodeAddresses[i] {
				t.Errorf("expected Address %v, found %v", address, nodeAddresses[i])
			}
		}
	}
}

func TestSetNodeProviderID(t *testing.T) {
	scheme := runtime.NewScheme()
	err := capi.AddToScheme(scheme)
	if err != nil {
		log.Printf("AddToScheme failed: %v", err)
	}
	err = bmoapis.AddToScheme(scheme)
	if err != nil {
		log.Printf("AddToScheme failed: %v", err)
	}

	testCases := map[string]struct {
		Node               v1.Node
		HostID             string
		ExpectedError      bool
		ExpectedProviderID string
	}{
		"Set target ProviderID, No matching node": {
			Node:               v1.Node{},
			HostID:             "abcd",
			ExpectedError:      true,
			ExpectedProviderID: "metal3://abcd",
		},
		"Set target ProviderID, matching node": {
			Node: v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"metal3.io/uuid": "abcd",
					},
				},
			},
			HostID:             "abcd",
			ExpectedError:      false,
			ExpectedProviderID: "metal3://abcd",
		},
		"Set target ProviderID, providerID set": {
			Node: v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"metal3.io/uuid": "abcd",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: "metal3://abcd",
				},
			},
			HostID:             "abcd",
			ExpectedError:      false,
			ExpectedProviderID: "metal3://abcd",
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)

		c := fakeclient.NewFakeClientWithScheme(scheme)
		corev1Client := clientfake.NewSimpleClientset(&tc.Node).CoreV1()
		mockCapiClientGetter := func(c client.Client, cluster *capi.Cluster) (
			clientcorev1.CoreV1Interface, error,
		) {
			return corev1Client, nil
		}
		machineMgr, err := NewMachineManager(c, nil, nil,
			&capi.Machine{}, &capbm.BareMetalMachine{}, klogr.New(),
		)

		if err != nil {
			t.Error(err)
		}

		err = machineMgr.SetNodeProviderID(context.TODO(), tc.HostID,
			tc.ExpectedProviderID, mockCapiClientGetter,
		)

		if tc.ExpectedError {
			if err == nil {
				t.Errorf("Expected error when trying to set node provider ID")
			}
			continue
		} else {
			if err != nil {
				t.Errorf("Did not expect error when setting node providerID : %v", err)
				continue
			}
		}

		// get the result
		node, err := corev1Client.Nodes().Get(tc.Node.Name, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Get failed, %v", err)
			return
		}

		if node.Spec.ProviderID != tc.ExpectedProviderID {
			t.Errorf("Wrong providerID, expected %v, got %v", tc.ExpectedProviderID,
				node.Spec.ProviderID,
			)
		}
	}
}

func newConfig(t *testing.T,
	UserDataNamespace string,
	labels map[string]string,
	reqs []capbm.HostSelectorRequirement) (*capbm.BareMetalMachine, *corev1.ObjectReference) {
	config := capbm.BareMetalMachine{
		Spec: capbm.BareMetalMachineSpec{
			Image: capbm.Image{
				URL:      testImageURL,
				Checksum: testImageChecksumURL,
			},
			UserData: &corev1.SecretReference{
				Name:      testUserDataSecretName,
				Namespace: UserDataNamespace,
			},
			HostSelector: capbm.HostSelector{
				MatchLabels:      labels,
				MatchExpressions: reqs,
			},
		},
	}

	infrastructureRef := &corev1.ObjectReference{
		Name:       "someothermachine",
		Namespace:  "myns",
		Kind:       "Machine",
		APIVersion: capi.GroupVersion.String(),
	}
	return &config, infrastructureRef
}

func newMachine(machineName string, bareMetalMachineName string, infraRef *corev1.ObjectReference) *capi.Machine {
	if machineName == "" {
		return &capi.Machine{}
	}

	if infraRef == nil {
		infraRef = &corev1.ObjectReference{}
	}

	machine := &capi.Machine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Machine",
			APIVersion: capi.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: "myns",
		},
		Spec: capi.MachineSpec{
			InfrastructureRef: *infraRef,
		},
	}
	return machine
}

func newBareMetalMachine(name string,
	ownerRef *metav1.OwnerReference,
	spec *capbm.BareMetalMachineSpec,
	status *capbm.BareMetalMachineStatus,
	objMeta *metav1.ObjectMeta) *capbm.BareMetalMachine {

	if name == "" {
		return &capbm.BareMetalMachine{}
	}

	typeMeta := &metav1.TypeMeta{
		Kind:       "BMMachine",
		APIVersion: capbm.GroupVersion.String(),
	}

	if spec == nil {
		spec = &capbm.BareMetalMachineSpec{}
	}
	if status == nil {
		status = &capbm.BareMetalMachineStatus{}
	}

	return &capbm.BareMetalMachine{
		TypeMeta:   *typeMeta,
		ObjectMeta: *objMeta,
		Spec:       *spec,
		Status:     *status,
	}
}

func newBareMetalHost(name string,
	spec *bmh.BareMetalHostSpec,
	state bmh.ProvisioningState,
	status *bmh.BareMetalHostStatus,
	powerOn bool) *bmh.BareMetalHost {

	if name == "" {
		return &bmh.BareMetalHost{}
	}

	objMeta := &metav1.ObjectMeta{
		Name:      name,
		Namespace: "myns",
	}

	if spec == nil {
		return &bmh.BareMetalHost{
			ObjectMeta: *objMeta,
		}
	}

	if status != nil {
		status.Provisioning.State = state
		status.PoweredOn = powerOn
	}

	return &bmh.BareMetalHost{
		ObjectMeta: *objMeta,
		Spec:       *spec,
		Status:     *status,
	}
}
