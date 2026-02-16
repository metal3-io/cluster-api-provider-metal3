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

	"github.com/go-logr/logr"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientfake "k8s.io/client-go/kubernetes/fake"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var bmhuid = types.UID("63856098-4b80-11ec-81d3-0242ac130003")

var providerID = fmt.Sprintf("%s%s", baremetal.ProviderIDPrefix, bmhuid)

var bootstrapDataSecretName = "testdatasecret"

var v1beta1BootstrapReadyTrue = clusterv1.Conditions{clusterv1.Condition{Type: clusterv1.BootstrapReadyV1Beta1Condition, Status: corev1.ConditionTrue}}

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
		Name:            metal3machineName,
		Namespace:       namespaceName,
		OwnerReferences: m3mOwnerRefs(),
		Annotations:     map[string]string{},
	}
}

func m3mMetaWithDeletion() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:              metal3machineName,
		Namespace:         namespaceName,
		DeletionTimestamp: &deletionTimestamp,
		Finalizers:        []string{"foo"},
		OwnerReferences:   m3mOwnerRefs(),
		Annotations:       map[string]string{},
	}
}

func m3mMetaWithAnnotation() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            metal3machineName,
		Namespace:       namespaceName,
		OwnerReferences: m3mOwnerRefs(),
		Annotations: map[string]string{
			baremetal.HostAnnotation: namespaceName + "/" + baremetalhostName,
		},
	}
}

func m3mMetaWithIncorrectAnnotation() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            metal3machineName,
		Namespace:       namespaceName,
		OwnerReferences: m3mOwnerRefs(),
		Annotations: map[string]string{
			baremetal.HostAnnotation: "///incorrectFormat",
		},
	}
}

func m3mMetaWithAnnotationDeletion() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:              metal3machineName,
		Namespace:         namespaceName,
		DeletionTimestamp: &deletionTimestamp,
		Finalizers:        []string{"foo"},
		OwnerReferences:   m3mOwnerRefs(),
		Annotations: map[string]string{
			baremetal.HostAnnotation: namespaceName + "/" + baremetalhostName,
		},
	}
}

func userDataSecret() *corev1.Secret {
	return &corev1.Secret{
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

func m3mSpecWithSecret() *infrav1.Metal3MachineSpec {
	return &infrav1.Metal3MachineSpec{
		UserData: &corev1.SecretReference{
			Name:      metal3machineName + "-user-data",
			Namespace: namespaceName,
		},
	}
}

func metal3machineWithOwnerRefs() *infrav1.Metal3Machine {
	return newMetal3Machine(
		metal3machineName, m3mMetaWithOwnerRef(), nil, nil, false,
	)
}

func machineWithInfra() *clusterv1.Machine {
	return newMachine(clusterName, machineName, metal3machineName, "")
}

func machineWithBootstrap() *clusterv1.Machine {
	machine := newMachine(
		clusterName, machineName, metal3machineName, "",
	)
	machine.Spec.Bootstrap = clusterv1.Bootstrap{
		ConfigRef:      clusterv1.ContractVersionedObjectReference{},
		DataSecretName: &bootstrapDataSecretName,
	}
	machine.Status = clusterv1.MachineStatus{
		Deprecated: &clusterv1.MachineDeprecatedStatus{
			V1Beta1: &clusterv1.MachineV1Beta1DeprecatedStatus{
				Conditions: v1beta1BootstrapReadyTrue,
			},
		},
	}
	return machine
}

func machineWithDataSecret() *clusterv1.Machine {
	machine := newMachine(
		clusterName, machineName, metal3machineName, "",
	)
	machine.Spec.Bootstrap = clusterv1.Bootstrap{
		DataSecretName: &bootstrapDataSecretName,
	}
	machine.Status = clusterv1.MachineStatus{
		Deprecated: &clusterv1.MachineDeprecatedStatus{
			V1Beta1: &clusterv1.MachineV1Beta1DeprecatedStatus{
				Conditions: v1beta1BootstrapReadyTrue,
			},
		},
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
		ConditionsExpected         []metav1.Condition
		ExpectedRequeueDuration    time.Duration
		LabelExpected              bool
		ClusterInfraReady          bool
		CheckBMFinalizer           bool
		CheckBMState               bool
		CheckBMProviderID          bool
		CheckBMProviderIDUnchanged bool
		CheckBMProviderIDNew       bool
		CheckBootStrapReady        bool
		CheckBMHostCleaned         bool
		CheckBMHostProvisioned     bool
		ExpectedOnlineStatus       bool
	}

	DescribeTable("Reconcile tests",
		func(tc TestCaseReconcile) {
			testmachine := &clusterv1.Machine{}
			testcluster := &clusterv1.Cluster{}
			testBMmachine := &infrav1.Metal3Machine{}
			testBMHost := &bmov1alpha1.BareMetalHost{}
			oldProviderID := testBMmachine.Spec.ProviderID

			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(tc.Objects...).WithStatusSubresource(tc.Objects...).Build()
			mockCapiClientGetter := func(_ context.Context, _ client.Client, _ *clusterv1.Cluster) (
				clientcorev1.CoreV1Interface, error,
			) {
				return clientfake.NewSimpleClientset(tc.TargetObjects...).CoreV1(), nil
			}

			r := &Metal3MachineReconciler{
				Client:           fakeClient,
				ManagerFactory:   baremetal.NewManagerFactory(fakeClient),
				Log:              logr.Discard(),
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
			_ = fakeClient.Get(context.TODO(), *getKey(machineName), testmachine)
			objMeta := testmachine.ObjectMeta
			_ = fakeClient.Get(context.TODO(), *getKey(clusterName), testcluster)
			_ = fakeClient.Get(context.TODO(), *getKey(metal3machineName), testBMmachine)
			_ = fakeClient.Get(context.TODO(), *getKey(baremetalhostName), testBMHost)

			if tc.ErrorExpected {
				Expect(err).To(HaveOccurred())
				if tc.ErrorType != nil {
					Expect(reflect.TypeOf(tc.ErrorType)).To(BeIdenticalTo(reflect.TypeOf(baremetal.RootCause(err))))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.RequeueExpected {
				Expect(res.Requeue).To(BeTrue())
				Expect(res.RequeueAfter).To(Equal(tc.ExpectedRequeueDuration))
			} else {
				Expect(res.Requeue).To(BeFalse())
			}
			if tc.ErrorReasonExpected {
				Expect(testBMmachine.Status.Deprecated).NotTo(BeNil())
				Expect(testBMmachine.Status.Deprecated.V1Beta1).NotTo(BeNil())
				Expect(testBMmachine.Status.Deprecated.V1Beta1.FailureReason).NotTo(BeNil())
				Expect(tc.ErrorReason).To(Equal(*testBMmachine.Status.Deprecated.V1Beta1.FailureReason))
			}
			for _, condExp := range tc.ConditionsExpected {
				_ = fakeClient.Get(ctx, *getKey(metal3ClusterName), testBMmachine)
				condGot := conditions.Get(testBMmachine, condExp.Type)
				Expect(condGot).NotTo(BeNil(), fmt.Sprintf("Expected condition type=%s status=%s reason=%s", condExp.Type, string(condExp.Status), condExp.Reason))
				Expect(condGot.Status).To(Equal(condExp.Status))
				if condExp.Reason != "" {
					Expect(condGot.Reason).To(Equal(condExp.Reason))
				}
			}
			if tc.LabelExpected {
				Expect(objMeta.Labels[clusterv1.ClusterNameLabel]).NotTo(BeNil())
			}
			if tc.CheckBMFinalizer {
				Expect(baremetal.Contains(testBMmachine.Finalizers, infrav1.MachineFinalizer)).To(BeTrue())
			}
			if tc.CheckBMState {
				Expect(testBMmachine.Status.Ready).To(BeTrue())
			}
			if tc.CheckBMProviderID {
				if tc.CheckBMProviderIDUnchanged {
					Expect(testBMmachine.Spec.ProviderID).To(Equal(oldProviderID))
				} else {
					if tc.CheckBMProviderIDNew {
						Expect(testBMmachine.Spec.ProviderID).To(Equal(ptr.To(fmt.Sprintf("%s%s/%s/%s", baremetal.ProviderIDPrefix,
							testBMHost.ObjectMeta.Namespace, testBMHost.ObjectMeta.Name, testBMmachine.ObjectMeta.Name))))
					} else {
						Expect(testBMmachine.Spec.ProviderID).To(Equal(ptr.To(fmt.Sprintf("%s%s", baremetal.ProviderIDPrefix,
							string(testBMHost.ObjectMeta.UID)))))
					}
				}
			}
			if tc.CheckBootStrapReady {
				// Expect(testmachine.Status.BootstrapReady).To(BeTrue())
				bootstrapReadyCondition := deprecatedv1beta1conditions.Get(testmachine, clusterv1.BootstrapReadyV1Beta1Condition)
				Expect(bootstrapReadyCondition.Status).To(Equal(corev1.ConditionTrue))
			} else {
				// Expect(testmachine.Status.BootstrapReady).To(BeFalse())
				bootstrapReadyCondition := deprecatedv1beta1conditions.Get(testmachine, clusterv1.BootstrapReadyV1Beta1Condition)
				Expect(bootstrapReadyCondition).To(BeNil())
			}
			if tc.CheckBMHostCleaned {
				Expect(testBMHost.Spec.Image).To(BeNil())
				Expect(testBMHost.Spec.UserData).To(BeNil())
				Expect(testBMHost.Spec.Online).To(Equal(tc.ExpectedOnlineStatus))
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
				infrastructureReadyCondition := deprecatedv1beta1conditions.Get(testcluster, clusterv1.InfrastructureReadyV1Beta1Condition)
				Expect(infrastructureReadyCondition.Status).To(Equal(corev1.ConditionTrue))
			} else {
				infrastructureReadyCondition := deprecatedv1beta1conditions.Get(testcluster, clusterv1.InfrastructureReadyV1Beta1Condition)
				if infrastructureReadyCondition != nil {
					Expect(infrastructureReadyCondition.Status).To(Equal(corev1.ConditionFalse))
				}
			}
		},
		//Given: machine with a metal3machine in Spec.Infra. No metal3machine object
		//Expected: No error. NotFound error is consumed by reconciler and returns nil.
		Entry("Should not return an error when metal3machine is not found",
			TestCaseReconcile{
				Objects: []client.Object{
					newMachine(clusterName, machineName, metal3machineName, ""),
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
				ErrorExpected:       false,
				ErrorReasonExpected: true,
				ErrorReason:         "",
				RequeueExpected:     false,
			},
		),
		//Given: Machine, Metal3Machine, Cluster. No Metal3Cluster. Cluster Infra not ready
		//Expected: No Error. it should wait and not return error. Condition AssociateBMHCondition should be false with WaitingForClusterInfrastructureReason.
		Entry("Should not return an error when owner Cluster infrastructure is not ready",
			TestCaseReconcile{
				Objects: []client.Object{
					newCluster(clusterName, nil, &clusterv1.ClusterStatus{
						Conditions: []metav1.Condition{
							{
								Type:   clusterv1.InfrastructureReadyCondition,
								Status: metav1.ConditionFalse,
							},
						},
					}),
					metal3machineWithOwnerRefs(),
					machineWithInfra(),
				},
				ErrorExpected:     false,
				RequeueExpected:   false,
				ClusterInfraReady: false,
				ConditionsExpected: []metav1.Condition{
					{
						Type:   infrav1.AssociateBareMetalHostV1Beta2Condition,
						Status: metav1.ConditionFalse,
						Reason: infrav1.WaitingForClusterInfrastructureReadyV1Beta2Reason,
					},
					{
						Type:   infrav1.Metal3MachineReadyV1Beta2Condition,
						Status: metav1.ConditionFalse,
					},
				},
			},
		),
		//Given: Machine, Metal3Machine, Cluster, Metal3Cluster. Bootstrap is not Ready.
		//Expected: No Error. Condition AssociateBMHCondition should be false with WaitingForBootstrapReadyReason.
		Entry("Should not return an error when machine bootstrap data is not ready",
			TestCaseReconcile{
				Objects: []client.Object{
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					metal3machineWithOwnerRefs(),
					machineWithInfra(),
				},
				ErrorExpected:       false,
				RequeueExpected:     false,
				ClusterInfraReady:   true,
				CheckBootStrapReady: false,
				ConditionsExpected: []metav1.Condition{
					{
						Type:   infrav1.AssociateBareMetalHostV1Beta2Condition,
						Status: metav1.ConditionFalse,
						Reason: infrav1.WaitingForBootstrapDataV1Beta2Reason,
					},
					{
						Type:   infrav1.Metal3MachineReadyV1Beta2Condition,
						Status: metav1.ConditionFalse,
					},
				},
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
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
				},
				ErrorExpected:           false,
				RequeueExpected:         false,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
				CheckBMFinalizer:        true,
			},
		),
		//Given: Machine, Metal3Machine (No Spec/Status), Cluster, Metal3Cluster.
		// Cluster.Spec.Paused=true
		//Expected: Requeue Expected, set AssociateBMHCondition to Metal3MachinePaused
		Entry("Should requeue when owner Cluster is paused",
			TestCaseReconcile{
				Objects: []client.Object{
					metal3machineWithOwnerRefs(),
					machineWithInfra(),
					newCluster(clusterName, clusterPauseSpec(), nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
				},
				ErrorExpected:           false,
				RequeueExpected:         true,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
				ConditionsExpected: []metav1.Condition{
					{
						Type:   clusterv1.PausedCondition,
						Status: metav1.ConditionTrue,
						Reason: clusterv1.PausedReason,
					},
					{
						Type:   infrav1.Metal3MachineReadyV1Beta2Condition,
						Status: metav1.ConditionFalse,
					},
				},
			},
		),
		//Given: Machine, Metal3Machine (No Spec/Status), Cluster, Metal3Cluster.
		// Metal3Machine has cluster.x-k8s.io/paused annotation
		//Expected: Requeue Expected
		Entry("Should requeue when Metal3Machine has paused annotation",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, nil, nil, nil, true),
					machineWithInfra(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
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
						&infrav1.Metal3MachineSpec{
							ProviderID: &providerID,
						},
						&infrav1.Metal3MachineStatus{
							Ready: true,
						},
						false,
					),
					machineWithInfra(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost(baremetalhostName, nil, nil, nil, false),
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
						metal3machineName, m3mMetaWithOwnerRef(), &infrav1.Metal3MachineSpec{
							Image: infrav1.Image{
								Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.sha256sum",
								URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
								// Checking the pointers,
								// CheckBMHostProvisioned is true
								ChecksumType: ptr.To("sha512"),
								DiskFormat:   ptr.To("raw"),
							},
						}, nil, false,
					),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost(baremetalhostName, nil, &bmov1alpha1.BareMetalHostStatus{
						Provisioning: bmov1alpha1.ProvisionStatus{
							State: bmov1alpha1.StateAvailable,
						},
					}, nil, false),
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
						metal3machineName, m3mMetaWithOwnerRef(), &infrav1.Metal3MachineSpec{
							Image: infrav1.Image{
								Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.sha256sum",
								URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
								// No ChecksumType and DiskFormat given to test without them
								// CheckBMHostProvisioned is true
							},
						}, nil, false,
					),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost(baremetalhostName, nil, &bmov1alpha1.BareMetalHostStatus{
						Provisioning: bmov1alpha1.ProvisionStatus{
							State: bmov1alpha1.StateReady,
						},
					}, nil, false),
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
						&infrav1.Metal3MachineSpec{
							Image: infrav1.Image{
								Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.sha256sum",
								URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
							},
						}, nil, false,
					),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost(baremetalhostName, nil, nil, nil, false),
				},
				TargetObjects: []runtime.Object{
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Labels: map[string]string{
								"metal3.io/uuid": "63856098-4b80-11ec-81d3-0242ac130003",
							},
						},
						Spec: corev1.NodeSpec{},
					},
				},
				ErrorExpected:        false,
				RequeueExpected:      false,
				ClusterInfraReady:    true,
				CheckBMFinalizer:     true,
				CheckBMProviderID:    true,
				CheckBMProviderIDNew: true,
				CheckBootStrapReady:  true,
				ConditionsExpected: []metav1.Condition{
					{
						Type:   infrav1.AssociateBareMetalHostV1Beta2Condition,
						Status: metav1.ConditionTrue,
					},
				},
			},
		),
		//Given: Machine(with Bootstrap data), M3Machine (Annotation Given, provider ID set), BMH (provisioned)
		//Expected: No Error, BMH.Spec.ProviderID is set properly (unchanged)
		Entry("Should set ProviderID when bootstrap data is available, ProviderID is given, BMH is provisioned",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(
						metal3machineName, m3mMetaWithAnnotation(),
						&infrav1.Metal3MachineSpec{
							ProviderID: ptr.To(providerID),
							Image: infrav1.Image{
								Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.sha256sum",
								URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
							},
						}, nil, false,
					),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost(baremetalhostName, nil, nil, nil, false),
				},
				TargetObjects: []runtime.Object{
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
						},
						Spec: corev1.NodeSpec{},
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
				ErrorExpected:        false,
				RequeueExpected:      false,
				ClusterInfraReady:    true,
				CheckBMFinalizer:     true,
				CheckBMProviderID:    true,
				CheckBMProviderIDNew: true,
				CheckBootStrapReady:  true,
				ConditionsExpected: []metav1.Condition{
					{
						Type:   infrav1.AssociateBareMetalHostV1Beta2Condition,
						Status: metav1.ConditionTrue,
					},
					{
						Type:   infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition,
						Status: metav1.ConditionTrue,
					},
					{
						Type:   infrav1.Metal3MachineReadyV1Beta2Condition,
						Status: metav1.ConditionTrue,
					},
				},
			},
		),
		// Given: Machine(with Bootstrap data), M3Machine (Annotation Given, no provider ID), BMH (provisioning)
		// Expected: No Error, Requeue expected
		// BMH.Spec.ProviderID is not set based on the UID since BMH is in provisioning
		Entry("Should requeue when bootstrap data is available, ProviderID is not given, BMH is provisioning",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, m3mMetaWithAnnotation(), &infrav1.Metal3MachineSpec{
						Image: infrav1.Image{
							Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.sha256sum",
							URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
						},
					}, nil, false),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newBareMetalHost(baremetalhostName, nil, &bmov1alpha1.BareMetalHostStatus{
						Provisioning: bmov1alpha1.ProvisionStatus{
							State: bmov1alpha1.StateProvisioning,
						},
					}, nil, false),
				},
				ErrorExpected:           false,
				RequeueExpected:         false,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
				CheckBMFinalizer:        true,
				CheckBootStrapReady:     true,
			},
		),

		// ------------- Cloud Provider
		// ------------- Bootstrap
		// ------------- Hostname look up

		// should set ProviderID on Node when
		//   - cloud provider is disabled
		//   - kubeadm/node labels disabled
		Entry("Should set ProviderID on Node when matching by hostname",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, m3mMetaWithAnnotation(), &infrav1.Metal3MachineSpec{
						ProviderID: ptr.To("abc"),
						Image: infrav1.Image{
							Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.sha256sum",
							URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
						},
					}, &infrav1.Metal3MachineStatus{
						Ready: true,
						// NOTE: Addresses will be populated from BMH
					}, false),
					machineWithDataSecret(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, &infrav1.Metal3ClusterSpec{
						CloudProviderEnabled: ptr.To(false),
					}, nil, nil, false),
					newBareMetalHost(baremetalhostName, nil, &bmov1alpha1.BareMetalHostStatus{
						HardwareDetails: &bmov1alpha1.HardwareDetails{
							Hostname: "example.com",
						},
						Provisioning: bmov1alpha1.ProvisionStatus{
							State: bmov1alpha1.StateProvisioned,
						},
					}, nil, false),
				},
				TargetObjects: []runtime.Object{
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Labels: map[string]string{
								"kubernetes.io/hostname": "example.com",
							},
						},
						Spec: corev1.NodeSpec{},
					},
				},
				ErrorExpected:       false,
				RequeueExpected:     false,
				ClusterInfraReady:   true,
				CheckBootStrapReady: true,
			},
		),

		// should not return error when setting ProviderID on Node and
		//   - cloud provider is disabled
		//   - kubeadm disabled
		//   - Node is not ready yet
		Entry("Should not return error when setting ProviderID and Node is not ready",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, m3mMetaWithAnnotation(), &infrav1.Metal3MachineSpec{
						ProviderID: ptr.To("abc"),
						Image: infrav1.Image{
							Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.sha256sum",
							URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
						},
					}, &infrav1.Metal3MachineStatus{
						Ready: true,
						// NOTE: Addresses will be populated from BMH
					}, false),
					machineWithDataSecret(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, &infrav1.Metal3ClusterSpec{
						CloudProviderEnabled: ptr.To(false),
					}, nil, nil, false),
					newBareMetalHost(baremetalhostName, nil, &bmov1alpha1.BareMetalHostStatus{
						HardwareDetails: &bmov1alpha1.HardwareDetails{
							Hostname: "example.com",
						},
						Provisioning: bmov1alpha1.ProvisionStatus{
							State: bmov1alpha1.StateProvisioned,
						},
					}, nil, false),
				},
				ErrorExpected:           false,
				RequeueExpected:         true,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
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
					newBareMetalHost(baremetalhostName, nil, nil, nil, false),
				},
				ErrorExpected:           false,
				RequeueExpected:         true,
				ExpectedRequeueDuration: requeueAfter,
				ClusterInfraReady:       true,
				CheckBMFinalizer:        true,
				CheckBMProviderID:       false,
				CheckBootStrapReady:     true,
				ConditionsExpected: []metav1.Condition{
					{
						Type:   infrav1.AssociateBareMetalHostV1Beta2Condition,
						Status: metav1.ConditionTrue,
					},
				},
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
					newBareMetalHost(baremetalhostName, nil, nil, nil, false),
				},
				TargetObjects: []runtime.Object{
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Labels: map[string]string{
								baremetal.ProviderLabelPrefix: string(bmhuid),
							},
						},
						Spec: corev1.NodeSpec{},
					},
				},
				ErrorExpected:        false,
				RequeueExpected:      false,
				ClusterInfraReady:    true,
				CheckBMFinalizer:     true,
				CheckBMProviderID:    true,
				CheckBMProviderIDNew: true,
				CheckBootStrapReady:  true,
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
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
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
					newBareMetalHost(baremetalhostName, &bmov1alpha1.BareMetalHostSpec{
						ConsumerRef: &corev1.ObjectReference{
							Name:       metal3machineName,
							Namespace:  namespaceName,
							Kind:       metal3MachineKind,
							APIVersion: infrav1.GroupVersion.String(),
						},
						Online:                true,
						AutomatedCleaningMode: "metadata",
					}, &bmov1alpha1.BareMetalHostStatus{}, nil, false),
				},
				ErrorExpected:           false,
				RequeueExpected:         true,
				ExpectedRequeueDuration: time.Second * 0,
				ClusterInfraReady:       true,
				CheckBMHostCleaned:      true,
				ExpectedOnlineStatus:    false,
			},
		),
		// Our test Given: Machine, M3Machine(with incorrect Annotation), Cluster, Metal3Cluster.
		// Expected: set AssociateBMHCondition to PauseAnnotationRemoveFailed, do not return error.
		Entry("Should not return an error when Pause Annotation is not removed, set AssociateBMHCondition to PauseAnnotationRemoveFailed",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, m3mMetaWithIncorrectAnnotation(), nil, nil, false),
					machineWithBootstrap(),
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
				},
				ErrorExpected:           false,
				RequeueExpected:         false,
				CheckBootStrapReady:     true,
				ClusterInfraReady:       true,
				ExpectedRequeueDuration: time.Second * 0,
				ConditionsExpected: []metav1.Condition{
					{
						Type:   clusterv1.PausedCondition,
						Status: metav1.ConditionFalse,
						Reason: infrav1.BareMetalHostPauseAnnotationRemoveFailedV1Beta2Reason,
					},
					{
						Type:   infrav1.Metal3MachineReadyV1Beta2Condition,
						Status: metav1.ConditionFalse,
					},
				},
			},
		),
		// Our test Given: Machine, M3Machine(with incorrect Annotation), Cluster(paused), Metal3Cluster.
		// Expected: set AssociateBMHCondition to PauseAnnotationSetFailed, do not return error.
		Entry("Should not return an error when SetPauseAnnotation failed, should set Condition to PauseAnnotationSetFailed",
			TestCaseReconcile{
				Objects: []client.Object{
					newMetal3Machine(metal3machineName, m3mMetaWithIncorrectAnnotation(), nil, nil, false),
					machineWithBootstrap(),
					newCluster(clusterName, clusterPauseSpec(), nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
				},
				ErrorExpected:           false,
				RequeueExpected:         false,
				CheckBootStrapReady:     true,
				ClusterInfraReady:       true,
				ExpectedRequeueDuration: time.Second * 0,
				ConditionsExpected: []metav1.Condition{
					{
						Type:   clusterv1.PausedCondition,
						Status: metav1.ConditionFalse,
						Reason: infrav1.BareMetalHostPauseAnnotationSetFailedV1Beta2Reason,
					},
					{
						Type:   infrav1.Metal3MachineReadyV1Beta2Condition,
						Status: metav1.ConditionFalse,
					},
				},
			},
		),
	)
})
