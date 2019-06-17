package machine

import (
	"context"
	"testing"
	"time"

	bmoapis "github.com/metal3-io/baremetal-operator/pkg/apis"
	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	bmv1alpha1 "github.com/metal3-io/cluster-api-provider-baremetal/pkg/apis/baremetal/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterapis "sigs.k8s.io/cluster-api/pkg/apis"
	machinev1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

const (
	testImageURL                = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2"
	testImageChecksumURL        = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum"
	testUserDataSecretName      = "worker-user-data"
	testUserDataSecretNamespace = "myns"
)

func TestChooseHost(t *testing.T) {
	scheme := runtime.NewScheme()
	bmoapis.AddToScheme(scheme)

	host1 := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host1",
			Namespace: "myns",
		},
		Spec: bmh.BareMetalHostSpec{
			MachineRef: &corev1.ObjectReference{
				Name:      "someothermachine",
				Namespace: "myns",
			},
		},
	}
	host2 := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host2",
			Namespace: "myns",
		},
	}
	host3 := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host3",
			Namespace: "myns",
		},
		Spec: bmh.BareMetalHostSpec{
			MachineRef: &corev1.ObjectReference{
				Name:      "machine1",
				Namespace: "myns",
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
	host_with_label := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host_with_label",
			Namespace: "myns",
			Labels:    map[string]string{"key1": "value1"},
		},
	}

	config, providerSpec := newConfig(t, "", map[string]string{}, []bmv1alpha1.HostSelectorRequirement{})
	config2, providerSpec2 := newConfig(t, "", map[string]string{"key1": "value1"}, []bmv1alpha1.HostSelectorRequirement{})
	config3, providerSpec3 := newConfig(t, "", map[string]string{"boguskey": "value"}, []bmv1alpha1.HostSelectorRequirement{})
	config4, providerSpec4 := newConfig(t, "", map[string]string{},
		[]bmv1alpha1.HostSelectorRequirement{
			bmv1alpha1.HostSelectorRequirement{
				Key:      "key1",
				Operator: "in",
				Values:   []string{"abc", "value1", "123"},
			},
		})
	config5, providerSpec5 := newConfig(t, "", map[string]string{},
		[]bmv1alpha1.HostSelectorRequirement{
			bmv1alpha1.HostSelectorRequirement{
				Key:      "key1",
				Operator: "pancakes",
				Values:   []string{"abc", "value1", "123"},
			},
		})

	testCases := []struct {
		Machine          machinev1.Machine
		Hosts            []runtime.Object
		ExpectedHostName string
		Config           *bmv1alpha1.BareMetalMachineProviderSpec
	}{
		{
			// should pick host2, which lacks a MachineRef
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
				Spec: machinev1.MachineSpec{
					ProviderSpec: providerSpec,
				},
			},
			Hosts:            []runtime.Object{&host2, &host1},
			ExpectedHostName: host2.Name,
		},
		{
			// should ignore discoveredHost and pick host2, which lacks a MachineRef
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
			},
			Hosts:            []runtime.Object{&discoveredHost, &host2, &host1},
			ExpectedHostName: host2.Name,
		},
		{
			// should pick host3, which already has a matching MachineRef
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
				Spec: machinev1.MachineSpec{
					ProviderSpec: providerSpec,
				},
			},
			Hosts:            []runtime.Object{&host1, &host3, &host2},
			ExpectedHostName: host3.Name,
		},
		{
			// should not pick a host, because two are already taken, and the third is in
			// a different namespace
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine2",
					Namespace: "myns",
				},
				Spec: machinev1.MachineSpec{
					ProviderSpec: providerSpec,
				},
			},
			Hosts:            []runtime.Object{&host1, &host3, &host4},
			ExpectedHostName: "",
		},
		{
			// Can choose hosts with a label, even without a label selector
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
				Spec: machinev1.MachineSpec{
					ProviderSpec: providerSpec,
				},
			},
			Hosts:            []runtime.Object{&host_with_label},
			ExpectedHostName: host_with_label.Name,
		},
		{
			// Choose the host with the right label
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
				Spec: machinev1.MachineSpec{
					ProviderSpec: providerSpec2,
				},
			},
			Hosts:            []runtime.Object{&host2, &host_with_label},
			ExpectedHostName: host_with_label.Name,
			Config:           config2,
		},
		{
			// No host that matches required label
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
				Spec: machinev1.MachineSpec{
					ProviderSpec: providerSpec3,
				},
			},
			Hosts:            []runtime.Object{&host2, &host_with_label},
			ExpectedHostName: "",
			Config:           config3,
		},
		{
			// Host that matches a matchExpression
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
				Spec: machinev1.MachineSpec{
					ProviderSpec: providerSpec4,
				},
			},
			Hosts:            []runtime.Object{&host2, &host_with_label},
			ExpectedHostName: host_with_label.Name,
			Config:           config4,
		},
		{
			// No Host available that matches a matchExpression
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
				Spec: machinev1.MachineSpec{
					ProviderSpec: providerSpec4,
				},
			},
			Hosts:            []runtime.Object{&host2},
			ExpectedHostName: "",
			Config:           config4,
		},
		{
			// No host chosen given an Invalid match expression
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
				Spec: machinev1.MachineSpec{
					ProviderSpec: providerSpec5,
				},
			},
			Hosts:            []runtime.Object{&host2, &host_with_label},
			ExpectedHostName: "",
			Config:           config5,
		},
	}

	for _, tc := range testCases {
		c := fakeclient.NewFakeClientWithScheme(scheme, tc.Hosts...)

		actuator, err := NewActuator(ActuatorParams{
			Client: c,
		})
		if err != nil {
			t.Errorf("%v", err)
		}
		cfg := tc.Config
		if cfg == nil {
			cfg = config
		}
		result, err := actuator.chooseHost(context.TODO(), &tc.Machine, cfg)
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
		if result.Name != tc.ExpectedHostName {
			t.Errorf("host %s chosen instead of %s", result.Name, tc.ExpectedHostName)
		}
	}
}

func TestSetHostSpec(t *testing.T) {
	for _, tc := range []struct {
		UserDataNamespace         string
		ExpectedUserDataNamespace string
	}{
		{
			UserDataNamespace:         "otherns",
			ExpectedUserDataNamespace: "otherns",
		},
		{
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: "myns",
		},
	} {

		// test data
		config, providerSpec := newConfig(t, tc.UserDataNamespace, map[string]string{}, []bmv1alpha1.HostSelectorRequirement{})
		host := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "host2",
				Namespace: "myns",
			},
		}
		machine := machinev1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine1",
				Namespace: "myns",
			},
			Spec: machinev1.MachineSpec{
				ProviderSpec: providerSpec,
			},
		}

		// test setup
		scheme := runtime.NewScheme()
		bmoapis.AddToScheme(scheme)
		c := fakeclient.NewFakeClientWithScheme(scheme, &host)

		actuator, err := NewActuator(ActuatorParams{
			Client: c,
		})
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		// run the function
		err = actuator.setHostSpec(context.TODO(), &host, &machine, config)
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		// get the saved result
		savedHost := bmh.BareMetalHost{}
		err = c.Get(context.TODO(), client.ObjectKey{Name: host.Name, Namespace: host.Namespace}, &savedHost)
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		// validate the result
		if savedHost.Spec.MachineRef == nil {
			t.Errorf("MachineRef not set")
			return
		}
		if savedHost.Spec.MachineRef.Name != machine.Name {
			t.Errorf("found machine ref %v", savedHost.Spec.MachineRef)
		}
		if savedHost.Spec.MachineRef.Namespace != machine.Namespace {
			t.Errorf("found machine ref %v", savedHost.Spec.MachineRef)
		}
		if savedHost.Spec.Online != true {
			t.Errorf("host not set to Online")
		}
		if savedHost.Spec.Image == nil {
			t.Errorf("Image not set")
			return
		}
		if savedHost.Spec.Image.URL != testImageURL {
			t.Errorf("expected ImageURL %s, got %s", testImageURL, savedHost.Spec.Image.URL)
		}
		if savedHost.Spec.Image.Checksum != testImageChecksumURL {
			t.Errorf("expected ImageChecksumURL %s, got %s", testImageChecksumURL, savedHost.Spec.Image.Checksum)
		}
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
	}
}

func TestExists(t *testing.T) {
	scheme := runtime.NewScheme()
	bmoapis.AddToScheme(scheme)

	host := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "somehost",
			Namespace: "myns",
		},
	}
	c := fakeclient.NewFakeClientWithScheme(scheme, &host)

	testCases := []struct {
		Client      client.Client
		Machine     machinev1.Machine
		Expected    bool
		FailMessage string
	}{
		{
			Client: c,
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						HostAnnotation: "myns/somehost",
					},
				},
			},
			Expected:    true,
			FailMessage: "failed to find the existing host",
		},
		{
			Client: c,
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						HostAnnotation: "myns/wrong",
					},
				},
			},
			Expected:    false,
			FailMessage: "found host even though annotation value incorrect",
		},
		{
			Client:      c,
			Machine:     machinev1.Machine{},
			Expected:    false,
			FailMessage: "found host even though annotation not present",
		},
	}

	for _, tc := range testCases {
		actuator, err := NewActuator(ActuatorParams{
			Client: tc.Client,
		})
		if err != nil {
			t.Error(err)
		}

		result, err := actuator.Exists(context.TODO(), nil, &tc.Machine)
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
	bmoapis.AddToScheme(scheme)

	host := bmh.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myhost",
			Namespace: "myns",
		},
	}
	c := fakeclient.NewFakeClientWithScheme(scheme, &host)

	testCases := []struct {
		Client        client.Client
		Machine       machinev1.Machine
		ExpectPresent bool
		FailMessage   string
	}{
		{
			Client: c,
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						HostAnnotation: "myns/myhost",
					},
				},
			},
			ExpectPresent: true,
			FailMessage:   "did not find expected host",
		},
		{
			Client: c,
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						HostAnnotation: "myns/wrong",
					},
				},
			},
			ExpectPresent: false,
			FailMessage:   "found host even though annotation value incorrect",
		},
		{
			Client:        c,
			Machine:       machinev1.Machine{},
			ExpectPresent: false,
			FailMessage:   "found host even though annotation not present",
		},
	}

	for _, tc := range testCases {
		actuator, err := NewActuator(ActuatorParams{
			Client: tc.Client,
		})
		if err != nil {
			t.Error(err)
		}

		result, err := actuator.getHost(context.TODO(), &tc.Machine)
		if err != nil {
			t.Error(err)
		}
		if (result != nil) != tc.ExpectPresent {
			t.Error(tc.FailMessage)
		}
	}
}

func TestEnsureAnnotation(t *testing.T) {
	scheme := runtime.NewScheme()
	clusterapis.AddToScheme(scheme)

	testCases := []struct {
		Machine machinev1.Machine
		Host    bmh.BareMetalHost
	}{
		{
			// annotation exists and is correct
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						HostAnnotation: "myns/myhost",
					},
				},
			},
			Host: bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
				},
			},
		},
		{
			// annotation exists but is wrong
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						HostAnnotation: "myns/wrongvalue",
					},
				},
			},
			Host: bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
				},
			},
		},
		{
			// annotations are empty
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			Host: bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
				},
			},
		},
		{
			// annotations are nil
			Machine: machinev1.Machine{},
			Host: bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
				},
			},
		},
	}

	for _, tc := range testCases {
		c := fakeclient.NewFakeClientWithScheme(scheme, &tc.Machine)
		actuator, err := NewActuator(ActuatorParams{
			Client: c,
		})
		if err != nil {
			t.Error(err)
		}

		err = actuator.ensureAnnotation(context.TODO(), &tc.Machine, &tc.Host)
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}

		// get the machine and make sure it has the correct annotation
		machine := machinev1.Machine{}
		key := client.ObjectKey{
			Name:      tc.Machine.Name,
			Namespace: tc.Machine.Namespace,
		}
		err = c.Get(context.TODO(), key, &machine)
		annotations := machine.ObjectMeta.GetAnnotations()
		if annotations == nil {
			t.Error("no annotations found")
		}
		result, ok := annotations[HostAnnotation]
		if !ok {
			t.Error("host annotation not found")
		}
		if result != "myns/myhost" {
			t.Errorf("host annotation has value %s, expected \"myns/myhost\"", result)
		}
	}
}

func TestDelete(t *testing.T) {
	scheme := runtime.NewScheme()
	bmoapis.AddToScheme(scheme)

	testCases := []struct {
		Host               *bmh.BareMetalHost
		Machine            machinev1.Machine
		ExpectedMachineRef *corev1.ObjectReference
		ExpectedResult     error
	}{
		{
			// machine ref should be removed
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
				},
				Spec: bmh.BareMetalHostSpec{
					MachineRef: &corev1.ObjectReference{
						Name:      "mymachine",
						Namespace: "myns",
					},
				},
			},
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mymachine",
					Namespace: "myns",
					Annotations: map[string]string{
						HostAnnotation: "myns/myhost",
					},
				},
			},
			ExpectedResult: &clustererror.RequeueAfterError{},
		},
		{
			// machine ref does not match, so it should not be removed
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
				},
				Spec: bmh.BareMetalHostSpec{
					MachineRef: &corev1.ObjectReference{
						Name:      "someoneelsesmachine",
						Namespace: "myns",
					},
				},
			},
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mymachine",
					Namespace: "myns",
					Annotations: map[string]string{
						HostAnnotation: "myns/myhost",
					},
				},
			},
			ExpectedMachineRef: &corev1.ObjectReference{
				Name:      "someoneelsesmachine",
				Namespace: "myns",
			},
		},
		{
			// no machine ref, so wait for deprovisioning
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
				},
			},
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mymachine",
					Namespace: "myns",
					Annotations: map[string]string{
						HostAnnotation: "myns/myhost",
					},
				},
			},
			ExpectedResult: &clustererror.RequeueAfterError{RequeueAfter: time.Second * 30},
		},
		{
			// no host at all, so this is a no-op
			Host: nil,
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mymachine",
					Namespace: "myns",
					Annotations: map[string]string{
						HostAnnotation: "myns/myhost",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		c := fakeclient.NewFakeClientWithScheme(scheme)
		if tc.Host != nil {
			c.Create(context.TODO(), tc.Host)
		}
		actuator, err := NewActuator(ActuatorParams{
			Client: c,
		})
		if err != nil {
			t.Error(err)
		}

		err = actuator.Delete(context.TODO(), nil, &tc.Machine)

		var expectedResult bool
		switch e := tc.ExpectedResult.(type) {
		case nil:
			expectedResult = (err == nil)
		case *clustererror.RequeueAfterError:
			var perr *clustererror.RequeueAfterError
			if perr, expectedResult = err.(*clustererror.RequeueAfterError); expectedResult {
				expectedResult = (*e == *perr)
			}
		}
		if !expectedResult {
			t.Errorf("unexpected error \"%v\" (expected \"%v\")", err, tc.ExpectedResult)
		}
		if tc.Host != nil {
			key := client.ObjectKey{
				Name:      tc.Host.Name,
				Namespace: tc.Host.Namespace,
			}
			host := bmh.BareMetalHost{}
			c.Get(context.TODO(), key, &host)
			name := ""
			expectedName := ""
			if host.Spec.MachineRef != nil {
				name = host.Spec.MachineRef.Name
			}
			if tc.ExpectedMachineRef != nil {
				expectedName = tc.ExpectedMachineRef.Name
			}
			if name != expectedName {
				t.Errorf("expected MachineRef %v, found %v", tc.ExpectedMachineRef, host.Spec.MachineRef)
			}
		}
	}
}

func TestConfigFromProviderSpec(t *testing.T) {
	ps := machinev1.ProviderSpec{
		Value: &runtime.RawExtension{
			Raw: []byte(`{"apiVersion":"baremetal.cluster.k8s.io/v1alpha1","userData":{"Name":"worker-user-data","Namespace":"myns"},"image":{"Checksum":"http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum","URL":"http://172.22.0.1/images/rhcos-ootpa-latest.qcow2"},"kind":"BareMetalMachineProviderSpec"}`),
		},
	}
	config, err := configFromProviderSpec(ps)
	if err != nil {
		t.Errorf("Error: %s", err.Error())
		return
	}
	if config == nil {
		t.Errorf("got a nil config")
		return
	}

	if config.Image.URL != testImageURL {
		t.Errorf("expected Image.URL %s, got %s", testImageURL, config.Image.URL)
	}
	if config.Image.Checksum != testImageChecksumURL {
		t.Errorf("expected Image.Checksum %s, got %s", testImageChecksumURL, config.Image.Checksum)
	}
	if config.UserData == nil {
		t.Errorf("UserData not set")
		return
	}
	if config.UserData.Name != testUserDataSecretName {
		t.Errorf("expected UserData.Name %s, got %s", testUserDataSecretName, config.UserData.Name)
	}
	if config.UserData.Namespace != testUserDataSecretNamespace {
		t.Errorf("expected UserData.Namespace %s, got %s", testUserDataSecretNamespace, config.UserData.Namespace)
	}
}

func newConfig(t *testing.T, UserDataNamespace string, labels map[string]string, reqs []bmv1alpha1.HostSelectorRequirement) (*bmv1alpha1.BareMetalMachineProviderSpec, machinev1.ProviderSpec) {
	config := bmv1alpha1.BareMetalMachineProviderSpec{
		Image: bmv1alpha1.Image{
			URL:      testImageURL,
			Checksum: testImageChecksumURL,
		},
		UserData: &corev1.SecretReference{
			Name:      testUserDataSecretName,
			Namespace: UserDataNamespace,
		},
		HostSelector: bmv1alpha1.HostSelector{
			MatchLabels:      labels,
			MatchExpressions: reqs,
		},
	}
	out, err := yaml.Marshal(&config)
	if err != nil {
		t.Logf("could not marshal BareMetalMachineProviderSpec: %v", err)
		t.FailNow()
	}
	return &config, machinev1.ProviderSpec{
		Value: &runtime.RawExtension{Raw: out},
	}
}

func TestUpdateMachineStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	clusterapis.AddToScheme(scheme)

	nic1 := bmh.NIC{
		IP: "192.168.1.1",
	}

	nic2 := bmh.NIC{
		IP: "172.0.20.2",
	}

	testCases := []struct {
		Host            *bmh.BareMetalHost
		Machine         *machinev1.Machine
		ExpectedMachine machinev1.Machine
	}{
		{
			// machine status updated
			Host: &bmh.BareMetalHost{
				Status: bmh.BareMetalHostStatus{
					HardwareDetails: &bmh.HardwareDetails{
						NIC: []bmh.NIC{nic1, nic2},
					},
				},
			},
			Machine: &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mymachine",
					Namespace: "myns",
				},
				Status: machinev1.MachineStatus{},
			},
			ExpectedMachine: machinev1.Machine{
				Status: machinev1.MachineStatus{
					Addresses: []corev1.NodeAddress{
						corev1.NodeAddress{
							Address: "192.168.1.1",
							Type:    "InternalIP",
						},
						corev1.NodeAddress{
							Address: "172.0.20.2",
							Type:    "InternalIP",
						},
					},
				},
			},
		},
		{
			// machine status unchanged
			Host: &bmh.BareMetalHost{
				Status: bmh.BareMetalHostStatus{
					HardwareDetails: &bmh.HardwareDetails{
						NIC: []bmh.NIC{nic1, nic2},
					},
				},
			},
			Machine: &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mymachine",
					Namespace: "myns",
				},
				Status: machinev1.MachineStatus{
					Addresses: []corev1.NodeAddress{
						corev1.NodeAddress{
							Address: "192.168.1.1",
							Type:    "InternalIP",
						},
						corev1.NodeAddress{
							Address: "172.0.20.2",
							Type:    "InternalIP",
						},
					},
				},
			},
			ExpectedMachine: machinev1.Machine{
				Status: machinev1.MachineStatus{
					Addresses: []corev1.NodeAddress{
						corev1.NodeAddress{
							Address: "192.168.1.1",
							Type:    "InternalIP",
						},
						corev1.NodeAddress{
							Address: "172.0.20.2",
							Type:    "InternalIP",
						},
					},
				},
			},
		},
		{
			// machine status unchanged
			Host: &bmh.BareMetalHost{},
			Machine: &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mymachine",
					Namespace: "myns",
				},
				Status: machinev1.MachineStatus{},
			},
			ExpectedMachine: machinev1.Machine{
				Status: machinev1.MachineStatus{},
			},
		},
	}

	for _, tc := range testCases {
		c := fakeclient.NewFakeClientWithScheme(scheme)
		if tc.Machine != nil {
			c.Create(context.TODO(), tc.Machine)
		}
		actuator, err := NewActuator(ActuatorParams{
			Client: c,
		})
		if err != nil {
			t.Error(err)
		}

		err = actuator.updateMachineStatus(context.TODO(), tc.Machine, tc.Host)
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}
		key := client.ObjectKey{
			Name:      tc.Machine.Name,
			Namespace: tc.Machine.Namespace,
		}
		machine := machinev1.Machine{}
		c.Get(context.TODO(), key, &machine)

		if &tc.Machine != nil {
			key := client.ObjectKey{
				Name:      tc.Machine.Name,
				Namespace: tc.Machine.Namespace,
			}
			machine := machinev1.Machine{}
			c.Get(context.TODO(), key, &machine)

			if tc.Machine.Status.Addresses != nil {
				for i, address := range tc.ExpectedMachine.Status.Addresses {
					if address != machine.Status.Addresses[i] {
						t.Errorf("expected Address %v, found %v", address, machine.Status.Addresses[i])
					}
				}
			}
		}
	}
}

func TestApplyMachineStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	clusterapis.AddToScheme(scheme)

	addr1 := corev1.NodeAddress{
		Type:    "InternalIP",
		Address: "192.168.1.1",
	}

	addr2 := corev1.NodeAddress{
		Type:    "InternalIP",
		Address: "172.0.20.2",
	}

	testCases := []struct {
		Machine               *machinev1.Machine
		Addresses             []corev1.NodeAddress
		ExpectedNodeAddresses []corev1.NodeAddress
	}{
		{
			// Machine status updated
			Machine: &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
				Status: machinev1.MachineStatus{
					Addresses: []corev1.NodeAddress{},
				},
			},
			Addresses:             []corev1.NodeAddress{addr1, addr2},
			ExpectedNodeAddresses: []corev1.NodeAddress{addr1, addr2},
		},
		{
			// Machine status unchanged
			Machine: &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
				Status: machinev1.MachineStatus{
					Addresses: []corev1.NodeAddress{addr1, addr2},
				},
			},
			Addresses:             []corev1.NodeAddress{addr1, addr2},
			ExpectedNodeAddresses: []corev1.NodeAddress{addr1, addr2},
		},
	}

	for _, tc := range testCases {
		c := fakeclient.NewFakeClientWithScheme(scheme)
		if tc.Machine != nil {
			c.Create(context.TODO(), tc.Machine)
		}
		actuator, err := NewActuator(ActuatorParams{
			Client: c,
		})
		if err != nil {
			t.Error(err)
		}

		err = actuator.applyMachineStatus(context.TODO(), tc.Machine, tc.Addresses)
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}

		key := client.ObjectKey{
			Name:      tc.Machine.Name,
			Namespace: tc.Machine.Namespace,
		}
		machine := machinev1.Machine{}
		c.Get(context.TODO(), key, &machine)

		if tc.Machine.Status.Addresses != nil {
			for i, address := range tc.ExpectedNodeAddresses {
				if address != machine.Status.Addresses[i] {
					t.Errorf("expected Address %v, found %v", address, machine.Status.Addresses[i])
				}
			}
		}
	}
}

func TestNodeAddresses(t *testing.T) {
	scheme := runtime.NewScheme()
	clusterapis.AddToScheme(scheme)

	nic1 := bmh.NIC{
		IP: "192.168.1.1",
	}

	nic2 := bmh.NIC{
		IP: "172.0.20.2",
	}

	addr1 := corev1.NodeAddress{
		Type:    "InternalIP",
		Address: "192.168.1.1",
	}

	addr2 := corev1.NodeAddress{
		Type:    "InternalIP",
		Address: "172.0.20.2",
	}

	testCases := []struct {
		Host                  *bmh.BareMetalHost
		ExpectedNodeAddresses []corev1.NodeAddress
	}{
		{
			// One NIC
			Host: &bmh.BareMetalHost{
				Status: bmh.BareMetalHostStatus{
					HardwareDetails: &bmh.HardwareDetails{
						NIC: []bmh.NIC{nic1},
					},
				},
			},
			ExpectedNodeAddresses: []corev1.NodeAddress{addr1},
		},
		{
			// Two NICs
			Host: &bmh.BareMetalHost{
				Status: bmh.BareMetalHostStatus{
					HardwareDetails: &bmh.HardwareDetails{
						NIC: []bmh.NIC{nic1, nic2},
					},
				},
			},
			ExpectedNodeAddresses: []corev1.NodeAddress{addr1, addr2},
		},
		{
			// no host at all, so this is a no-op
			Host:                  nil,
			ExpectedNodeAddresses: nil,
		},
	}

	for _, tc := range testCases {
		c := fakeclient.NewFakeClientWithScheme(scheme)
		if tc.Host != nil {
			c.Create(context.TODO(), tc.Host)
		}
		actuator, err := NewActuator(ActuatorParams{
			Client: c,
		})
		if err != nil {
			t.Error(err)
		}

		nodeAddresses, err := actuator.nodeAddresses(tc.Host)
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}
		expectedNodeAddresses := tc.ExpectedNodeAddresses

		for i, address := range expectedNodeAddresses {
			if address != nodeAddresses[i] {
				t.Errorf("expected Address %v, found %v", address, nodeAddresses[i])
			}
		}
	}
}
