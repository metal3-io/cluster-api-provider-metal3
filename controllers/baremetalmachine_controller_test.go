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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Reconcile Baremetalcluster", func() {

	providerID := "metal3:///foo/bar"

	bootstrapData := "Qm9vdHN0cmFwIERhdGEK"

	type TestCaseReconcile struct {
		Objects         []runtime.Object
		ErrorExpected   bool
		RequeueExpected bool
	}

	DescribeTable("Reconcile tests",
		func(tc TestCaseReconcile) {
			c := fake.NewFakeClientWithScheme(setupScheme(), tc.Objects...)

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
			if tc.ErrorExpected {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.RequeueExpected {
				Expect(res.Requeue).NotTo(BeFalse())
			} else {
				Expect(res.Requeue).To(BeFalse())
			}
		},
		//Given machine, but no baremetalMachine resource
		Entry("Should not return an error when baremetalcluster is not found",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newMachine(clusterName, machineName, ""),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//Given both machine and baremetalMachine with OwnerRef not set.
		Entry("Should not return an error if OwnerRef is not set on BareMetalCluster",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(bareMetalMachineName, nil, nil, nil),
					newMachine(clusterName, machineName, ""),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//Given baremetalMachine with OwnerRef set, Machine is not found, it should error.
		Entry("Should return an error when Machine cannot be found",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(bareMetalMachineName, bmmOwnerRef, nil, nil),
				},
				ErrorExpected:   true,
				RequeueExpected: false,
			},
		),
		//Given machine with cluster label but cluster non-existent, it should error.
		Entry("Should return an error when owner Cluster cannot be found",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(bareMetalMachineName, bmmOwnerRef, nil, nil),
					newMachine(clusterName, machineName, bareMetalMachineName),
				},
				ErrorExpected:   true,
				RequeueExpected: false,
			},
		),
		//Given owner cluster infra not ready, it should wait and not return error
		Entry("Should not return an error when owner Cluster infrastructure is not ready",
			TestCaseReconcile{
				Objects: []runtime.Object{
					&clusterv1.Cluster{
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
							InfrastructureReady: false,
						},
					},
					newBareMetalMachine(bareMetalMachineName, bmmOwnerRef, nil, nil),
					newMachine(clusterName, machineName, bareMetalMachineName),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//Given owner cluster infra is ready and BMCluster does not exist, it should not return an error
		Entry("Should not return an error when owner Cluster infrastructure is ready and BMCluster does not exist",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(bareMetalMachineName, bmmOwnerRef, nil, nil),
					newMachine(clusterName, machineName, bareMetalMachineName),
					newCluster(clusterName),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//Given owner cluster infra is ready and BMCluster exists, it should not return an error
		Entry("Should not return an error when owner Cluster infrastructure is ready and BMCluster exist",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(bareMetalMachineName, bmmOwnerRef, nil, nil),
					newMachine(clusterName, machineName, bareMetalMachineName),
					newCluster(clusterName),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//BaremetalMachine already deployed
		Entry("Should not return an error when BaremetalMachine is deployed",
			TestCaseReconcile{
				Objects: []runtime.Object{
					&infrav1.BareMetalMachine{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      bareMetalMachineName,
							Namespace: namespaceName,
							Annotations: map[string]string{
								baremetal.HostAnnotation: "testNameSpace/bmh-0",
							},
							OwnerReferences: []metav1.OwnerReference{*bmmOwnerRef},
						},
						Spec: infrav1.BareMetalMachineSpec{
							ProviderID: &providerID,
						},
						Status: infrav1.BareMetalMachineStatus{
							Ready: true,
						},
					},
					newMachine(clusterName, machineName, bareMetalMachineName),
					newCluster(clusterName),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
					&bmh.BareMetalHost{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "bmh-0",
							Namespace: namespaceName,
						},
						Spec: bmh.BareMetalHostSpec{},
						Status: bmh.BareMetalHostStatus{
							Provisioning: bmh.ProvisionStatus{
								State: bmh.StateProvisioned,
							},
						},
					},
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//Bootstrap data available on machine
		Entry("Should not return an error when bootstrap data is available, no annotation",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(bareMetalMachineName, bmmOwnerRef, nil, nil),
					&clusterv1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      machineName,
							Namespace: namespaceName,
							Labels: map[string]string{
								clusterv1.MachineClusterLabelName: clusterName,
							},
						},
						Spec: clusterv1.MachineSpec{
							InfrastructureRef: v1.ObjectReference{
								Name:       bareMetalMachineName,
								Namespace:  namespaceName,
								Kind:       "BareMetalMachine",
								APIVersion: infrav1.GroupVersion.String(),
							},
							Bootstrap: clusterv1.Bootstrap{
								Data: &bootstrapData,
							},
						},
						Status: clusterv1.MachineStatus{
							BootstrapReady: true,
						},
					},
					newCluster(clusterName),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
					&bmh.BareMetalHost{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "bmh-0",
							Namespace: namespaceName,
						},
						Spec: bmh.BareMetalHostSpec{},
						Status: bmh.BareMetalHostStatus{
							Provisioning: bmh.ProvisionStatus{
								State: bmh.StateProvisioned,
							},
						},
					},
				},
				ErrorExpected:   false,
				RequeueExpected: true,
			},
		),
		//Bootstrap data available on machine
		Entry("Should not return an error when bootstrap data is available",
			TestCaseReconcile{
				Objects: []runtime.Object{
					&infrav1.BareMetalMachine{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      bareMetalMachineName,
							Namespace: namespaceName,
							Annotations: map[string]string{
								baremetal.HostAnnotation: "testNameSpace/bmh-0",
							},
							OwnerReferences: []metav1.OwnerReference{*bmmOwnerRef},
						},
						Spec:   infrav1.BareMetalMachineSpec{},
						Status: infrav1.BareMetalMachineStatus{},
					},
					&clusterv1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      machineName,
							Namespace: namespaceName,
							Labels: map[string]string{
								clusterv1.MachineClusterLabelName: clusterName,
							},
						},
						Spec: clusterv1.MachineSpec{
							InfrastructureRef: v1.ObjectReference{
								Name:       bareMetalMachineName,
								Namespace:  namespaceName,
								Kind:       "BareMetalMachine",
								APIVersion: infrav1.GroupVersion.String(),
							},
							Bootstrap: clusterv1.Bootstrap{
								Data: &bootstrapData,
							},
						},
						Status: clusterv1.MachineStatus{
							BootstrapReady: true,
						},
					},
					newCluster(clusterName),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
					&bmh.BareMetalHost{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "bmh-0",
							Namespace: namespaceName,
						},
						Spec: bmh.BareMetalHostSpec{},
						Status: bmh.BareMetalHostStatus{
							Provisioning: bmh.ProvisionStatus{
								State: bmh.StateProvisioned,
							},
						},
					},
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//Given deletion timestamp, delete is reconciled
		Entry("Should not return an error and finish deletion of BareMetalMachine",
			TestCaseReconcile{
				Objects: []runtime.Object{
					&corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Secret",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      bareMetalMachineName + "-user-data",
							Namespace: namespaceName,
						},

						Type: "Opaque",
					},
					&infrav1.BareMetalMachine{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:              bareMetalMachineName,
							Namespace:         namespaceName,
							DeletionTimestamp: &deletionTimestamp,
							OwnerReferences:   []metav1.OwnerReference{*bmmOwnerRef},
						},
						Spec: infrav1.BareMetalMachineSpec{
							UserData: &corev1.SecretReference{
								Name:      bareMetalMachineName + "-user-data",
								Namespace: namespaceName,
							},
						},
						Status: infrav1.BareMetalMachineStatus{},
					},
					newMachine(clusterName, machineName, bareMetalMachineName),
					newCluster(clusterName),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//Given deletion timestamp, delete is reconciled and requeued
		Entry("Should not return an error and deprovision bmh",
			TestCaseReconcile{
				Objects: []runtime.Object{
					&corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Secret",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      bareMetalMachineName + "-user-data",
							Namespace: namespaceName,
						},

						Type: "Opaque",
					},
					&infrav1.BareMetalMachine{
						TypeMeta: metav1.TypeMeta{
							Kind:       "BareMetalMachine",
							APIVersion: infrav1.GroupVersion.String(),
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:              bareMetalMachineName,
							Namespace:         namespaceName,
							DeletionTimestamp: &deletionTimestamp,
							Annotations: map[string]string{
								baremetal.HostAnnotation: "testNameSpace/bmh-0",
							},
							OwnerReferences: []metav1.OwnerReference{*bmmOwnerRef},
						},
						Spec: infrav1.BareMetalMachineSpec{
							UserData: &corev1.SecretReference{
								Name:      bareMetalMachineName + "-user-data",
								Namespace: namespaceName,
							},
						},
						Status: infrav1.BareMetalMachineStatus{},
					},
					newMachine(clusterName, machineName, bareMetalMachineName),
					newCluster(clusterName),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
					&bmh.BareMetalHost{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "bmh-0",
							Namespace: namespaceName,
						},
						Spec: bmh.BareMetalHostSpec{
							ConsumerRef: &corev1.ObjectReference{
								Name:       bareMetalMachineName,
								Namespace:  namespaceName,
								Kind:       "BareMetalMachine",
								APIVersion: infrav1.GroupVersion.String(),
							},
							Online: true,
						},
						Status: bmh.BareMetalHostStatus{},
					},
				},
				ErrorExpected:   false,
				RequeueExpected: true,
			},
		),
	)

	// Legacy tests
	It("TestBareMetalMachineReconciler_BareMetalClusterToBareMetalMachines", func() {
		baremetalCluster := newBareMetalCluster("my-baremetal-cluster",
			bmcOwnerRef, bmcSpec, nil,
		)
		objects := []runtime.Object{
			newCluster(clusterName),
			baremetalCluster,
			newMachine(clusterName, "my-machine-0", "my-baremetal-machine-0"),
			newMachine(clusterName, "my-machine-1", "my-baremetal-machine-1"),
			// Intentionally omitted
			newMachine(clusterName, "my-machine-2", ""),
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
		baremetalCluster := newBareMetalCluster("my-baremetal-cluster",
			bmcOwnerRef, bmcSpec, nil,
		)
		objects := []runtime.Object{
			baremetalCluster,
			newMachine(clusterName, "my-machine-0", "my-baremetal-machine-0"),
			newMachine(clusterName, "my-machine-1", "my-baremetal-machine-1"),
			// Intentionally omitted
			newMachine(clusterName, "my-machine-2", ""),
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

	type TestCaseBMHToBMM struct {
		Host          *bmh.BareMetalHost
		ExpectRequest bool
	}

	DescribeTable("BareMetalHost To BareMetalMachines tests",
		func(tc TestCaseBMHToBMM) {
			r := BareMetalMachineReconciler{}
			obj := handler.MapObject{
				Object: tc.Host,
			}
			reqs := r.BareMetalHostToBareMetalMachines(obj)

			if tc.ExpectRequest {
				Expect(len(reqs)).To(Equal(1), "Expected 1 request, found %d", len(reqs))

				req := reqs[0]
				Expect(req.NamespacedName.Name).To(Equal(tc.Host.Spec.ConsumerRef.Name),
					"Expected name %s, found %s", tc.Host.Spec.ConsumerRef.Name, req.NamespacedName.Name)
				Expect(req.NamespacedName.Namespace).To(Equal(tc.Host.Spec.ConsumerRef.Namespace),
					"Expected namespace %s, found %s", tc.Host.Spec.ConsumerRef.Namespace, req.NamespacedName.Namespace)

			} else {
				Expect(len(reqs)).To(Equal(0), "Expected 0 request, found %d", len(reqs))

			}
		},
		//Given machine, but no baremetalMachine resource
		Entry("BareMetalHost To BareMetalMachines",
			TestCaseBMHToBMM{
				Host: &bmh.BareMetalHost{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host1",
						Namespace: "myns",
					},
					Spec: bmh.BareMetalHostSpec{
						ConsumerRef: &corev1.ObjectReference{
							Name:       "someothermachine",
							Namespace:  "myns",
							Kind:       "BareMetalMachine",
							APIVersion: infrav1.GroupVersion.String(),
						},
					},
				},
				ExpectRequest: true,
			},
		),
		//Given machine, but no baremetalMachine resource
		Entry("BareMetalHost To BareMetalMachines, no ConsumerRef",
			TestCaseBMHToBMM{
				Host: &bmh.BareMetalHost{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host1",
						Namespace: "myns",
					},
					Spec: bmh.BareMetalHostSpec{},
				},
				ExpectRequest: false,
			},
		),
	)

})
