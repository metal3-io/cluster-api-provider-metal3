package machine

import (
	"context"
	"testing"

	bmoapis "github.com/metalkube/baremetal-operator/pkg/apis"
	bmh "github.com/metalkube/baremetal-operator/pkg/apis/metalkube/v1alpha1"
	clusterapis "github.com/openshift/cluster-api/pkg/apis"
	machinev1 "github.com/openshift/cluster-api/pkg/apis/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
			// should not pick a host, because both are already taken
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine2",
					Namespace: "myns",
				},
			},
			Hosts:            []runtime.Object{&host1, &host3},
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
						HostAnnotation: "somehost",
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
						HostAnnotation: "myns",
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
						HostAnnotation: "myhost",
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
						HostAnnotation: "myns",
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
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						HostAnnotation: "myhost",
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
			Machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						HostAnnotation: "wrongvalue",
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
		if result != tc.Host.Name {
			t.Errorf("host annotation has value %s, expected %s", result, tc.Host.Name)
		}
	}
}
