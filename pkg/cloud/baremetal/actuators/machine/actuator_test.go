package machine

import (
	"context"
	"testing"

	bmoapis "github.com/metalkube/baremetal-operator/pkg/apis"
	bmh "github.com/metalkube/baremetal-operator/pkg/apis/metalkube/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterapis "sigs.k8s.io/cluster-api/pkg/apis"
	machinev1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
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

	testCases := []struct {
		Machine          machinev1.Machine
		Hosts            []runtime.Object
		ExpectedHostName string
	}{
		{
			// should pick host2, which lacks a MachineRef
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
			},
			Hosts:            []runtime.Object{&host1, &host2},
			ExpectedHostName: host2.Name,
		},
		{
			// should pick host3, which already has a matching MachineRef
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "myns",
				},
			},
			Hosts:            []runtime.Object{&host1, &host2, &host3},
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
			},
			Hosts:            []runtime.Object{&host1, &host3, &host4},
			ExpectedHostName: "",
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

		result, err := actuator.chooseHost(context.TODO(), &tc.Machine)
		if tc.ExpectedHostName == "" {
			if err == nil {
				t.Error("found host when none should have been available")
			}
			if result != nil {
				t.Errorf("expected nil, got %v", result)
			}
			continue
		}
		if err != nil {
			t.Errorf("%v", err)
		}
		if result.Spec.MachineRef.Name != tc.Machine.Name {
			t.Errorf("found machine ref %v", result.Spec.MachineRef)
		}
		if result.Name != tc.ExpectedHostName {
			t.Errorf("host %s chosen instead of %s", result.Name, tc.ExpectedHostName)
		}
		savedHost := bmh.BareMetalHost{}
		err = c.Get(context.TODO(), client.ObjectKey{Name: result.Name, Namespace: result.Namespace}, &savedHost)
		if err != nil {
			t.Errorf("%v", err)
		}
		if savedHost.Spec.MachineRef == nil {
			t.Errorf("machine ref %v not saved to host", result.Spec.MachineRef)
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
			// no machine ref, so this is a no-op
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
		if err != nil {
			t.Errorf("unexpected error %v", err)
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
