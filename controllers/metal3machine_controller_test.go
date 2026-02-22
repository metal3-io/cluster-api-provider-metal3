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
	"errors"

	"github.com/go-logr/logr"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type reconcileNormalTestCase struct {
	ExpectError                     bool
	ExpectRequeue                   bool
	Provisioned                     bool
	BootstrapNotReady               bool
	Annotated                       bool
	AssociateFails                  bool
	AssociateReason                 string
	AnnotatedAfterAssociate         bool
	GetProviderIDFails              bool
	SetNodeProviderIDFails          bool
	CloudProviderEnabled            bool
	Metal3DataClaimCreated          bool
	SetProviderIDFromNodeLabelFails bool
}

func setReconcileNormalExpectations(ctrl *gomock.Controller,
	tc reconcileNormalTestCase,
) *baremetal_mocks.MockMachineManagerInterface {
	m := baremetal_mocks.NewMockMachineManagerInterface(ctrl)

	m.EXPECT().SetFinalizer()

	// provisioned, we should only call Update, nothing else
	m.EXPECT().IsProvisioned().Return(tc.Provisioned)
	if tc.Provisioned {
		m.EXPECT().MachineHasNodeRef().Return(tc.Provisioned)
		m.EXPECT().SetV1beta2Condition(
			infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition,
			metav1.ConditionTrue,
			infrav1.AssociateMetal3MachineMetaDataSuccessV1Beta2Reason, "")
		m.EXPECT().Update(context.TODO()).Return(nil)
		m.EXPECT().IsBootstrapReady().MaxTimes(0)
		m.EXPECT().AssociateM3Metadata(context.TODO()).MaxTimes(0)
		m.EXPECT().HasAnnotation().MaxTimes(0)
		m.EXPECT().GetProviderIDAndBMHID().MaxTimes(0)
		return m
	}

	// Bootstrap data not ready, we'll requeue, not call anything else
	m.EXPECT().IsBootstrapReady().Return(!tc.BootstrapNotReady)
	if tc.BootstrapNotReady {
		m.EXPECT().SetConditionMetal3MachineToFalse(infrav1.AssociateBMHCondition,
			infrav1.WaitingForBootstrapReadyReason, clusterv1.ConditionSeverityInfo, "")
		m.EXPECT().SetV1beta2Condition(infrav1.AssociateBareMetalHostV1Beta2Condition,
			metav1.ConditionFalse, infrav1.WaitingForBootstrapDataV1Beta2Reason,
			"Waiting for bootstrap data to be ready before proceeding")
		m.EXPECT().AssociateM3Metadata(context.TODO()).MaxTimes(0)
		m.EXPECT().HasAnnotation().MaxTimes(0)
		m.EXPECT().GetProviderIDAndBMHID().MaxTimes(0)
		m.EXPECT().Update(context.TODO()).MaxTimes(0)
		return m
	}

	// Bootstrap data is ready and node is not annotated, i.e. not associated
	m.EXPECT().HasAnnotation().Return(tc.Annotated)
	if !tc.Annotated {
		// if associate fails, we do not go further
		if tc.AssociateFails {
			m.EXPECT().Associate(context.TODO()).Return("", errors.New("failed"))
			m.EXPECT().SetConditionMetal3MachineToFalse(infrav1.AssociateBMHCondition,
				infrav1.AssociateBMHFailedReason, clusterv1.ConditionSeverityError, gomock.Any())
			m.EXPECT().SetV1beta2Condition(infrav1.AssociateBareMetalHostV1Beta2Condition,
				metav1.ConditionFalse, infrav1.AssociateBareMetalHostFailedV1Beta2Reason, gomock.Any())
			m.EXPECT().AssociateM3Metadata(context.TODO()).MaxTimes(0)
			m.EXPECT().Update(context.TODO()).MaxTimes(0)
			return m
		}
		// Use the specified associate reason or default to success
		associateReason := tc.AssociateReason
		if associateReason == "" {
			associateReason = infrav1.AssociateBareMetalHostSuccessV1Beta2Reason
		}
		m.EXPECT().Associate(context.TODO()).Return(associateReason, nil)
		// After association, HasAnnotation is checked again
		m.EXPECT().HasAnnotation().Return(tc.AnnotatedAfterAssociate)
		if tc.AnnotatedAfterAssociate {
			m.EXPECT().SetConditionMetal3MachineToTrue(infrav1.AssociateBMHCondition)
			m.EXPECT().SetV1beta2Condition(infrav1.AssociateBareMetalHostV1Beta2Condition,
				metav1.ConditionTrue, associateReason, "")
		}
		return m
	}

	if tc.Annotated {
		m.EXPECT().Update(context.TODO()).Return(nil).MaxTimes(10)
		m.EXPECT().GetMetal3Machine().Return(&infrav1.Metal3Machine{}).Times(2)
		m.EXPECT().SetConditionMetal3MachineToTrue(infrav1.AssociateBMHCondition)
		m.EXPECT().SetV1beta2Condition(infrav1.AssociateBareMetalHostV1Beta2Condition,
			metav1.ConditionTrue, infrav1.AssociateBareMetalHostSuccessV1Beta2Reason,
			"")
		if tc.Metal3DataClaimCreated {
			m.EXPECT().AssociateM3Metadata(context.TODO())
			m.EXPECT().SetV1beta2Condition(infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition,
				metav1.ConditionTrue, infrav1.AssociateMetal3MachineMetaDataSuccessV1Beta2Reason,
				"")
		} else {
			m.EXPECT().AssociateM3Metadata(context.TODO()).Return(errors.New("failed"))
			m.EXPECT().SetConditionMetal3MachineToFalse(infrav1.KubernetesNodeReadyCondition,
				infrav1.AssociateM3MetaDataFailedReason, clusterv1.ConditionSeverityWarning, gomock.Any())
			m.EXPECT().SetV1beta2Condition(infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition,
				metav1.ConditionFalse, infrav1.AssociateMetal3MachineMetaDataFailedV1Beta2Reason, gomock.Any())
			return m
		}
		if tc.CloudProviderEnabled {
			m.EXPECT().CloudProviderEnabled().Return(true)
		} else {
			m.EXPECT().CloudProviderEnabled().Return(false)
		}

		m.EXPECT().IsBaremetalHostProvisioned(context.TODO()).Return(true)
		m.EXPECT().NodeWithMatchingProviderIDExists(context.TODO(), nil).Return(false)
		if tc.SetProviderIDFromNodeLabelFails {
			m.EXPECT().SetProviderIDFromNodeLabel(context.TODO(), nil).Return(false, errors.New("failed"))
		} else {
			m.EXPECT().SetProviderIDFromNodeLabel(context.TODO(), nil).Return(true, nil)
			m.EXPECT().GetMetal3Machine().Return(&infrav1.Metal3Machine{})
			m.EXPECT().SetMetal3DataReadyConditionTrue(infrav1.SecretsSetExternallyV1Beta2Reason)
			m.EXPECT().SetReadyTrue()
		}
	}
	return m
}

type reconcileDeleteTestCase struct {
	ExpectError               bool
	ExpectRequeue             bool
	DeleteFails               bool
	DissociateM3MetadataFails bool
	DeleteRequeue             bool
}

func setReconcileDeleteExpectations(ctrl *gomock.Controller,
	tc reconcileDeleteTestCase,
) *baremetal_mocks.MockMachineManagerInterface {
	m := baremetal_mocks.NewMockMachineManagerInterface(ctrl)
	m.EXPECT().SetConditionMetal3MachineToFalse(infrav1.KubernetesNodeReadyCondition, infrav1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
	m.EXPECT().SetV1beta2Condition(infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition, metav1.ConditionFalse, infrav1.Metal3MachineDeletingV1Beta2Reason, "")

	if tc.DissociateM3MetadataFails {
		m.EXPECT().DissociateM3Metadata(context.TODO()).Return(errors.New("failed"))
		m.EXPECT().SetConditionMetal3MachineToFalse(infrav1.KubernetesNodeReadyCondition, infrav1.DisassociateM3MetaDataFailedReason, clusterv1.ConditionSeverityWarning, gomock.Any())
		m.EXPECT().SetV1beta2Condition(infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition, metav1.ConditionFalse, infrav1.DisassociateM3MetaDataFailedV1Beta2Reason, gomock.Any())
		m.EXPECT().Delete(context.TODO()).MaxTimes(0)
		m.EXPECT().UnsetFinalizer().MaxTimes(0)
		return m
	}
	if tc.DeleteFails {
		m.EXPECT().DissociateM3Metadata(context.TODO())
		m.EXPECT().Delete(context.TODO()).Return(errors.New("failed"))
		m.EXPECT().SetConditionMetal3MachineToFalse(infrav1.KubernetesNodeReadyCondition, infrav1.DeletionFailedReason, clusterv1.ConditionSeverityWarning, gomock.Any())
		m.EXPECT().SetV1beta2Condition(infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition, metav1.ConditionFalse, infrav1.Metal3MachineDeletingFailedV1Beta2Reason, gomock.Any())
		m.EXPECT().UnsetFinalizer().MaxTimes(0)
		return m
	} else if tc.DeleteRequeue {
		m.EXPECT().DissociateM3Metadata(context.TODO())
		m.EXPECT().Delete(context.TODO()).Return(baremetal.WithTransientError(errors.New("failed"), requeueAfter))
		m.EXPECT().SetConditionMetal3MachineToFalse(infrav1.KubernetesNodeReadyCondition, infrav1.DeletionFailedReason, clusterv1.ConditionSeverityWarning, gomock.Any())
		m.EXPECT().SetV1beta2Condition(infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition, metav1.ConditionFalse, infrav1.Metal3MachineDeletingFailedV1Beta2Reason, gomock.Any())
		m.EXPECT().UnsetFinalizer().MaxTimes(0)
		return m
	}

	m.EXPECT().DissociateM3Metadata(context.TODO())
	m.EXPECT().Delete(context.TODO()).Return(nil)
	m.EXPECT().UnsetFinalizer()
	return m
}

var _ = Describe("Metal3Machine manager", func() {

	Describe("Test MachineReconcileNormal", func() {

		var gomockCtrl *gomock.Controller
		var bmReconcile *Metal3MachineReconciler

		BeforeEach(func() {
			gomockCtrl = gomock.NewController(GinkgoT())

			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).Build()

			bmReconcile = &Metal3MachineReconciler{
				Client:           fakeClient,
				ManagerFactory:   baremetal.NewManagerFactory(fakeClient),
				Log:              logr.Discard(),
				CapiClientGetter: nil,
				WatchFilterValue: "",
			}
		})

		AfterEach(func() {
			gomockCtrl.Finish()
		})

		DescribeTable("ReconcileNormal tests",
			func(tc reconcileNormalTestCase) {
				m := setReconcileNormalExpectations(gomockCtrl, tc)
				res, err := bmReconcile.reconcileNormal(context.TODO(), m, logr.Discard())

				if tc.ExpectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
				if tc.ExpectRequeue {
					Expect(res.Requeue).To(BeTrue())
				} else {
					Expect(res.Requeue).To(BeFalse())
				}
			},
			Entry("Provisioned", reconcileNormalTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
				Provisioned:   true,
			}),
			Entry("Bootstrap not ready", reconcileNormalTestCase{
				ExpectError:       false,
				ExpectRequeue:     false,
				BootstrapNotReady: true,
			}),
			Entry("Not Annotated, Associate with regular success, annotation set", reconcileNormalTestCase{
				ExpectError:             false,
				ExpectRequeue:           false,
				Annotated:               false,
				AssociateReason:         infrav1.AssociateBareMetalHostSuccessV1Beta2Reason,
				AnnotatedAfterAssociate: true,
			}),
			Entry("Not Annotated, Associate with regular success, annotation not set", reconcileNormalTestCase{
				ExpectError:             false,
				ExpectRequeue:           false,
				Annotated:               false,
				AssociateReason:         infrav1.AssociateBareMetalHostSuccessV1Beta2Reason,
				AnnotatedAfterAssociate: false,
			}),
			Entry("Not Annotated, Associate via node reuse, annotation set", reconcileNormalTestCase{
				ExpectError:             false,
				ExpectRequeue:           false,
				Annotated:               false,
				AssociateReason:         infrav1.AssociateBareMetalHostViaNodeReuseSuccessV1Beta2Reason,
				AnnotatedAfterAssociate: true,
			}),
			Entry("Not Annotated, Associate via node reuse, annotation not set", reconcileNormalTestCase{
				ExpectError:             false,
				ExpectRequeue:           false,
				Annotated:               false,
				AssociateReason:         infrav1.AssociateBareMetalHostViaNodeReuseSuccessV1Beta2Reason,
				AnnotatedAfterAssociate: false,
			}),
			Entry("Not Annotated, Associate fails", reconcileNormalTestCase{
				ExpectError:    true,
				ExpectRequeue:  false,
				Annotated:      false,
				AssociateFails: true,
			}),
			Entry("Annotated", reconcileNormalTestCase{
				ExpectError:            false,
				ExpectRequeue:          false,
				Annotated:              true,
				Metal3DataClaimCreated: true,
			}),
			Entry("BMH ID set, GetProviderID fails", reconcileNormalTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
			}),
			Entry("BMH ID set", reconcileNormalTestCase{
				ExpectError:        false,
				ExpectRequeue:      false,
				GetProviderIDFails: true,
			}),
			Entry("Associate Metal3Data and create Metal3DataClaim", reconcileNormalTestCase{
				ExpectError:            false,
				ExpectRequeue:          false,
				Annotated:              true,
				Metal3DataClaimCreated: true,
			}),
			Entry("Associate Metal3Data and create Metal3DataClaim failed", reconcileNormalTestCase{
				ExpectError:            true,
				ExpectRequeue:          false,
				Annotated:              true,
				Metal3DataClaimCreated: false,
			}),
			Entry("SetProviderIDFromNodeLabel failed", reconcileNormalTestCase{
				ExpectError:                     true,
				ExpectRequeue:                   false,
				Annotated:                       true,
				Metal3DataClaimCreated:          true,
				SetProviderIDFromNodeLabelFails: true,
			}),
			Entry("SetProviderIDFromNodeLabel passed", reconcileNormalTestCase{
				ExpectError:                     false,
				ExpectRequeue:                   false,
				Annotated:                       true,
				Metal3DataClaimCreated:          true,
				SetProviderIDFromNodeLabelFails: false,
			}),
		)
	})

	Describe("Test MachineReconcileDelete", func() {

		var gomockCtrl *gomock.Controller
		var bmReconcile *Metal3MachineReconciler

		BeforeEach(func() {
			gomockCtrl = gomock.NewController(GinkgoT())

			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).Build()

			bmReconcile = &Metal3MachineReconciler{
				Client:           fakeClient,
				ManagerFactory:   baremetal.NewManagerFactory(fakeClient),
				Log:              logr.Discard(),
				CapiClientGetter: nil,
				WatchFilterValue: "",
			}
		})

		AfterEach(func() {
			gomockCtrl.Finish()
		})

		DescribeTable("Deletion tests",
			func(tc reconcileDeleteTestCase) {
				m := setReconcileDeleteExpectations(gomockCtrl, tc)
				res, err := bmReconcile.reconcileDelete(context.TODO(), m, logr.Discard())
				if tc.ExpectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
				if tc.ExpectRequeue {
					Expect(res.Requeue).To(BeTrue())
				} else {
					Expect(res.Requeue).To(BeFalse())
				}
			},
			Entry("Deletion success", reconcileDeleteTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
			}),
			Entry("Deletion failure", reconcileDeleteTestCase{
				ExpectError:   true,
				ExpectRequeue: false,
				DeleteFails:   true,
			}),
			Entry("Deletion requeue", reconcileDeleteTestCase{
				ExpectError:   false,
				ExpectRequeue: true,
				DeleteRequeue: true,
			}),
			Entry("DissociateM3Metadata failure", reconcileDeleteTestCase{
				ExpectError:               true,
				ExpectRequeue:             false,
				DeleteRequeue:             false,
				DissociateM3MetadataFails: true,
			}),
		)
	})

	type TestCaseMetal3ClusterToM3M struct {
		Cluster       *clusterv1.Cluster
		M3Cluster     *infrav1.Metal3Cluster
		Machine0      *clusterv1.Machine
		Machine1      *clusterv1.Machine
		Machine2      *clusterv1.Machine
		ExpectRequest bool
	}

	DescribeTable("Metal3Cluster To Metal3Machines tests",
		func(tc TestCaseMetal3ClusterToM3M) {
			objects := []client.Object{
				tc.Cluster,
				tc.M3Cluster,
				tc.Machine0,
				tc.Machine1,
				tc.Machine2,
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()

			r := Metal3MachineReconciler{
				Client:           fakeClient,
				Log:              logr.Discard(),
				WatchFilterValue: "",
			}

			obj := client.Object(tc.M3Cluster)
			reqs := r.Metal3ClusterToMetal3Machines(context.Background(), obj)

			m3machineNames := make([]string, len(reqs))
			for i := range reqs {
				m3machineNames[i] = reqs[i].Name
			}

			if tc.ExpectRequest {
				Expect(reqs).To(HaveLen(2), "Expected 2 Metal3 machines to reconcile but got %d", len(reqs))
				for _, expectedName := range []string{"my-metal3-machine-0", "my-metal3-machine-1"} {
					Expect(contains(m3machineNames, expectedName)).To(BeTrue(), "expected %q in slice %v", expectedName, m3machineNames)
				}
			} else {
				Expect(reqs).To(BeEmpty(), "Expected 0 request, found %d", len(reqs))
			}
		},
		// Given correct resources, metal3Machines reconcile
		Entry("Metal3Cluster To Metal3Machines, associated Metal3Machine Reconcile",
			TestCaseMetal3ClusterToM3M{
				Cluster:       newCluster(clusterName, nil, nil),
				M3Cluster:     newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, nil, false),
				Machine0:      newMachine(clusterName, "my-machine-0", "my-metal3-machine-0", ""),
				Machine1:      newMachine(clusterName, "my-machine-1", "my-metal3-machine-1", ""),
				Machine2:      newMachine(clusterName, "my-machine-2", "", ""),
				ExpectRequest: true,
			},
		),
		// No owner cluster, no reconciliation
		Entry("Metal3Cluster To Metal3Machines, No owner Cluster, No reconciliation",
			TestCaseMetal3ClusterToM3M{
				Cluster:       newCluster("my-other-cluster", nil, nil),
				M3Cluster:     newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, nil, false),
				Machine0:      newMachine(clusterName, "my-machine-0", "my-metal3-machine-0", ""),
				Machine1:      newMachine(clusterName, "my-machine-1", "my-metal3-machine-1", ""),
				Machine2:      newMachine(clusterName, "my-machine-2", "", ""),
				ExpectRequest: false,
			},
		),
		// No metal3 cluster, no reconciliation
		Entry("Metal3Cluster To Metal3Machines, No metal3Cluster, No reconciliation",
			TestCaseMetal3ClusterToM3M{
				Cluster:       newCluster("my-other-cluster", nil, nil),
				M3Cluster:     &infrav1.Metal3Cluster{},
				Machine0:      newMachine(clusterName, "my-machine-0", "my-metal3-machine-0", ""),
				Machine1:      newMachine(clusterName, "my-machine-1", "my-metal3-machine-1", ""),
				Machine2:      newMachine(clusterName, "my-machine-2", "", ""),
				ExpectRequest: false,
			},
		),
	)

	type TestCaseBMHToM3M struct {
		Host          *bmov1alpha1.BareMetalHost
		ExpectRequest bool
	}

	DescribeTable("BareMetalHost To Metal3Machines tests",
		func(tc TestCaseBMHToM3M) {
			r := Metal3MachineReconciler{}
			obj := client.Object(tc.Host)
			reqs := r.BareMetalHostToMetal3Machines(context.Background(), obj)

			if tc.ExpectRequest {
				Expect(reqs).To(HaveLen(1), "Expected 1 request, found %d", len(reqs))
				req := reqs[0]
				Expect(req.NamespacedName.Name).To(Equal(tc.Host.Spec.ConsumerRef.Name),
					"Expected name %s, found %s", tc.Host.Spec.ConsumerRef.Name, req.NamespacedName.Name)
				Expect(req.NamespacedName.Namespace).To(Equal(tc.Host.Spec.ConsumerRef.Namespace),
					"Expected namespace %s, found %s", tc.Host.Spec.ConsumerRef.Namespace, req.NamespacedName.Namespace)

			} else {
				Expect(reqs).To(BeEmpty(), "Expected 0 request, found %d", len(reqs))
			}
		},
		// Given machine, but no metal3machine resource
		Entry("BareMetalHost To Metal3Machines",
			TestCaseBMHToM3M{
				Host: &bmov1alpha1.BareMetalHost{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host1",
						Namespace: namespaceName,
					},
					Spec: bmov1alpha1.BareMetalHostSpec{
						ConsumerRef: &corev1.ObjectReference{
							Name:       "someothermachine",
							Namespace:  namespaceName,
							Kind:       metal3MachineKind,
							APIVersion: infrav1.GroupVersion.String(),
						},
					},
				},
				ExpectRequest: true,
			},
		),
		// Given machine, but no metal3machine resource
		Entry("BareMetalHost To Metal3Machines, no ConsumerRef",
			TestCaseBMHToM3M{
				Host: &bmov1alpha1.BareMetalHost{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host1",
						Namespace: namespaceName,
					},
					Spec: bmov1alpha1.BareMetalHostSpec{},
				},
				ExpectRequest: false,
			},
		),
	)

	type TestCaseM3DToM3M struct {
		OwnerRef      *metav1.OwnerReference
		ExpectRequest bool
	}

	DescribeTable("Metal3DataClaim To Metal3Machines tests",
		func(tc TestCaseM3DToM3M) {
			r := Metal3MachineReconciler{}
			ownerRefs := []metav1.OwnerReference{}
			if tc.OwnerRef != nil {
				ownerRefs = append(ownerRefs, *tc.OwnerRef)
			}
			dataClaim := &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: ownerRefs,
				},
				Spec: infrav1.Metal3DataClaimSpec{},
			}
			obj := client.Object(dataClaim)
			reqs := r.Metal3DataClaimToMetal3Machines(context.Background(), obj)

			if tc.ExpectRequest {
				Expect(reqs).To(HaveLen(1), "Expected 1 request, found %d", len(reqs))
				req := reqs[0]
				Expect(req.NamespacedName.Name).To(Equal(tc.OwnerRef.Name),
					"Expected name %s, found %s", tc.OwnerRef.Name, req.NamespacedName.Name)
				Expect(req.NamespacedName.Namespace).To(Equal(dataClaim.Namespace),
					"Expected namespace %s, found %s", dataClaim.Namespace, req.NamespacedName.Namespace)

			} else {
				Expect(reqs).To(BeEmpty(), "Expected 0 request, found %d", len(reqs))
			}
		},
		Entry("No Metal3Machine in Spec",
			TestCaseM3DToM3M{
				ExpectRequest: false,
			},
		),
		Entry("Metal3Machine in ownerRef",
			TestCaseM3DToM3M{
				OwnerRef: &metav1.OwnerReference{
					Name:       "abc",
					Kind:       metal3MachineKind,
					APIVersion: infrav1.GroupVersion.String(),
				},
				ExpectRequest: true,
			},
		),
		Entry("Wrong Kind",
			TestCaseM3DToM3M{
				OwnerRef: &metav1.OwnerReference{
					Name:       "abc",
					Kind:       "sdfousdf",
					APIVersion: infrav1.GroupVersion.String(),
				},
				ExpectRequest: false,
			},
		),
		Entry("Wrong Version, should work",
			TestCaseM3DToM3M{
				OwnerRef: &metav1.OwnerReference{
					Name:       "abc",
					Kind:       metal3MachineKind,
					APIVersion: infrav1.GroupVersion.Group + "/v1blah1",
				},
				ExpectRequest: true,
			},
		),
		Entry("Wrong Group, should not work",
			TestCaseM3DToM3M{
				OwnerRef: &metav1.OwnerReference{
					Name:       "abc",
					Kind:       metal3MachineKind,
					APIVersion: "foo.bar/" + infrav1.GroupVersion.Version,
				},
				ExpectRequest: false,
			},
		),
	)

	type TestCaseClusterToM3M struct {
		Cluster       *clusterv1.Cluster
		Machine       *clusterv1.Machine
		Machine1      *clusterv1.Machine
		Machine2      *clusterv1.Machine
		M3Machine     *infrav1.Metal3Machine
		ExpectRequest bool
	}

	DescribeTable("Cluster To Metal3Machines tests",
		func(tc TestCaseClusterToM3M) {
			objects := []client.Object{
				tc.Cluster,
				tc.Machine,
				tc.Machine1,
				tc.M3Machine,
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			r := Metal3MachineReconciler{
				Client: fakeClient,
			}
			obj := client.Object(tc.Cluster)
			reqs := r.ClusterToMetal3Machines(context.Background(), obj)

			if tc.ExpectRequest {
				Expect(reqs).To(HaveLen(1), "Expected 1 request, found %d", len(reqs))
				req := infrav1.Metal3Machine{}
				err := fakeClient.Get(context.TODO(), reqs[0].NamespacedName, &req)
				Expect(err).NotTo(HaveOccurred())

				Expect(req.Labels[clusterv1.ClusterNameLabel]).To(Equal(tc.Cluster.Name),
					"Expected label %s, found %s", tc.Cluster.Name, req.Labels[clusterv1.ClusterNameLabel])
			} else {
				Expect(reqs).To(BeEmpty(), "Expected 0 request, found %d", len(reqs))
			}
		},
		// Given Cluster, Machine with metal3machine resource, metal3Machine reconcile
		Entry("Cluster To Metal3Machines, associated Machine Reconciles",
			TestCaseClusterToM3M{
				Cluster:       newCluster(clusterName, nil, nil),
				M3Machine:     newMetal3Machine(metal3machineName, m3mObjectMetaWithOwnerRef(), nil, nil, false),
				Machine:       newMachine(clusterName, machineName, metal3machineName, ""),
				Machine1:      newMachine(clusterName, "my-machine-1", "", ""),
				ExpectRequest: true,
			},
		),

		// Given Cluster, Machine without metal3Machine resource, no reconciliation
		Entry("Cluster To Metal3Machines, no metal3Machine, no Reconciliation",
			TestCaseClusterToM3M{
				Cluster:       newCluster(clusterName, nil, nil),
				M3Machine:     newMetal3Machine("my-metal3-machine-0", nil, nil, nil, false),
				Machine:       newMachine(clusterName, "my-machine-0", "", ""),
				Machine1:      newMachine(clusterName, "my-machine-1", "", ""),
				ExpectRequest: false,
			},
		),
	)

	type testCaseMetal3DataToMetal3Machines struct {
		ownerRefs        []metav1.OwnerReference
		expectedRequests []ctrl.Request
	}

	DescribeTable("test Metal3DataToMetal3Machines",
		func(tc testCaseMetal3DataToMetal3Machines) {
			ipClaim := &infrav1.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespaceName,
					OwnerReferences: tc.ownerRefs,
				},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(ipClaim).Build()
			r := Metal3MachineReconciler{
				Client: fakeClient,
			}
			obj := client.Object(ipClaim)
			reqs := r.Metal3DataToMetal3Machines(context.Background(), obj)
			Expect(reqs).To(Equal(tc.expectedRequests))
		},
		Entry("No OwnerRefs", testCaseMetal3DataToMetal3Machines{
			expectedRequests: []ctrl.Request{},
		}),
		Entry("OwnerRefs", testCaseMetal3DataToMetal3Machines{
			ownerRefs: []metav1.OwnerReference{
				{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       metal3MachineKind,
					Name:       "abc",
				},
				{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "Metal3DataClaim",
					Name:       "bcd",
				},
				{
					APIVersion: "foo.bar/v1",
					Kind:       metal3MachineKind,
					Name:       "cde",
				},
			},
			expectedRequests: []ctrl.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "abc",
						Namespace: namespaceName,
					},
				},
			},
		}),
	)
})
