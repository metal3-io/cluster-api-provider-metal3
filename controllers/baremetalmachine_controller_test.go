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
	"fmt"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Reconcile Baremetalcluster", func() {

	RefTestBareMetalCluster := &infrav1.BareMetalCluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "BareMetalCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      baremetalClusterName,
			Namespace: namespaceName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       clusterName,
				},
			},
		},
		Spec: infrav1.BareMetalClusterSpec{
			APIEndpoint: "http://192.168.111.249:6443",
		},
	}

	RefTestCluster := &clusterv1.Cluster{

		TypeMeta: metav1.TypeMeta{
			Kind: "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespaceName,
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &v1.ObjectReference{
				Name:       baremetalClusterName,
				Namespace:  namespaceName,
				Kind:       "InfrastructureConfig",
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha2",
			},
		},
		Status: clusterv1.ClusterStatus{
			InfrastructureReady: true,
		},
	}
	//Given machine, but no baremetalMachine resource
	It("Should not return an error when baremetalcluster is not found", func() {
		m := newMachine(clusterName, machineName, nil)
		c := fake.NewFakeClientWithScheme(setupScheme(), m)

		r := &BareMetalMachineReconciler{
			Client: c,
			Log:    klogr.New(),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
			},
		}

		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	//Given both machine and baremetalMachine with OwnerRef not set.
	It("Should not return an error if OwnerRef is not set on BareMetalCluster", func() {

		baremetalMachine := &infrav1.BareMetalMachine{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
			},
			Spec:   infrav1.BareMetalMachineSpec{},
			Status: infrav1.BareMetalMachineStatus{},
		}
		m := newMachine(clusterName, machineName, baremetalMachine)
		c := fake.NewFakeClientWithScheme(setupScheme(), m, baremetalMachine)

		r := &BareMetalMachineReconciler{
			Client: c,
			Log:    klogr.New(),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
			},
		}

		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	//Given both machine and baremetalMachine with OwnerRef set, Machine is not namespaced, it should error.
	It("Should return an error when Machine cannot be found", func() {

		baremetalMachine := &infrav1.BareMetalMachine{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       machineName,
					},
				},
			},
			Spec:   infrav1.BareMetalMachineSpec{},
			Status: infrav1.BareMetalMachineStatus{},
		}
		c := fake.NewFakeClientWithScheme(setupScheme(), baremetalMachine)

		r := &BareMetalMachineReconciler{
			Client: c,
			Log:    klogr.New(),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
			},
		}

		res, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	//Given machine with cluster label but cluster non-existent, it should error.
	It("Should return an error when owner Cluster cannot be found", func() {

		baremetalMachine := &infrav1.BareMetalMachine{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       machineName,
					},
				},
			},
			Spec:   infrav1.BareMetalMachineSpec{},
			Status: infrav1.BareMetalMachineStatus{},
		}
		m := newMachine(clusterName, machineName, baremetalMachine)
		c := fake.NewFakeClientWithScheme(setupScheme(), m, baremetalMachine)

		r := &BareMetalMachineReconciler{
			Client: c,
			Log:    klogr.New(),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
			},
		}

		res, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	//Given owner cluster infra not ready, it should wait and not return error
	It("Should not return an error when owner Cluster infrastructure is not ready", func() {
		cluster1 := newCluster(clusterName)
		baremetalMachine := &infrav1.BareMetalMachine{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       machineName,
					},
				},
			},
			Spec:   infrav1.BareMetalMachineSpec{},
			Status: infrav1.BareMetalMachineStatus{},
		}
		m := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      machineName,
				Namespace: namespaceName,
				Labels: map[string]string{
					clusterv1.MachineClusterLabelName: clusterName,
				},
			},
			Spec: clusterv1.MachineSpec{
				InfrastructureRef: v1.ObjectReference{
					Name:       baremetalMachine.Name,
					Namespace:  baremetalMachine.Namespace,
					Kind:       baremetalMachine.Kind,
					APIVersion: baremetalMachine.GroupVersionKind().GroupVersion().String(),
				},
			},
		}
		c := fake.NewFakeClientWithScheme(setupScheme(), m, baremetalMachine, cluster1)

		r := &BareMetalMachineReconciler{
			Client: c,
			Log:    klogr.New(),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
			},
		}

		res, err := r.Reconcile(req)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	//Given owner cluster infra is ready and BMCluster does not exist, it should not return an error
	It("Should not return an error when owner Cluster infrastructure is ready and BMCluster does not exist", func() {

		baremetalMachine := &infrav1.BareMetalMachine{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       machineName,
					},
				},
			},
			Spec:   infrav1.BareMetalMachineSpec{},
			Status: infrav1.BareMetalMachineStatus{},
		}
		m := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      machineName,
				Namespace: namespaceName,
				Labels: map[string]string{
					clusterv1.MachineClusterLabelName: clusterName,
				},
			},
			Spec: clusterv1.MachineSpec{
				InfrastructureRef: v1.ObjectReference{
					Name:       baremetalMachine.Name,
					Namespace:  baremetalMachine.Namespace,
					Kind:       baremetalMachine.Kind,
					APIVersion: baremetalMachine.GroupVersionKind().GroupVersion().String(),
				},
			},
		}
		c := fake.NewFakeClientWithScheme(setupScheme(), m, baremetalMachine, RefTestCluster)

		r := &BareMetalMachineReconciler{
			Client: c,
			Log:    klogr.New(),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
			},
		}

		res, err := r.Reconcile(req)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	//Given owner cluster infra is ready and BMCluster exists, it should not return an error
	It("Should not return an error when owner Cluster infrastructure is ready and BMCluster exist", func() {

		baremetalMachine := &infrav1.BareMetalMachine{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       machineName,
					},
				},
			},
			Spec:   infrav1.BareMetalMachineSpec{},
			Status: infrav1.BareMetalMachineStatus{},
		}
		m := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      machineName,
				Namespace: namespaceName,
				Labels: map[string]string{
					clusterv1.MachineClusterLabelName: clusterName,
				},
			},
			Spec: clusterv1.MachineSpec{
				InfrastructureRef: v1.ObjectReference{
					Name:       baremetalMachine.Name,
					Namespace:  baremetalMachine.Namespace,
					Kind:       baremetalMachine.Kind,
					APIVersion: baremetalMachine.GroupVersionKind().GroupVersion().String(),
				},
			},
		}
		c := fake.NewFakeClientWithScheme(setupScheme(), m, baremetalMachine, RefTestCluster, RefTestBareMetalCluster)

		r := &BareMetalMachineReconciler{
			Client:         c,
			ManagerFactory: baremetal.NewManagerFactory(c),
			Log:            klogr.New(),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
			},
		}

		res, err := r.Reconcile(req)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	//Given deletion timestamp, delete is reconciled
	It("Should not return an error and finish deletion of BareMetalMachine", func() {

		baremetalMachine := &infrav1.BareMetalMachine{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       machineName,
					},
				},
			},
			Spec: infrav1.BareMetalMachineSpec{
				UserData: &corev1.SecretReference{
					Name:      bareMetalMachineName + "-user-data",
					Namespace: namespaceName,
				},
			},
			Status: infrav1.BareMetalMachineStatus{},
		}
		m := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      machineName,
				Namespace: namespaceName,
				Labels: map[string]string{
					clusterv1.MachineClusterLabelName: clusterName,
				},
			},
			Spec: clusterv1.MachineSpec{
				InfrastructureRef: v1.ObjectReference{
					Name:       baremetalMachine.Name,
					Namespace:  baremetalMachine.Namespace,
					Kind:       baremetalMachine.Kind,
					APIVersion: baremetalMachine.GroupVersionKind().GroupVersion().String(),
				},
			},
		}
		machineSecret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      bareMetalMachineName + "-user-data",
				Namespace: namespaceName,
			},

			Type: "Opaque",
		}

		deletionTimestamp := metav1.Now()
		baremetalMachine.SetDeletionTimestamp(&deletionTimestamp)
		c := fake.NewFakeClientWithScheme(setupScheme(), machineSecret, m, baremetalMachine, RefTestCluster, RefTestBareMetalCluster)

		r := &BareMetalMachineReconciler{
			Client:         c,
			ManagerFactory: baremetal.NewManagerFactory(c),
			Log:            klogr.New(),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      bareMetalMachineName,
				Namespace: namespaceName,
			},
		}

		res, err := r.Reconcile(req)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	// Legacy tests
	It("TestBareMetalMachineReconciler_BareMetalClusterToBareMetalMachines", func() {
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
		Expect(len(out)).To(Equal(2), "Expected 2 baremetal machines to reconcile but got %d", len(out))

		for _, expectedName := range []string{"my-machine-0", "my-machine-1"} {
			Expect(contains(machineNames, expectedName)).To(BeTrue(), "expected %q in slice %v", expectedName, machineNames)
		}
	})

	It("TestBareMetalMachineReconciler_BareMetalClusterToBareMetalMachines_with_no_cluster", func() {
		baremetalCluster := newBareMetalCluster(clusterName, "my-baremetal-cluster")
		baremetalMachine1 := newBareMetalMachine("my-baremetal-machine-0")
		baremetalMachine2 := newBareMetalMachine("my-baremetal-machine-1")
		objects := []runtime.Object{
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
		fmt.Printf("%v", out)
		Expect(len(out)).To(Equal(0), "Expected 0 request, found %d", len(out))
	})

	It("TestBareMetalMachineReconciler_BareMetalClusterToBareMetalMachines_with_no_bareMetalcluster", func() {
		objects := []runtime.Object{}
		c := fake.NewFakeClientWithScheme(setupScheme(), objects...)
		r := BareMetalMachineReconciler{
			Client: c,
			Log:    klogr.New(),
		}
		mo := handler.MapObject{}
		out := r.BareMetalClusterToBareMetalMachines(mo)
		fmt.Printf("%v", out)
		Expect(len(out)).To(Equal(0), "Expected 0 request, found %d", len(out))
	})

	It("TestBareMetalHostToBareMetalMachines", func() {
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
				Expect(len(reqs)).To(Equal(1), "Expected 1 request, found %d", len(reqs))

				req := reqs[0]
				Expect(req.NamespacedName.Name).To(Equal(tc.Host.Spec.ConsumerRef.Name), "Expected name %s, found %s", tc.Host.Spec.ConsumerRef.Name, req.NamespacedName.Name)
				Expect(req.NamespacedName.Namespace).To(Equal(tc.Host.Spec.ConsumerRef.Namespace), "Expected namespace %s, found %s", tc.Host.Spec.ConsumerRef.Namespace, req.NamespacedName.Namespace)

			} else {
				Expect(len(reqs)).To(Equal(0), "Expected 0 request, found %d", len(reqs))

			}
		}

	})
})

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
			Name:      clusterName,
			Namespace: namespaceName,
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
			Name:      machineName,
			Namespace: namespaceName,
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
