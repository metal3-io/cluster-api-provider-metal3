/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package baremetal

import (
	"context"
	"encoding/base64"
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
var CloudInitData = []byte("metal3:cloudInitData1010101test__hello")

var bmmSpec = &capbm.BareMetalMachineSpec{
	ProviderID: &ProviderID,
}

var bmmSpecAll = &capbm.BareMetalMachineSpec{
	ProviderID: &ProviderID,
	UserData: &corev1.SecretReference{
		Name:      "mybmmachine",
		Namespace: "myns",
	},
	Image: capbm.Image{
		URL:      testImageURL,
		Checksum: testImageChecksumURL,
	},
	HostSelector: capbm.HostSelector{},
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

var expectedImg = &bmh.Image{
	URL:      testImageURL,
	Checksum: testImageChecksumURL,
}

var expectedImgTest = &bmh.Image{
	URL:      testImageURL + "test",
	Checksum: testImageChecksumURL + "test",
}

var bmhSpec = &bmh.BareMetalHostSpec{
	ConsumerRef: consumerRef,
	Image: &bmh.Image{
		URL: "myimage",
	},
}

var bmhSpecTestImg = &bmh.BareMetalHostSpec{
	ConsumerRef: consumerRef,
	Image:       expectedImgTest,
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
		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.Hosts...)

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

func TestSetHostSpec(t *testing.T) {
	testCases := map[string]struct {
		Client                    client.Client
		UserDataNamespace         string
		ExpectedUserDataNamespace string
		Host                      *bmh.BareMetalHost
		ExpectedImage             *bmh.Image
		ExpectUserData            bool
	}{
		"User data has explicit alternate namespace": {
			UserDataNamespace:         "otherns",
			ExpectedUserDataNamespace: "otherns",
			Host:                      newBareMetalHost("host2", nil, bmh.StateNone, nil, false),
			ExpectedImage:             expectedImg,
			ExpectUserData:            true,
		},
		"User data has no namespace": {
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: "myns",
			Host:                      newBareMetalHost("host2", nil, bmh.StateNone, nil, false),
			ExpectedImage:             expectedImg,
			ExpectUserData:            true,
		},
		"Externally provisioned, same machine": {
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: "myns",
			Host:                      newBareMetalHost("host2", nil, bmh.StateNone, nil, false),
			ExpectedImage:             expectedImg,
			ExpectUserData:            true,
		},
		"Previously provisioned, different image, unchanged": {
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: "myns",
			Host:                      newBareMetalHost("host2", bmhSpecTestImg, bmh.StateNone, nil, false),
			ExpectedImage:             expectedImgTest,
			ExpectUserData:            false,
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)
		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.Host)

		bmmconfig, infrastructureRef := newConfig(t, tc.UserDataNamespace, map[string]string{}, []capbm.HostSelectorRequirement{})
		machine := newMachine("machine1", "", infrastructureRef)

		machineMgr, err := NewMachineManager(c, nil, nil, machine, bmmconfig, klogr.New())
		if err != nil {
			t.Error(err)
			return
		}

		err = machineMgr.setHostSpec(context.TODO(), tc.Host)
		if err != nil {
			t.Error(err)
			return
		}

		// get the saved host
		savedHost := bmh.BareMetalHost{}
		err = c.Get(context.TODO(), client.ObjectKey{Name: tc.Host.Name, Namespace: tc.Host.Namespace}, &savedHost)
		if err != nil {
			t.Error(err)
		}

		// validate the saved host
		if savedHost.Spec.ConsumerRef == nil {
			t.Error("ConsumerRef not set")
			return
		}
		if savedHost.Spec.ConsumerRef.Name != bmmconfig.Name {
			t.Errorf("found consumer ref %v", savedHost.Spec.ConsumerRef)
		}
		if savedHost.Spec.ConsumerRef.Namespace != bmmconfig.Namespace {
			t.Errorf("found consumer ref %v", savedHost.Spec.ConsumerRef)
		}
		if savedHost.Spec.ConsumerRef.Kind != "BareMetalMachine" {
			t.Errorf("found consumer ref %v", savedHost.Spec.ConsumerRef)
		}
		if savedHost.Spec.Online != true {
			t.Errorf("host not set to Online")
		}
		if tc.ExpectedImage == nil {
			if savedHost.Spec.Image != nil {
				t.Errorf("Expected image %v but got %v", tc.ExpectedImage, savedHost.Spec.Image)
				return
			}
		} else {
			if *(savedHost.Spec.Image) != *(tc.ExpectedImage) {
				t.Errorf("Expected image %v but got %v", tc.ExpectedImage, savedHost.Spec.Image)
				return
			}
		}
		if tc.ExpectUserData {
			if savedHost.Spec.UserData == nil {
				t.Errorf("UserData not set")
				return
			}
			if savedHost.Spec.UserData.Namespace != tc.ExpectedUserDataNamespace {
				t.Errorf("expected Userdata.Namespace %s, got %s", tc.ExpectedUserDataNamespace, savedHost.Spec.UserData.Namespace)
			}
			if savedHost.Spec.UserData.Name != testUserDataSecretName {
				t.Errorf("expected Userdata.Name %s, got %s", testUserDataSecretName, savedHost.Spec.UserData.Name)
			}
		} else {
			if savedHost.Spec.UserData != nil {
				t.Errorf("did not expect user data, got %v", savedHost.Spec.UserData)
			}
		}
	}
}

func TestExists(t *testing.T) {

	host := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "somehost",
			Namespace: "myns",
		},
	}
	c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), &host)

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

		result, err := machineMgr.exists(context.TODO())
		if err != nil {
			t.Error(err)
		}
		if result != tc.Expected {
			t.Error(tc.FailMessage)
		}
	}
}

func TestGetHost(t *testing.T) {

	host := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myhost",
			Namespace: "myns",
		},
	}
	c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), &host)

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
		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.Host)
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

func TestSmallFunctions(t *testing.T) {
	host := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myhost",
			Namespace: "myns",
		},
	}
	c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), &host)
	testCases := map[string]struct {
		Client           client.Client
		Machine          *capi.Machine
		BMMachine        *capbm.BareMetalMachine
		ExpectCtrlNode   bool
		ExpectWorkerNode bool
	}{
		"Test small functions, worker node": {
			Client:           c,
			Machine:          newMachine("", "", nil),
			BMMachine:        newBareMetalMachine("mybmmachine", nil, bmmSpec, nil, bmmObjectMetaEmptyAnnotations),
			ExpectCtrlNode:   false,
			ExpectWorkerNode: true,
		},
		"Test small functions, control plane node": {
			Client: c,
			Machine: &capi.Machine{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Machine",
					APIVersion: capi.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mymachine",
					Namespace: "myns",
					Labels: map[string]string{
						capi.MachineControlPlaneLabelName: "labelHere",
					},
				},
			},
			BMMachine:        newBareMetalMachine("mybmmachine", nil, bmmSpec, nil, bmmObjectMetaEmptyAnnotations),
			ExpectCtrlNode:   true,
			ExpectWorkerNode: false,
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)
		machineMgr, err := NewMachineManager(tc.Client, nil, nil, tc.Machine, tc.BMMachine, klogr.New())
		if err != nil {
			t.Error(err)
		}

		role := machineMgr.role()
		if role != "control-plane" && role != "node" {
			t.Errorf("Invalid machine role %s", role)
		}

		isCtrlPlane := machineMgr.isControlPlane()
		if tc.ExpectCtrlNode {
			if !isCtrlPlane {
				t.Error("Control plane node expected")
			}
		}
		if tc.ExpectWorkerNode {
			if isCtrlPlane {
				t.Error("Worker node expected")
			}
		}
	}
}

func TestEnsureAnnotation(t *testing.T) {
	testCases := map[string]struct {
		Machine                 capi.Machine
		Host                    *bmh.BareMetalHost
		BMMachine               *capbm.BareMetalMachine
		ExpectAnnotation        bool
		ExpectInvalidAnnotation bool
	}{
		"Annotation exists and is correct": {
			Machine:                 capi.Machine{},
			BMMachine:               newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithValidAnnotations),
			Host:                    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
			ExpectAnnotation:        true,
			ExpectInvalidAnnotation: false,
		},
		"Annotation exists but is wrong": {
			Machine:                 capi.Machine{},
			BMMachine:               newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithInvalidAnnotations),
			Host:                    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
			ExpectAnnotation:        true,
			ExpectInvalidAnnotation: true,
		},
		"Annotations are empty": {
			Machine:                 capi.Machine{},
			BMMachine:               newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaEmptyAnnotations),
			Host:                    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
			ExpectAnnotation:        false,
			ExpectInvalidAnnotation: false,
		},
		"Annotations are nil": {
			Machine:                 capi.Machine{},
			BMMachine:               newBareMetalMachine("", nil, nil, nil, bmmObjectMetaNoAnnotations),
			Host:                    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
			ExpectAnnotation:        false,
			ExpectInvalidAnnotation: false,
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)
		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.BMMachine)

		machineMgr, err := NewMachineManager(c, nil, nil, &tc.Machine, tc.BMMachine, klogr.New())
		if err != nil {
			t.Error(err)
		}

		err = machineMgr.ensureAnnotation(context.TODO(), tc.Host)
		if err != nil {
			t.Error(err)
		}

		key := client.ObjectKey{
			Name:      tc.BMMachine.ObjectMeta.Name,
			Namespace: tc.BMMachine.ObjectMeta.Namespace,
		}

		bmmachine := capbm.BareMetalMachine{}
		err = c.Get(context.TODO(), key, &bmmachine)
		if err != nil {
			t.Error(err)
		}

		annotations := bmmachine.ObjectMeta.GetAnnotations()
		if annotations == nil {
			if tc.ExpectAnnotation {
				t.Error("No annotations found")
			}
		} else {
			if tc.ExpectAnnotation {
				if annotations[HostAnnotation] != tc.BMMachine.Annotations[HostAnnotation] {
					if !tc.ExpectInvalidAnnotation {
						t.Error("Annotation mismatch")
					}
				}
			}
			if !tc.ExpectAnnotation {
				t.Error("Annotation not should not exist")
			}
		}

		ok := machineMgr.HasAnnotation()
		result := annotations[HostAnnotation]
		if !ok {
			if tc.ExpectAnnotation {
				t.Error("Host annotation not found")
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

		objects := []runtime.Object{
			tc.BMMachine,
		}
		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)

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
		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), &tc.BMMachine)

		machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine, &tc.BMMachine, klogr.New())
		if err != nil {
			t.Error(err)
		}

		err = machineMgr.updateMachineStatus(context.TODO(), tc.Host)
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}

		key := client.ObjectKey{
			Name:      tc.BMMachine.ObjectMeta.Name,
			Namespace: tc.BMMachine.ObjectMeta.Namespace,
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

		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm())
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
		machineMgr, err := NewMachineManager(c, newCluster(clusterName),
			newBareMetalCluster(baremetalClusterName, bmcOwnerRef,
				&capbm.BareMetalClusterSpec{NoCloudProvider: true}, nil,
			),
			&capi.Machine{}, &capbm.BareMetalMachine{}, klogr.New(),
		)

		if err != nil {
			t.Error(err)
			continue
		}

		err = machineMgr.SetNodeProviderID(tc.HostID,
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

func TestAssociate(t *testing.T) {
	testCases := map[string]struct {
		Machine   *capi.Machine
		Host      *bmh.BareMetalHost
		BMMachine *capbm.BareMetalMachine
	}{
		"Associate empty machine, baremetal machine spec nil": {
			Machine:   newMachine("", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithValidAnnotations),
			Host:      newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
		},
		"Associate empty machine, baremetal machine spec set": {
			Machine:   newMachine("", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, bmmSpecAll, nil, bmmObjectMetaWithValidAnnotations),
			Host:      newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
		},
		"Associate empty machine, host empty, baremetal machine spec set": {
			Machine:   newMachine("", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, bmmSpecAll, nil, bmmObjectMetaWithValidAnnotations),
			Host:      newBareMetalHost("", nil, bmh.StateNone, nil, false),
		},
		"Associate machine, host empty, baremetal machine spec set": {
			Machine:   newMachine("mymachine", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, bmmSpecAll, nil, bmmObjectMetaWithValidAnnotations),
			Host:      newBareMetalHost("", nil, bmh.StateNone, nil, false),
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)

		objects := []runtime.Object{
			tc.Host,
			tc.BMMachine,
			tc.Machine,
		}
		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)

		machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine, tc.BMMachine, klogr.New())
		if err != nil {
			t.Error(err)
		}

		err = machineMgr.Associate(context.TODO())
		if err != nil {
			t.Error(err)
		}
	}
}

func TestUpdate(t *testing.T) {
	testCases := map[string]struct {
		Machine   *capi.Machine
		Host      *bmh.BareMetalHost
		BMMachine *capbm.BareMetalMachine
	}{
		"Update machine": {
			Machine:   newMachine("mymachine", "", nil),
			BMMachine: newBareMetalMachine("mybmmachine", nil, nil, nil, bmmObjectMetaWithValidAnnotations),
			Host:      newBareMetalHost("myhost", nil, bmh.StateNone, nil, false),
		},
	}

	for name, tc := range testCases {
		t.Logf("## TC-%s ##", name)

		objects := []runtime.Object{
			tc.Host,
			tc.BMMachine,
			tc.Machine,
		}
		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)

		machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine, tc.BMMachine, klogr.New())
		if err != nil {
			t.Error(err)
		}

		err = machineMgr.Update(context.TODO())
		if err != nil {
			t.Error(err)
		}
	}
}

//-----------------
// Helper functions
//-----------------
func setupSchemeMm() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := capbm.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := bmoapis.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := capi.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
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
		Kind:       "BMMachine",
		APIVersion: capbm.GroupVersion.String(),
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

	data := base64.StdEncoding.EncodeToString(CloudInitData)

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
			Bootstrap: capi.Bootstrap{
				ConfigRef: &corev1.ObjectReference{},
				Data:      &data,
			},
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
	} else {
		status = &bmh.BareMetalHostStatus{}
	}

	return &bmh.BareMetalHost{
		ObjectMeta: *objMeta,
		Spec:       *spec,
		Status:     *status,
	}
}
