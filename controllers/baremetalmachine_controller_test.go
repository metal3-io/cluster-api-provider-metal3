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

package controllers

import (
	"testing"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

func TestBareMetalMachineReconciler_BareMetalClusterToBareMetalMachines(t *testing.T) {
	clusterName := "my-cluster"
	baremetalCluster := newBareMetalCluster(clusterName, "my-baremetal-cluster")
	baremetalMachine1 := newBareMetalMachine("my-baremetal-machine-0")
	baremetalMachine2 := newBareMetalMachine("my-baremetal-machine-1")
	objects := []runtime.Object{
		newCluster(clusterName),
		baremetalCluster,
		newMachine(clusterName, "my-machine-0", baremetalMachine1),
		newMachine(clusterName, "my-machine-1", baremetalMachine2),
		// Intentionally omitted
		newMachine(clusterName, "my-machine-2", nil),
	}
	c := fake.NewFakeClientWithScheme(setupScheme(), objects...)
	r := BareMetalMachineReconciler{
		Client: c,
		Log:    klogr.New(),
	}
	mo := handler.MapObject{
		Object: baremetalCluster,
	}
	out := r.BareMetalClusterToBareMetalMachines(mo)
	machineNames := make([]string, len(out))
	for i := range out {
		machineNames[i] = out[i].Name
	}
	if len(out) != 2 {
		t.Fatal("expected 2 baremetal machines to reconcile but got", len(out))
	}
	for _, expectedName := range []string{"my-machine-0", "my-machine-1"} {
		if !contains(machineNames, expectedName) {
			t.Fatalf("expected %q in slice %v", expectedName, machineNames)
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, straw := range haystack {
		if straw == needle {
			return true
		}
	}
	return false
}

func newCluster(clusterName string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
	}
}

func newBareMetalCluster(clusterName, baremetalName string) *infrav1.BareMetalCluster {
	return &infrav1.BareMetalCluster{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: baremetalName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       clusterName,
				},
			},
		},
	}
}

func newMachine(clusterName, machineName string, baremetalMachine *infrav1.BareMetalMachine) *clusterv1.Machine {
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name: machineName,
			Labels: map[string]string{
				clusterv1.MachineClusterLabelName: clusterName,
			},
		},
	}
	if baremetalMachine != nil {
		machine.Spec.InfrastructureRef = v1.ObjectReference{
			Name:       baremetalMachine.Name,
			Namespace:  baremetalMachine.Namespace,
			Kind:       baremetalMachine.Kind,
			APIVersion: baremetalMachine.GroupVersionKind().GroupVersion().String(),
		}
	}
	return machine
}

func newBareMetalMachine(name string) *infrav1.BareMetalMachine {
	return &infrav1.BareMetalMachine{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec:   infrav1.BareMetalMachineSpec{},
		Status: infrav1.BareMetalMachineStatus{},
	}
}

func TestBareMetalHostToBareMetalMachines(t *testing.T) {
	r := BareMetalMachineReconciler{}

	for _, tc := range []struct {
		Host          *bmh.BareMetalHost
		ExpectRequest bool
	}{
		{
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "host1",
					Namespace: "myns",
				},
				Spec: bmh.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       "someothermachine",
						Namespace:  "myns",
						Kind:       "Machine",
						APIVersion: capi.GroupVersion.String(),
					},
				},
			},
			ExpectRequest: true,
		},
		{
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "host1",
					Namespace: "myns",
				},
				Spec: bmh.BareMetalHostSpec{},
			},
			ExpectRequest: false,
		},
	} {
		obj := handler.MapObject{
			Object: tc.Host,
		}
		reqs := r.BareMetalHostToBareMetalMachines(obj)

		if tc.ExpectRequest {
			if len(reqs) != 1 {
				t.Errorf("Expected 1 request, found %d", len(reqs))
			}
			req := reqs[0]
			if req.NamespacedName.Name != tc.Host.Spec.ConsumerRef.Name {
				t.Errorf("Expected name %s, found %s", tc.Host.Spec.ConsumerRef.Name, req.NamespacedName.Name)
			}
			if req.NamespacedName.Namespace != tc.Host.Spec.ConsumerRef.Namespace {
				t.Errorf("Expected namespace %s, found %s", tc.Host.Spec.ConsumerRef.Namespace, req.NamespacedName.Namespace)
			}

		} else {
			if len(reqs) != 0 {
				t.Errorf("Expected 0 request, found %d", len(reqs))
			}
		}
	}
}
