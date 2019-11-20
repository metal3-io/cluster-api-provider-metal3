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
	"context"
	"fmt"
	"reflect"
	"time"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientfake "k8s.io/client-go/kubernetes/fake"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Reconcile Baremetalcluster", func() {

	providerID := "metal3:///foo/bar"

	bootstrapData := "Qm9vdHN0cmFwIERhdGEK"

	bmmOwnerRefs := []metav1.OwnerReference{
		{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Machine",
			Name:       machineName,
		},
	}

	bmmMetaWithOwnerRef := &metav1.ObjectMeta{
		Name:            "abc",
		Namespace:       namespaceName,
		OwnerReferences: bmmOwnerRefs,
		Annotations:     map[string]string{},
	}

	bmmMetaWithDeletion := &metav1.ObjectMeta{
		Name:              "abc",
		Namespace:         namespaceName,
		DeletionTimestamp: &deletionTimestamp,
		OwnerReferences:   bmmOwnerRefs,
		Annotations:       map[string]string{},
	}

	bmmMetaWithAnnotation := &metav1.ObjectMeta{
		Name:            "abc",
		Namespace:       namespaceName,
		OwnerReferences: bmmOwnerRefs,
		Annotations: map[string]string{
			baremetal.HostAnnotation: "testNameSpace/bmh-0",
		},
	}

	bmmMetaWithAnnotationDeletion := &metav1.ObjectMeta{
		Name:              "abc",
		Namespace:         namespaceName,
		DeletionTimestamp: &deletionTimestamp,
		OwnerReferences:   bmmOwnerRefs,
		Annotations: map[string]string{
			baremetal.HostAnnotation: "testNameSpace/bmh-0",
		},
	}

	userDataSecret := &corev1.Secret{
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

	bmmSpecWithSecret := &infrav1.BareMetalMachineSpec{
		UserData: &corev1.SecretReference{
			Name:      bareMetalMachineName + "-user-data",
			Namespace: namespaceName,
		},
	}

	bareMetalMachineWithOwnerRefs := newBareMetalMachine(
		bareMetalMachineName, bmmMetaWithOwnerRef, nil, nil,
	)

	bmcSpec := &infrav1.BareMetalClusterSpec{
		APIEndpoint:     "http://192.168.111.249:6443",
		NoCloudProvider: true,
	}

	machineWithInfra := newMachine(clusterName, machineName, bareMetalMachineName)
	machineWithBootstrap := newMachine(
		clusterName, machineName, bareMetalMachineName,
	)
	machineWithBootstrap.Spec.Bootstrap = clusterv1.Bootstrap{
		Data: &bootstrapData,
	}
	machineWithBootstrap.Status = clusterv1.MachineStatus{
		BootstrapReady: true,
	}

	type TestCaseReconcile struct {
		Objects                 []runtime.Object
		TargetObjects           []runtime.Object
		ErrorExpected           bool
		RequeueExpected         bool
		ErrorReasonExpected     bool
		ErrorReason             capierrors.MachineStatusError
		ErrorType               error
		ExpectedRequeueDuration time.Duration
		LabelExpected           bool
		ClusterInfraReady       bool
		CheckBMFinalizer        bool
		CheckBMState            bool
		CheckBMProviderID       bool
		CheckBootStrapReady     bool
		CheckBMHostCleaned      bool
		CheckBMHostProvisioned  bool
	}

	DescribeTable("Reconcile tests",
		func(tc TestCaseReconcile) {
			testmachine := &clusterv1.Machine{}
			testcluster := &clusterv1.Cluster{}
			testBMmachine := &infrav1.BareMetalMachine{}
			testBMHost := &bmh.BareMetalHost{}

			c := fake.NewFakeClientWithScheme(setupScheme(), tc.Objects...)
			mockCapiClientGetter := func(c client.Client, cluster *capi.Cluster) (
				clientcorev1.CoreV1Interface, error,
			) {
				return clientfake.NewSimpleClientset(tc.TargetObjects...).CoreV1(), nil
			}

			r := &BareMetalMachineReconciler{
				Client:           c,
				ManagerFactory:   baremetal.NewManagerFactory(c),
				Log:              klogr.New(),
				CapiClientGetter: mockCapiClientGetter,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      bareMetalMachineName,
					Namespace: namespaceName,
				},
			}
			res, err := r.Reconcile(req)
			_ = c.Get(context.TODO(), *getKey(machineName), testmachine)
			objMeta := testmachine.ObjectMeta
			_ = c.Get(context.TODO(), *getKey(clusterName), testcluster)
			_ = c.Get(context.TODO(), *getKey(bareMetalMachineName), testBMmachine)
			_ = c.Get(context.TODO(), *getKey("bmh-0"), testBMHost)

			if tc.ErrorExpected {
				Expect(err).To(HaveOccurred())
				if tc.ErrorType != nil {
					Expect(reflect.TypeOf(tc.ErrorType) == reflect.TypeOf(errors.Cause(err))).To(BeTrue())
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.RequeueExpected {
				Expect(res.Requeue).NotTo(BeFalse())
				Expect(res.RequeueAfter).To(Equal(tc.ExpectedRequeueDuration))
			} else {
				Expect(res.Requeue).To(BeFalse())
			}
			if tc.ErrorReasonExpected {
				Expect(testBMmachine.Status.ErrorReason).NotTo(BeNil())
				Expect(tc.ErrorReason).To(Equal(*testBMmachine.Status.ErrorReason))
			}
			if tc.LabelExpected {
				Expect(objMeta.Labels[clusterv1.MachineClusterLabelName]).NotTo(BeNil())
			}
			if tc.CheckBMFinalizer {
				Expect(util.Contains(testBMmachine.Finalizers, infrav1.MachineFinalizer)).To(BeTrue())
			}
			if tc.CheckBMState {
				Expect(testBMmachine.Status.Ready).To(BeTrue())
			}
			if tc.CheckBMProviderID {
				Expect(testBMmachine.Spec.ProviderID).To(Equal(pointer.StringPtr(fmt.Sprintf("metal3://%s",
					string(testBMHost.ObjectMeta.UID)))))
			}
			if tc.CheckBootStrapReady {
				Expect(testmachine.Status.BootstrapReady).To(BeTrue())
			} else {
				Expect(testmachine.Status.BootstrapReady).To(BeFalse())
			}
			if tc.CheckBMHostCleaned {
				Expect(testBMHost.Spec.Image).To(BeNil())
				Expect(testBMHost.Spec.Online).To(BeFalse())
				Expect(testBMHost.Spec.UserData).To(BeNil())
			}
			if tc.CheckBMHostProvisioned {
				Expect(*testBMHost.Spec.Image).Should(BeEquivalentTo(testBMmachine.Spec.Image))
				Expect(testBMHost.Spec.UserData).NotTo(BeNil())
				Expect(testBMHost.Spec.ConsumerRef.Name).To(Equal(testBMmachine.Name))
			}
			if tc.ClusterInfraReady {
				Expect(testcluster.Status.InfrastructureReady).To(BeTrue())
			} else {
				Expect(testcluster.Status.InfrastructureReady).To(BeFalse())
			}

		},
		//Given: machine with a bareMetalMachine in Spec.Infra. No baremetalMachine object
		//Expected: No error. NotFound error is consumed by reconciler and returns nil.
		Entry("Should not return an error when baremetalMachine is not found",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newMachine(clusterName, machineName, bareMetalMachineName),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//Given: baremetalMachine with OwnerRef not set.
		//Expected: No error. Reconciler waits for  Machine Controller to set OwnerRef
		Entry("Should not return an error if OwnerRef is not set on BareMetalCluster",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(bareMetalMachineName, nil, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//Given: baremetalMachine with OwnerRef set, No Machine object.
		//Expected: Error. Machine not found.
		Entry("Should return an error when Machine cannot be found",
			TestCaseReconcile{
				Objects: []runtime.Object{
					bareMetalMachineWithOwnerRefs,
				},
				ErrorExpected:   true,
				ErrorType:       &apierrors.StatusError{},
				RequeueExpected: false,
			},
		),
		//Given: Machine with cluster label. but no Cluster object.
		//Expected: Error. Cluster not found
		Entry("Should return an error when owner Cluster cannot be found",
			TestCaseReconcile{
				Objects: []runtime.Object{
					bareMetalMachineWithOwnerRefs,
					machineWithInfra,
				},
				ErrorExpected:       true,
				ErrorReasonExpected: true,
				ErrorReason:         capierrors.InvalidConfigurationMachineError,
				RequeueExpected:     false,
			},
		),
		//Given: Machine, BareMetalMachine, Cluster. No BareMetalCluster. Cluster Infra not ready
		//Expected: No Error. it should wait and not return error
		Entry("Should not return an error when owner Cluster infrastructure is not ready",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newCluster(clusterName, nil, &clusterv1.ClusterStatus{
						InfrastructureReady: false,
					}),
					bareMetalMachineWithOwnerRefs,
					machineWithInfra,
				},
				ErrorExpected:     false,
				RequeueExpected:   false,
				ClusterInfraReady: false,
			},
		),
		//Given: Machine, BareMetalMachine, Cluster. No BareMetalCluster. Cluster Infra ready
		//Expected: No error. Reconciler should wait for BMC Controller to create the BMCluster
		Entry("Should not return an error when owner Cluster infrastructure is ready and BMCluster does not exist",
			TestCaseReconcile{
				Objects: []runtime.Object{
					bareMetalMachineWithOwnerRefs,
					machineWithInfra,
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:     false,
				RequeueExpected:   false,
				ClusterInfraReady: true,
			},
		),
		//Given: Machine, BareMetalMachine (No Spec/Status), Cluster, BareMetalCluster. Cluster Infra ready
		//Expected: No error. Reconciler should set Finalizer on BareMetalMachine
		Entry("Should not return an error when owner Cluster infrastructure is ready and BMCluster exist",
			TestCaseReconcile{
				Objects: []runtime.Object{
					bareMetalMachineWithOwnerRefs,
					machineWithInfra,
					newCluster(clusterName, nil, nil),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
				},
				ErrorExpected:     false,
				RequeueExpected:   false,
				ClusterInfraReady: true,
				CheckBMFinalizer:  true,
			},
		),
		//Given: BMMachine (Spec: Provider ID, Status: Ready), BMHost(Provisioned).
		//Expected: Since BMH is in provisioned state, nothing will happen since machine. bootstrapReady is false.
		Entry("Should not return an error when BaremetalMachine is deployed",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(bareMetalMachineName, bmmMetaWithAnnotation,
						&infrav1.BareMetalMachineSpec{
							ProviderID: &providerID,
						},
						&infrav1.BareMetalMachineStatus{
							Ready: true,
						},
					),
					machineWithInfra,
					newCluster(clusterName, nil, nil),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
					newBareMetalHost(nil, nil),
				},
				ErrorExpected:       false,
				RequeueExpected:     false,
				ClusterInfraReady:   true,
				CheckBMFinalizer:    true,
				CheckBMState:        true,
				CheckBootStrapReady: false,
			},
		),
		//Given: Machine has Bootstrap data available while BMMachine has no Host Annotation
		// BMH is in ready state
		//Expected: Requeue Expected
		//			BMHost.Spec.Image = BMmachine.Spec.Image,
		// 			BMHost.Spec.UserData is populated
		// 			Expect BMHost.Spec.ConsumerRef.Name = BMmachine.Name
		Entry("Should set BMH Spec in correct state and requeue when all objects are available but no annotation",
			TestCaseReconcile{
				Objects: []runtime.Object{

					newBareMetalMachine(
						bareMetalMachineName, bmmMetaWithOwnerRef, &infrav1.BareMetalMachineSpec{
							Image: infrav1.Image{
								Checksum: "abcd",
								URL:      "abcd",
							},
						}, nil,
					),
					machineWithBootstrap,
					newCluster(clusterName, nil, nil),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
					newBareMetalHost(nil, &bmh.BareMetalHostStatus{
						Provisioning: bmh.ProvisionStatus{
							State: bmh.StateReady,
						},
					}),
				},
				ErrorExpected:           false,
				RequeueExpected:         true,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
				CheckBMFinalizer:        true,
				CheckBootStrapReady:     true,
				CheckBMHostProvisioned:  true,
			},
		),
		//Given: Machine(with Bootstrap data), BMMachine (Annotation Given, no provider ID), BMH (provisioned)
		//Expected: No Error, BMH.Spec.ProviderID is set properly based on the UID
		Entry("Should set ProviderID when bootstrap data is available, ProviderID is not given, BMH is provisioned",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(
						bareMetalMachineName, bmmMetaWithAnnotation,
						&infrav1.BareMetalMachineSpec{
							Image: infrav1.Image{
								Checksum: "abcd",
								URL:      "abcd",
							},
						}, nil,
					),
					machineWithBootstrap,
					newCluster(clusterName, nil, nil),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
					newBareMetalHost(nil, nil),
				},
				ErrorExpected:       false,
				RequeueExpected:     false,
				ClusterInfraReady:   true,
				CheckBMFinalizer:    true,
				CheckBMProviderID:   true,
				CheckBootStrapReady: true,
			},
		),
		//Given: Machine(with Bootstrap data), BMMachine (Annotation Given, no provider ID), BMH (provisioning)
		//Expected: No Error, Requeue expected
		//		BMH.Spec.ProviderID is not set based on the UID since BMH is in provisioning
		Entry("Should requeue when bootstrap data is available, ProviderID is not given, BMH is provisioning",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(bareMetalMachineName, bmmMetaWithAnnotation, &infrav1.BareMetalMachineSpec{
						Image: infrav1.Image{
							Checksum: "abcd",
							URL:      "abcd",
						},
					}, nil),
					machineWithBootstrap,
					newCluster(clusterName, nil, nil),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
					newBareMetalHost(nil, &bmh.BareMetalHostStatus{
						Provisioning: bmh.ProvisionStatus{
							State: bmh.StateProvisioning,
						},
					}),
				},
				ErrorExpected:           false,
				RequeueExpected:         true,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
				CheckBMFinalizer:        true,
				CheckBootStrapReady:     true,
			},
		),
		//Given: Baremetalmachine with annotation to a BMH provisioned, machine with
		// bootstrap data, no target cluster node available
		//Expected: no error, requeing. ProviderID should not be set.
		Entry("Should requeue when patching an unavailable node",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(bareMetalMachineName, bmmMetaWithAnnotation, nil, nil),
					machineWithBootstrap,
					newCluster(clusterName, nil, nil),
					newBareMetalCluster(baremetalClusterName, bmcOwnerRef, bmcSpec, nil),
					newBareMetalHost(nil, nil),
				},
				ErrorExpected:           false,
				RequeueExpected:         true,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
				CheckBMFinalizer:        true,
				CheckBMProviderID:       false,
				CheckBootStrapReady:     true,
			},
		),
		//Given: Baremetalmachine with annotation to a BMH provisioned, machine with
		// bootstrap data, target cluster node available
		//Expected: no error, no requeue. ProviderID should be set.
		Entry("Should not requeue when patching an available node",
			TestCaseReconcile{
				Objects: []runtime.Object{
					newBareMetalMachine(bareMetalMachineName, bmmMetaWithAnnotation, nil, nil),
					machineWithBootstrap,
					newCluster(clusterName, nil, nil),
					newBareMetalCluster(baremetalClusterName, bmcOwnerRef, bmcSpec, nil),
					newBareMetalHost(nil, nil),
				},
				TargetObjects: []runtime.Object{
					&v1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bmh-0",
							Labels: map[string]string{
								"metal3.io/uuid": "54db7dd5-269a-4d94-a12a-c4eafcecb8e7",
							},
						},
						Spec: v1.NodeSpec{},
					},
				},
				ErrorExpected:       false,
				RequeueExpected:     false,
				ClusterInfraReady:   true,
				CheckBMFinalizer:    true,
				CheckBMProviderID:   true,
				CheckBootStrapReady: true,
			},
		),
		//Given: Deletion timestamp on BMMachine, No BMHost Given
		//Expected: Delete is reconciled,BMMachine Finalizer is removed
		Entry("Should not return an error and finish deletion of BareMetalMachine",
			TestCaseReconcile{
				Objects: []runtime.Object{
					userDataSecret,
					newBareMetalMachine(bareMetalMachineName, bmmMetaWithDeletion,
						bmmSpecWithSecret, nil,
					),
					machineWithInfra,
					newCluster(clusterName, nil, nil),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
				},
				ErrorExpected:     false,
				RequeueExpected:   false,
				ClusterInfraReady: true,
				CheckBMFinalizer:  false,
			},
		),
		//Given: Deletion timestamp on BMMachine, BMHost Given
		//Expected: Requeue Expected
		//          Delete is reconciled. BMH should be deprovisioned
		Entry("Should not return an error and deprovision bmh",
			TestCaseReconcile{
				Objects: []runtime.Object{
					userDataSecret,
					newBareMetalMachine(bareMetalMachineName,
						bmmMetaWithAnnotationDeletion, bmmSpecWithSecret, nil,
					),
					machineWithInfra,
					newCluster(clusterName, nil, nil),
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
					newBareMetalHost(&bmh.BareMetalHostSpec{
						ConsumerRef: &corev1.ObjectReference{
							Name:       bareMetalMachineName,
							Namespace:  namespaceName,
							Kind:       "BareMetalMachine",
							APIVersion: infrav1.GroupVersion.String(),
						},
						Online: true,
					}, &bmh.BareMetalHostStatus{}),
				},
				ErrorExpected:           false,
				RequeueExpected:         true,
				ExpectedRequeueDuration: time.Second * 0,
				ClusterInfraReady:       true,
				CheckBMHostCleaned:      true,
			},
		),
	)

	// Legacy tests
	It("TestBareMetalMachineReconciler_BareMetalClusterToBareMetalMachines", func() {
		baremetalCluster := newBareMetalCluster("my-baremetal-cluster",
			bmcOwnerRef, bmcSpec, nil,
		)
		objects := []runtime.Object{
			newCluster(clusterName, nil, nil),
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
