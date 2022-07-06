/*
Copyright 2021 The Kubernetes Authors.

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

	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	infrav1alpha5 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha5"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientfake "k8s.io/client-go/kubernetes/fake"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var bmhuid = types.UID("63856098-4b80-11ec-81d3-0242ac130003")

var providerID = fmt.Sprintf("%s%s", baremetal.ProviderIDPrefix, bmhuid)

var bootstrapDataSecretName = "testdatasecret"

func m3mOwnerRefs() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Machine",
			Name:       machineName,
		},
	}
}

func m3mMetaWithOwnerRef() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "abc",
		Namespace:       namespaceName,
		OwnerReferences: m3mOwnerRefs(),
		Annotations:     map[string]string{},
	}
}

func m3mMetaWithDeletion() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:              "abc",
		Namespace:         namespaceName,
		DeletionTimestamp: &deletionTimestamp,
		OwnerReferences:   m3mOwnerRefs(),
		Annotations:       map[string]string{},
	}
}

func m3mMetaWithAnnotation() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "abc",
		Namespace:       namespaceName,
		OwnerReferences: m3mOwnerRefs(),
		Annotations: map[string]string{
			baremetal.HostAnnotation: namespaceName + "/bmh-0",
		},
	}
}

func m3mMetaWithAnnotationDeletion() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:              "abc",
		Namespace:         namespaceName,
		DeletionTimestamp: &deletionTimestamp,
		OwnerReferences:   m3mOwnerRefs(),
		Annotations: map[string]string{
			baremetal.HostAnnotation: namespaceName + "/bmh-0",
		},
	}
}

func userDataSecret() *v1.Secret {
	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      metal3machineName + "-user-data",
			Namespace: namespaceName,
		},

		Type: "Opaque",
	}
}

func m3mSpecWithSecret() *infrav1alpha5.Metal3MachineSpec {
	return &infrav1alpha5.Metal3MachineSpec{
		UserData: &v1.SecretReference{
			Name:      metal3machineName + "-user-data",
			Namespace: namespaceName,
		},
	}
}

func metal3machineWithOwnerRefs() *infrav1alpha5.Metal3Machine {
	return newMetal3Machine(
		metal3machineName, m3mMetaWithOwnerRef(), nil, nil, false,
	)
}

func machineWithInfra() *clusterv1.Machine {
	return newMachine(clusterName, machineName, metal3machineName)
}

func machineWithBootstrap() *clusterv1.Machine {
	machine := newMachine(
		clusterName, machineName, metal3machineName,
	)
	machine.Spec.Bootstrap = clusterv1.Bootstrap{
		DataSecretName: &bootstrapDataSecretName,
	}
	machine.Status = clusterv1.MachineStatus{
		BootstrapReady: true,
	}
	return machine
}

var _ = Describe("Reconcile metal3machine", func() {

	type TestCaseReconcile struct {
		Objects                    []client.Object
		TargetObjects              []runtime.Object
		ErrorExpected              bool
		RequeueExpected            bool
		ErrorReasonExpected        bool
		ErrorReason                capierrors.MachineStatusError
		ErrorType                  error
		ExpectedRequeueDuration    time.Duration
		LabelExpected              bool
		ClusterInfraReady          bool
		CheckBMFinalizer           bool
		CheckBMState               bool
		CheckBMProviderID          bool
		CheckBMProviderIDUnchanged bool
		CheckBootStrapReady        bool
		CheckBMHostCleaned         bool
		CheckBMHostProvisioned     bool
	}

	DescribeTable("Reconcile tests",
		func(tc TestCaseReconcile) {
			testmachine := &clusterv1.Machine{}
			testcluster := &clusterv1.Cluster{}
			testBMmachine := &infrav1alpha5.Metal3Machine{}
			testBMHost := &bmh.BareMetalHost{}

			c := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(tc.Objects...).Build()
			mockCapiClientGetter := func(ctx context.Context, c client.Client, cluster *clusterv1.Cluster) (
				clientcorev1.CoreV1Interface, error,
			) {
				return clientfake.NewSimpleClientset(tc.TargetObjects...).CoreV1(), nil
			}

			r := &Metal3MachineReconciler{
				Client:           c,
				ManagerFactory:   baremetal.NewManagerFactory(c),
				Log:              klogr.New(),
				CapiClientGetter: mockCapiClientGetter,
				WatchFilterValue: "",
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      metal3machineName,
					Namespace: namespaceName,
				},
			}
			ctx := context.Background()
			res, err := r.Reconcile(ctx, req)
			_ = c.Get(context.TODO(), *getKey(machineName), testmachine)
			objMeta := testmachine.ObjectMeta
			_ = c.Get(context.TODO(), *getKey(clusterName), testcluster)
			_ = c.Get(context.TODO(), *getKey(metal3machineName), testBMmachine)
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
				Expect(testBMmachine.Status.FailureReason).NotTo(BeNil())
				Expect(tc.ErrorReason).To(Equal(*testBMmachine.Status.FailureReason))
			}
			if tc.LabelExpected {
				Expect(objMeta.Labels[clusterv1.ClusterLabelName]).NotTo(BeNil())
			}
			if tc.CheckBMFinalizer {
				Expect(baremetal.Contains(testBMmachine.Finalizers, infrav1alpha5.MachineFinalizer)).To(BeTrue())
			}
			if tc.CheckBMState {
				Expect(testBMmachine.Status.Ready).To(BeTrue())
			}
			if tc.CheckBMProviderID {
				if tc.CheckBMProviderIDUnchanged {
					Expect(testBMmachine.Spec.ProviderID).NotTo(Equal(pointer.StringPtr(fmt.Sprintf("%s%s", baremetal.ProviderIDPrefix,
						string(testBMHost.ObjectMeta.UID)))))
				} else {
					Expect(testBMmachine.Spec.ProviderID).To(Equal(pointer.StringPtr(fmt.Sprintf("%s%s", baremetal.ProviderIDPrefix,
						string(testBMHost.ObjectMeta.UID)))))
				}
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
				Expect(testBMHost.Spec.Image.URL).Should(BeEquivalentTo(testBMmachine.Spec.Image.URL))
				Expect(testBMHost.Spec.Image.Checksum).Should(BeEquivalentTo(testBMmachine.Spec.Image.Checksum))
				Expect(testBMHost.Spec.Image.DiskFormat).Should(BeEquivalentTo(testBMmachine.Spec.Image.DiskFormat))
				if testBMmachine.Spec.Image.ChecksumType != nil {
					Expect(testBMHost.Spec.Image.ChecksumType).Should(BeEquivalentTo(*testBMmachine.Spec.Image.ChecksumType))
				} else {
					Expect(testBMHost.Spec.Image.ChecksumType).Should(BeEquivalentTo(""))
				}
				Expect(testBMHost.Spec.UserData).NotTo(BeNil())
				Expect(testBMHost.Spec.ConsumerRef.Name).To(Equal(testBMmachine.Name))
			}
			if tc.ClusterInfraReady {
				Expect(testcluster.Status.InfrastructureReady).To(BeTrue())
			} else {
				Expect(testcluster.Status.InfrastructureReady).To(BeFalse())
			}

		},
		//Given: machine with a metal3machine in Spec.Infra. No metal3machine object
		//Expected: No error. NotFound error is consumed by reconciler and returns nil.
		Entry("Should not return an error when metal3machine is not found",
			TestCaseReconcile{
				Objects: []client.Object{
					newMachine(clusterName, machineName, metal3machineName),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//Given: metal3machine with OwnerRef not set.
		//Expected: No error. Reconciler waits for  Machine Controller to set OwnerRef
		Entry("Should not return an error if OwnerRef is not set on Metal3Cluster",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, nil, nil, nil, false),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		//Given: metal3machine with OwnerRef set, No Machine object.
		//Expected: Error. Machine not found.
		Entry("Should return an error when Machine cannot be found",
			TestCaseReconcile{
				Objects: []client.Object{
					metal3machineWithOwnerRefs(),
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
				Objects: []client.Object{
					metal3machineWithOwnerRefs(),
					machineWithInfra(),
				},
				ErrorExpected:       true,
				ErrorReasonExpected: true,
				ErrorReason:         capierrors.InvalidConfigurationMachineError,
				RequeueExpected:     false,
			},
		),
		//Given: Machine, Metal3Machine, Cluster. No Metal3Cluster. Cluster Infra not ready
		//Expected: No Error. it should wait and not return error
		Entry("Should not return an error when owner Cluster infrastructure is not ready",
			TestCaseReconcile{
				Objects: []client.Object{
					newCluster(clusterName, nil, &clusterv1.ClusterStatus{
						InfrastructureReady: false,
					}),
					metal3machineWithOwnerRefs(),
					machineWithInfra(),
				},
				ErrorExpected:     false,
				RequeueExpected:   false,
				ClusterInfraReady: false,
			},
		),
		//Given: Machine, Metal3Machine, Cluster. No Metal3Cluster. Cluster Infra ready
		//Expected: No error. Reconciler should wait for BMC Controller to create the BMCluster
		Entry("Should not return an error when owner Cluster infrastructure is ready and BMCluster does not exist",
			TestCaseReconcile{
				Objects: []client.Object{
					metal3machineWithOwnerRefs(),
					machineWithInfra(),
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:     false,
				RequeueExpected:   false,
				ClusterInfraReady: true,
			},
		),
		//Given: Machine, Metal3Machine (No Spec/Status), Cluster, Metal3Cluster. Cluster Infra ready
		//Expected: No error. Reconciler should set Finalizer on Metal3Machine
		Entry("Should not return an error when owner Cluster infrastructure is ready and BMCluster exist",
			TestCaseReconcile{
				Objects: []client.Object{
					metal3machineWithOwnerRefs(),
					machineWithInfra(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, false),
				},
				ErrorExpected:           false,
				RequeueExpected:         false,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
				CheckBMFinalizer:        true,
			},
		),
		//Given: Machine, Metal3Machine (No Spec/Status), Cluster, Metal3Cluster.
		//Cluster.Spec.Paused=true
		//Expected: Requeue Expected
		Entry("Should requeue when owner Cluster is paused",
			TestCaseReconcile{
				Objects: []client.Object{
					metal3machineWithOwnerRefs(),
					machineWithInfra(),
					newCluster(clusterName, clusterPauseSpec(), nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, false),
				},
				ErrorExpected:           false,
				RequeueExpected:         true,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
			},
		),
		//Given: Machine, Metal3Machine (No Spec/Status), Cluster, Metal3Cluster.
		//Metal3Machine has cluster.x-k8s.io/paused annotation
		//Expected: Requeue Expected
		Entry("Should requeue when Metal3Machine has paused annotation",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, nil, nil, nil, true),
					machineWithInfra(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, false),
				},
				ErrorExpected:           false,
				RequeueExpected:         true,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
			},
		),
		//Given: M3Machine (Spec: Provider ID, Status: Ready), BMHost(Provisioned).
		//Expected: Since BMH is in provisioned state, nothing will happen since machine. bootstrapReady is false.
		Entry("Should not return an error when metal3machine is deployed",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, m3mMetaWithAnnotation(),
						&infrav1alpha5.Metal3MachineSpec{
							ProviderID: &providerID,
						},
						&infrav1alpha5.Metal3MachineStatus{
							Ready: true,
						},
						false,
					),
					machineWithInfra(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost("bmh-0", nil, nil, nil, false),
				},
				ErrorExpected:       false,
				RequeueExpected:     false,
				ClusterInfraReady:   true,
				CheckBMFinalizer:    true,
				CheckBMState:        true,
				CheckBootStrapReady: false,
			},
		),
		//Given: Machine has Bootstrap data available while M3Machine has no Host Annotation
		// BMH is in available state
		//Expected: Requeue Expected
		//			BMHost.Spec.Image = BMmachine.Spec.Image,
		// 			BMHost.Spec.UserData is populated
		// 			Expect BMHost.Spec.ConsumerRef.Name = BMmachine.Name
		Entry("Should set BMH Spec in correct state and requeue when all objects are available but no annotation, BMH state is available",
			TestCaseReconcile{
				Objects: []client.Object{

					newMetal3Machine(
						metal3machineName, m3mMetaWithOwnerRef(), &infrav1alpha5.Metal3MachineSpec{
							Image: infrav1alpha5.Image{
								Checksum: "abcd",
								URL:      "abcd",
								// Checking the pointers,
								// CheckBMHostProvisioned is true
								ChecksumType: pointer.StringPtr("sha512"),
								DiskFormat:   pointer.StringPtr("raw"),
							},
						}, nil, false,
					),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost("bmh-0", nil, &bmov1alpha1.BareMetalHostStatus{
						Provisioning: bmov1alpha1.ProvisionStatus{
							State: bmov1alpha1.StateAvailable,
						},
					}),
				},
				ErrorExpected:           false,
				RequeueExpected:         false,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
				CheckBMFinalizer:        true,
				CheckBootStrapReady:     true,
				CheckBMHostProvisioned:  true,
			},
		),
		//Given: Machine has Bootstrap data available while M3Machine has no Host Annotation
		// BMH is in ready state
		//Expected: Requeue Expected
		//			BMHost.Spec.Image = BMmachine.Spec.Image,
		// 			BMHost.Spec.UserData is populated
		// 			Expect BMHost.Spec.ConsumerRef.Name = BMmachine.Name
		Entry("Should set BMH Spec in correct state and requeue when all objects are available but no annotation",
			TestCaseReconcile{
				Objects: []client.Object{

					newMetal3Machine(
						metal3machineName, m3mMetaWithOwnerRef(), &infrav1alpha5.Metal3MachineSpec{
							Image: infrav1alpha5.Image{
								Checksum: "abcd",
								URL:      "abcd",
								//No ChecksumType and DiskFormat given to test without them
								// CheckBMHostProvisioned is true
							},
						}, nil, false,
					),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost("bmh-0", nil, &bmov1alpha1.BareMetalHostStatus{
						Provisioning: bmov1alpha1.ProvisionStatus{
							State: bmov1alpha1.StateReady,
						},
					}),
				},
				ErrorExpected:           false,
				RequeueExpected:         false,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
				CheckBMFinalizer:        true,
				CheckBootStrapReady:     true,
				CheckBMHostProvisioned:  true,
			},
		),
		//Given: Machine(with Bootstrap data), M3Machine (Annotation Given, no provider ID), BMH (provisioned)
		//Expected: No Error, BMH.Spec.ProviderID is set properly based on the UID
		Entry("Should set ProviderID when bootstrap data is available, ProviderID is not given, BMH is provisioned",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(
						metal3machineName, m3mMetaWithAnnotation(),
						&infrav1alpha5.Metal3MachineSpec{
							Image: infrav1alpha5.Image{
								Checksum: "abcd",
								URL:      "abcd",
							},
						}, nil, false,
					),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost("bmh-0", nil, nil, nil, false),
				},
				TargetObjects: []runtime.Object{
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
						},
						Spec: corev1.NodeSpec{
							ProviderID: providerID,
						},
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
		//Given: Machine(with Bootstrap data), M3Machine (Annotation Given, provider ID set), BMH (provisioned)
		//Expected: No Error, BMH.Spec.ProviderID is set properly (unchanged)
		Entry("Should set ProviderID when bootstrap data is available, ProviderID is given, BMH is provisioned",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(
						metal3machineName, m3mMetaWithAnnotation(),
						&capm3.Metal3MachineSpec{
							ProviderID: pointer.StringPtr(fmt.Sprintf("%sabcd", baremetal.ProviderIDPrefix)),
							Image: capm3.Image{
								Checksum: "abcd",
								URL:      "abcd",
							},
						}, nil, false,
					),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost("bmh-0", nil, nil, nil, false),
				},
				TargetObjects: []runtime.Object{
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
						},
						Spec: corev1.NodeSpec{
							ProviderID: providerID,
						},
					},
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-1",
							Labels: map[string]string{
								baremetal.ProviderLabelPrefix: string(bmhuid),
							},
						},
						Spec: corev1.NodeSpec{},
					},
				},
				ErrorExpected:              false,
				RequeueExpected:            false,
				ClusterInfraReady:          true,
				CheckBMFinalizer:           true,
				CheckBMProviderID:          true,
				CheckBootStrapReady:        true,
				CheckBMProviderIDUnchanged: true,
			},
		),
		//Given: Machine(with Bootstrap data), M3Machine (Annotation Given, no provider ID), BMH (provisioning)
		//Expected: No Error, Requeue expected
		//		BMH.Spec.ProviderID is not set based on the UID since BMH is in provisioning
		Entry("Should requeue when bootstrap data is available, ProviderID is not given, BMH is provisioning",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, m3mMetaWithAnnotation(), &infrav1alpha5.Metal3MachineSpec{
						Image: infrav1alpha5.Image{
							Checksum: "abcd",
							URL:      "abcd",
						},
					}, nil, false),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost("bmh-0", nil, &bmov1alpha1.BareMetalHostStatus{
						Provisioning: bmov1alpha1.ProvisionStatus{
							State: bmov1alpha1.StateProvisioning,
						},
					}),
				},
				ErrorExpected:           false,
				RequeueExpected:         false,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
				CheckBMFinalizer:        true,
				CheckBootStrapReady:     true,
			},
		),
		//Given: metal3machine with annotation to a BMH provisioned, machine with
		// bootstrap data, no target cluster node available
		//Expected: no error, requeing. ProviderID should not be set.
		Entry("Should requeue when patching an unavailable node",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, m3mMetaWithAnnotation(), nil, nil, false),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, nil, false),
					newBareMetalHost("bmh-0", nil, nil, nil, false),
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
		//Given: metal3machine with annotation to a BMH provisioned, machine with
		// bootstrap data, target cluster node available
		//Expected: no error, no requeue. ProviderID should be set.
		Entry("Should not requeue when patching an available node",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, m3mMetaWithAnnotation(), nil, nil, false),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, nil, false),
					newBareMetalHost("bmh-0", nil, nil, nil, false),
				},
				TargetObjects: []runtime.Object{
					&v1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Labels: map[string]string{
								baremetal.ProviderLabelPrefix: "54db7dd5-269a-4d94-a12a-c4eafcecb8e7",
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
		//Given: Deletion timestamp on M3Machine, No BMHost Given
		//Expected: Delete is reconciled,M3Machine Finalizer is removed
		Entry("Should not return an error and finish deletion of Metal3Machine",
			TestCaseReconcile{
				Objects: []client.Object{
					userDataSecret(),
					newMetal3Machine(metal3machineName, m3mMetaWithDeletion(),
						m3mSpecWithSecret(), nil, false,
					),
					machineWithInfra(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, false),
				},
				ErrorExpected:     false,
				RequeueExpected:   false,
				ClusterInfraReady: true,
				CheckBMFinalizer:  false,
			},
		),
		//Given: Deletion timestamp on M3Machine, BMHost Given
		//Expected: Requeue Expected
		//          Delete is reconciled. BMH should be deprovisioned
		Entry("Should not return an error and deprovision bmh",
			TestCaseReconcile{
				Objects: []client.Object{
					userDataSecret(),
					newMetal3Machine(metal3machineName,
						m3mMetaWithAnnotationDeletion(), m3mSpecWithSecret(), nil, false,
					),
					machineWithInfra(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost("bmh-0", &bmov1alpha1.BareMetalHostSpec{
						ConsumerRef: &corev1.ObjectReference{
							Name:       metal3machineName,
							Namespace:  namespaceName,
							Kind:       "Metal3Machine",
							APIVersion: infrav1alpha5.GroupVersion.String(),
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
})
