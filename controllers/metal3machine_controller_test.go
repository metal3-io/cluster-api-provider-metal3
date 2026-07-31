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
		m.EXPECT().SetCondition(
			infrav1.AssociateMetal3MachineMetaDataCondition,
			metav1.ConditionTrue,
			infrav1.AssociateMetal3MachineMetaDataSuccessReason, "")
		m.EXPECT().Update(context.TODO()).Return(nil)
		m.EXPECT().IsBootstrapReady().MaxTimes(0)
		m.EXPECT().AssociateM3Metadata(context.TODO()).MaxTimes(0)
		m.EXPECT().HasAnnotation().MaxTimes(0)
		return m
	}

	// Bootstrap data not ready, we'll requeue, not call anything else
	m.EXPECT().IsBootstrapReady().Return(!tc.BootstrapNotReady)
	if tc.BootstrapNotReady {
		m.EXPECT().SetV1Beta1ConditionToFalse(infrav1.AssociateBMHV1Beta1Condition,
			infrav1.WaitingForBootstrapReadyV1Beta1Reason, clusterv1.ConditionSeverityInfo, "")
		m.EXPECT().SetCondition(infrav1.AssociateBareMetalHostCondition,
			metav1.ConditionFalse, infrav1.WaitingForBootstrapDataReason,
			"Waiting for bootstrap data to be ready before proceeding")
		m.EXPECT().AssociateM3Metadata(context.TODO()).MaxTimes(0)
		m.EXPECT().HasAnnotation().MaxTimes(0)
		m.EXPECT().Update(context.TODO()).MaxTimes(0)
		return m
	}

	// Bootstrap data is ready and node is not annotated, i.e. not associated
	m.EXPECT().HasAnnotation().Return(tc.Annotated)
	if !tc.Annotated {
		// if associate fails, we do not go further
		if tc.AssociateFails {
			m.EXPECT().Associate(context.TODO()).Return("", errors.New("failed"))
			m.EXPECT().SetV1Beta1ConditionToFalse(infrav1.AssociateBMHV1Beta1Condition,
				infrav1.AssociateBMHFailedV1Beta1Reason, clusterv1.ConditionSeverityError, "%s", gomock.Any())
			m.EXPECT().SetCondition(infrav1.AssociateBareMetalHostCondition,
				metav1.ConditionFalse, infrav1.AssociateBareMetalHostFailedReason, gomock.Any())
			m.EXPECT().AssociateM3Metadata(context.TODO()).MaxTimes(0)
			m.EXPECT().Update(context.TODO()).MaxTimes(0)
			return m
		}
		// Use the specified associate reason or default to success
		associateReason := tc.AssociateReason
		if associateReason == "" {
			associateReason = infrav1.AssociateBareMetalHostSuccessReason
		}
		m.EXPECT().Associate(context.TODO()).Return(associateReason, nil)
		// After association, HasAnnotation is checked again
		m.EXPECT().HasAnnotation().Return(tc.AnnotatedAfterAssociate)
		if tc.AnnotatedAfterAssociate {
			m.EXPECT().SetV1Beta1ConditionToTrue(infrav1.AssociateBMHV1Beta1Condition)
			m.EXPECT().SetCondition(infrav1.AssociateBareMetalHostCondition,
				metav1.ConditionTrue, associateReason, "")
		}
		return m
	}

	if tc.Annotated {
		m.EXPECT().Update(context.TODO()).Return(nil).MaxTimes(10)
		m.EXPECT().GetMetal3Machine().Return(&infrav1.Metal3Machine{}).Times(2)
		m.EXPECT().SetV1Beta1ConditionToTrue(infrav1.AssociateBMHV1Beta1Condition)
		m.EXPECT().SetCondition(infrav1.AssociateBareMetalHostCondition,
			metav1.ConditionTrue, infrav1.AssociateBareMetalHostSuccessReason,
			"")
		if tc.Metal3DataClaimCreated {
			m.EXPECT().AssociateM3Metadata(context.TODO())
			m.EXPECT().SetCondition(infrav1.AssociateMetal3MachineMetaDataCondition,
				metav1.ConditionTrue, infrav1.AssociateMetal3MachineMetaDataSuccessReason,
				"")
		} else {
			m.EXPECT().AssociateM3Metadata(context.TODO()).Return(errors.New("failed"))
			m.EXPECT().SetV1Beta1ConditionToFalse(infrav1.KubernetesNodeReadyV1Beta1Condition,
				infrav1.AssociateM3MetaDataFailedV1Beta1Reason, clusterv1.ConditionSeverityWarning, "%s", gomock.Any())
			m.EXPECT().SetCondition(infrav1.AssociateMetal3MachineMetaDataCondition,
				metav1.ConditionFalse, infrav1.AssociateMetal3MachineMetaDataFailedReason, gomock.Any())
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
			m.EXPECT().SetMetal3DataReadyConditionTrue(infrav1.SecretsSetExternallyReason)
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
	m.EXPECT().SetV1Beta1ConditionToFalse(infrav1.KubernetesNodeReadyV1Beta1Condition, infrav1.DeletingV1Beta1Reason, clusterv1.ConditionSeverityInfo, "")
	m.EXPECT().SetCondition(infrav1.AssociateMetal3MachineMetaDataCondition, metav1.ConditionFalse, infrav1.Metal3MachineDeletingReason, "")

	if tc.DissociateM3MetadataFails {
		m.EXPECT().DissociateM3Metadata(context.TODO()).Return(errors.New("failed"))
		m.EXPECT().SetV1Beta1ConditionToFalse(infrav1.KubernetesNodeReadyV1Beta1Condition, infrav1.DisassociateM3MetaDataFailedV1Beta1Reason, clusterv1.ConditionSeverityWarning, "%s", gomock.Any())
		m.EXPECT().SetCondition(infrav1.AssociateMetal3MachineMetaDataCondition, metav1.ConditionFalse, infrav1.DisassociateM3MetaDataFailedReason, gomock.Any())
		m.EXPECT().Delete(context.TODO()).MaxTimes(0)
		m.EXPECT().UnsetFinalizer().MaxTimes(0)
		return m
	}
	if tc.DeleteFails {
		m.EXPECT().DissociateM3Metadata(context.TODO())
		m.EXPECT().Delete(context.TODO()).Return(errors.New("failed"))
		m.EXPECT().SetV1Beta1ConditionToFalse(infrav1.KubernetesNodeReadyV1Beta1Condition, infrav1.DeletionFailedV1Beta1Reason, clusterv1.ConditionSeverityWarning, "%s", gomock.Any())
		m.EXPECT().SetCondition(infrav1.AssociateMetal3MachineMetaDataCondition, metav1.ConditionFalse, infrav1.Metal3MachineDeletingFailedReason, gomock.Any())
		m.EXPECT().UnsetFinalizer().MaxTimes(0)
		return m
	} else if tc.DeleteRequeue {
		m.EXPECT().DissociateM3Metadata(context.TODO())
		m.EXPECT().Delete(context.TODO()).Return(baremetal.WithTransientError(errors.New("failed"), requeueAfter))
		m.EXPECT().SetV1Beta1ConditionToFalse(infrav1.KubernetesNodeReadyV1Beta1Condition, infrav1.DeletionFailedV1Beta1Reason, clusterv1.ConditionSeverityWarning, "%s", gomock.Any())
		m.EXPECT().SetCondition(infrav1.AssociateMetal3MachineMetaDataCondition, metav1.ConditionFalse, infrav1.Metal3MachineDeletingFailedReason, gomock.Any())
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
				capiMachine := newMachine(clusterName, machineName, metal3machineName, "")
				capm3Machine := newMetal3Machine(metal3machineName, nil, nil, nil, false)
				res, err := bmReconcile.reconcileNormal(context.TODO(), m, capiMachine, capm3Machine, logr.Discard())

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
				AssociateReason:         infrav1.AssociateBareMetalHostSuccessReason,
				AnnotatedAfterAssociate: true,
			}),
			Entry("Not Annotated, Associate with regular success, annotation not set", reconcileNormalTestCase{
				ExpectError:             false,
				ExpectRequeue:           false,
				Annotated:               false,
				AssociateReason:         infrav1.AssociateBareMetalHostSuccessReason,
				AnnotatedAfterAssociate: false,
			}),
			Entry("Not Annotated, Associate via node reuse, annotation set", reconcileNormalTestCase{
				ExpectError:             false,
				ExpectRequeue:           false,
				Annotated:               false,
				AssociateReason:         infrav1.AssociateBareMetalHostViaNodeReuseSuccessReason,
				AnnotatedAfterAssociate: true,
			}),
			Entry("Not Annotated, Associate via node reuse, annotation not set", reconcileNormalTestCase{
				ExpectError:             false,
				ExpectRequeue:           false,
				Annotated:               false,
				AssociateReason:         infrav1.AssociateBareMetalHostViaNodeReuseSuccessReason,
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

	// newPlacementM3M builds a Metal3Machine for the placement failure domain
	// tests, optionally annotated with an associated host and/or the
	// cloned-from template annotations.
	newPlacementM3M := func(hostKey string, clonedFrom bool, spec *infrav1.Metal3MachineSpec) *infrav1.Metal3Machine {
		annotations := map[string]string{}
		if hostKey != "" {
			annotations[baremetal.HostAnnotation] = hostKey
		}
		if clonedFrom {
			annotations[clusterv1.TemplateClonedFromNameAnnotation] = "my-m3mt"
			annotations[clusterv1.TemplateClonedFromGroupKindAnnotation] = infrav1.ClonedFromGroupKind
		}
		meta := &metav1.ObjectMeta{
			Name:        metal3machineName,
			Namespace:   namespaceName,
			Annotations: annotations,
		}
		return newMetal3Machine(metal3machineName, meta, spec, nil, false)
	}

	newFDHost := func(name, fd string) *bmov1alpha1.BareMetalHost {
		var labels map[string]string
		if fd != "" {
			labels = map[string]string{baremetal.FailureDomainLabelName: fd}
		}
		return newBareMetalHost(name, nil, nil, labels, false)
	}

	type testCaseActualFailureDomain struct {
		M3Machine  *infrav1.Metal3Machine
		Host       *bmov1alpha1.BareMetalHost
		ExpectedFD string
	}

	DescribeTable("actualFailureDomain tests",
		func(tc testCaseActualFailureDomain) {
			objects := []client.Object{}
			if tc.Host != nil {
				objects = append(objects, tc.Host)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			r := &Metal3MachineReconciler{Client: fakeClient, Log: logr.Discard()}

			fd, err := r.actualFailureDomain(context.Background(), tc.M3Machine)
			Expect(err).NotTo(HaveOccurred())
			Expect(fd).To(Equal(tc.ExpectedFD))
		},
		Entry("No BMH annotation returns empty", testCaseActualFailureDomain{
			M3Machine:  newPlacementM3M("", false, nil),
			ExpectedFD: "",
		}),
		Entry("Annotated BMH with FD label returns label value", testCaseActualFailureDomain{
			M3Machine:  newPlacementM3M(namespaceName+"/bmh-0", false, nil),
			Host:       newFDHost("bmh-0", "rack2"),
			ExpectedFD: "rack2",
		}),
		Entry("Annotated BMH without FD label returns empty", testCaseActualFailureDomain{
			M3Machine:  newPlacementM3M(namespaceName+"/bmh-0", false, nil),
			Host:       newFDHost("bmh-0", ""),
			ExpectedFD: "",
		}),
		Entry("Annotated but missing BMH returns empty", testCaseActualFailureDomain{
			M3Machine:  newPlacementM3M(namespaceName+"/bmh-gone", false, nil),
			ExpectedFD: "",
		}),
		Entry("Namespace prefix in annotation is ignored, own namespace is used", testCaseActualFailureDomain{
			M3Machine:  newPlacementM3M("other-namespace/bmh-0", false, nil),
			Host:       newFDHost("bmh-0", "rack2"),
			ExpectedFD: "rack2",
		}),
	)

	newM3TemplateWithFDMapping := func() *infrav1.Metal3MachineTemplate {
		m3mt := newMetal3MachineTemplate("my-m3mt", namespaceName, map[string]string{})
		m3mt.Spec.FailureDomainDataTemplates = []infrav1.FailureDomainDataTemplate{
			{
				FailureDomain: "rack1",
				DataTemplate:  &infrav1.Metal3ObjectRef{Name: "m3dt-rack1"},
			},
			{
				FailureDomain: "rack2",
				DataTemplate:  &infrav1.Metal3ObjectRef{Name: "m3dt-rack2", Namespace: namespaceName},
			},
		}
		return m3mt
	}

	defaultDataTemplate := &infrav1.Metal3ObjectRef{Name: "m3dt-default", Namespace: namespaceName}

	type testCaseOverridePlacement struct {
		M3Machine        *infrav1.Metal3Machine
		M3Template       *infrav1.Metal3MachineTemplate
		ActualFD         string
		ExpectedTemplate *infrav1.Metal3ObjectRef
	}

	DescribeTable("overrideDataTemplateForPlacement tests",
		func(tc testCaseOverridePlacement) {
			objects := []client.Object{}
			if tc.M3Template != nil {
				objects = append(objects, tc.M3Template)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			r := &Metal3MachineReconciler{Client: fakeClient, Log: logr.Discard()}

			err := r.overrideDataTemplateForPlacement(context.Background(), tc.M3Machine, tc.ActualFD, logr.Discard())
			Expect(err).NotTo(HaveOccurred())

			if tc.ExpectedTemplate == nil {
				Expect(tc.M3Machine.Spec.DataTemplate).To(BeNil())
			} else {
				Expect(tc.M3Machine.Spec.DataTemplate).NotTo(BeNil())
				Expect(*tc.M3Machine.Spec.DataTemplate).To(Equal(*tc.ExpectedTemplate))
			}
		},
		Entry("Actual FD in mapping overrides dataTemplate, namespace defaulted", testCaseOverridePlacement{
			M3Machine:        newPlacementM3M("", true, &infrav1.Metal3MachineSpec{DataTemplate: defaultDataTemplate.DeepCopy()}),
			M3Template:       newM3TemplateWithFDMapping(),
			ActualFD:         "rack1",
			ExpectedTemplate: &infrav1.Metal3ObjectRef{Name: "m3dt-rack1", Namespace: namespaceName},
		}),
		Entry("Actual FD in mapping overrides nil dataTemplate", testCaseOverridePlacement{
			M3Machine:        newPlacementM3M("", true, &infrav1.Metal3MachineSpec{}),
			M3Template:       newM3TemplateWithFDMapping(),
			ActualFD:         "rack2",
			ExpectedTemplate: &infrav1.Metal3ObjectRef{Name: "m3dt-rack2", Namespace: namespaceName},
		}),
		Entry("Actual FD not in mapping keeps default", testCaseOverridePlacement{
			M3Machine:        newPlacementM3M("", true, &infrav1.Metal3MachineSpec{DataTemplate: defaultDataTemplate.DeepCopy()}),
			M3Template:       newM3TemplateWithFDMapping(),
			ActualFD:         "rack9",
			ExpectedTemplate: defaultDataTemplate,
		}),
		Entry("No cloned-from annotations keeps default", testCaseOverridePlacement{
			M3Machine:        newPlacementM3M("", false, &infrav1.Metal3MachineSpec{DataTemplate: defaultDataTemplate.DeepCopy()}),
			M3Template:       newM3TemplateWithFDMapping(),
			ActualFD:         "rack1",
			ExpectedTemplate: defaultDataTemplate,
		}),
		Entry("Missing template keeps default", testCaseOverridePlacement{
			M3Machine:        newPlacementM3M("", true, &infrav1.Metal3MachineSpec{DataTemplate: defaultDataTemplate.DeepCopy()}),
			M3Template:       nil,
			ActualFD:         "rack1",
			ExpectedTemplate: defaultDataTemplate,
		}),
		Entry("Already rendered machine keeps default", testCaseOverridePlacement{
			M3Machine: func() *infrav1.Metal3Machine {
				m3m := newPlacementM3M("", true, &infrav1.Metal3MachineSpec{DataTemplate: defaultDataTemplate.DeepCopy()})
				m3m.Status.RenderedData = &infrav1.Metal3ObjectRef{Name: "m3dt-default-0", Namespace: namespaceName}
				return m3m
			}(),
			M3Template:       newM3TemplateWithFDMapping(),
			ActualFD:         "rack1",
			ExpectedTemplate: defaultDataTemplate,
		}),
	)

	type testCaseReconcilePlacementFD struct {
		HostFD            string
		AssignedFD        string
		M3MSpecFD         string
		InitialStatusFD   string
		ClaimExists       bool
		ClaimTemplate     *infrav1.Metal3ObjectRef
		ExpectMachineFD   string
		ExpectM3MFD       string
		ExpectM3MStatusFD string
		ExpectDT          *infrav1.Metal3ObjectRef
	}

	DescribeTable("reconcilePlacementFailureDomain tests",
		func(tc testCaseReconcilePlacementFD) {
			capiMachine := newMachine(clusterName, machineName, metal3machineName, "")
			capiMachine.Spec.FailureDomain = tc.AssignedFD
			capm3Machine := newPlacementM3M(namespaceName+"/bmh-0", true, &infrav1.Metal3MachineSpec{
				FailureDomain: tc.M3MSpecFD,
				DataTemplate:  defaultDataTemplate.DeepCopy(),
			})
			capm3Machine.Status.FailureDomain = tc.InitialStatusFD

			objects := []client.Object{
				capiMachine,
				newFDHost("bmh-0", tc.HostFD),
				newM3TemplateWithFDMapping(),
			}
			if tc.ClaimExists {
				claimTemplate := tc.ClaimTemplate
				if claimTemplate == nil {
					claimTemplate = defaultDataTemplate.DeepCopy()
				}
				objects = append(objects, &infrav1.Metal3DataClaim{
					ObjectMeta: metav1.ObjectMeta{Name: metal3machineName, Namespace: namespaceName},
					Spec:       infrav1.Metal3DataClaimSpec{Template: claimTemplate},
				})
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			r := &Metal3MachineReconciler{Client: fakeClient, Log: logr.Discard()}

			err := r.reconcilePlacementFailureDomain(context.Background(), capiMachine, capm3Machine, logr.Discard())
			Expect(err).NotTo(HaveOccurred())

			// The core Machine is never written by CAPM3.
			updated := &clusterv1.Machine{}
			Expect(fakeClient.Get(context.Background(),
				types.NamespacedName{Namespace: namespaceName, Name: machineName}, updated)).To(Succeed())
			Expect(updated.Spec.FailureDomain).To(Equal(tc.ExpectMachineFD))
			Expect(capm3Machine.Spec.FailureDomain).To(Equal(tc.ExpectM3MFD))
			Expect(capm3Machine.Status.FailureDomain).To(Equal(tc.ExpectM3MStatusFD))
			Expect(capm3Machine.Spec.DataTemplate).NotTo(BeNil())
			Expect(*capm3Machine.Spec.DataTemplate).To(Equal(*tc.ExpectDT))
		},
		Entry("Fallback reported on Metal3Machine and dataTemplate follows placement", testCaseReconcilePlacementFD{
			HostFD:            "rack2",
			AssignedFD:        "rack1",
			M3MSpecFD:         "rack1",
			ExpectMachineFD:   "rack1",
			ExpectM3MFD:       "rack2",
			ExpectM3MStatusFD: "rack2",
			ExpectDT:          &infrav1.Metal3ObjectRef{Name: "m3dt-rack2", Namespace: namespaceName},
		}),
		Entry("Backfill: no assignment, placement reported", testCaseReconcilePlacementFD{
			HostFD:            "rack2",
			AssignedFD:        "",
			M3MSpecFD:         "",
			ExpectMachineFD:   "",
			ExpectM3MFD:       "rack2",
			ExpectM3MStatusFD: "rack2",
			ExpectDT:          &infrav1.Metal3ObjectRef{Name: "m3dt-rack2", Namespace: namespaceName},
		}),
		Entry("Unlabeled host: assignment mirrored, no placement reporting", testCaseReconcilePlacementFD{
			HostFD:            "",
			AssignedFD:        "rack1",
			M3MSpecFD:         "",
			ExpectMachineFD:   "rack1",
			ExpectM3MFD:       "rack1",
			ExpectM3MStatusFD: "",
			ExpectDT:          defaultDataTemplate,
		}),
		Entry("Unlabeled host: no-op when assignment already mirrored", testCaseReconcilePlacementFD{
			HostFD:            "",
			AssignedFD:        "rack1",
			M3MSpecFD:         "rack1",
			ExpectMachineFD:   "rack1",
			ExpectM3MFD:       "rack1",
			ExpectM3MStatusFD: "",
			ExpectDT:          defaultDataTemplate,
		}),
		Entry("Existing DataClaim freezes dataTemplate but FD still reported", testCaseReconcilePlacementFD{
			HostFD:            "rack2",
			AssignedFD:        "rack1",
			M3MSpecFD:         "rack1",
			ClaimExists:       true,
			ExpectMachineFD:   "rack1",
			ExpectM3MFD:       "rack2",
			ExpectM3MStatusFD: "rack2",
			ExpectDT:          defaultDataTemplate,
		}),
		Entry("Existing DataClaim is the source of truth for the dataTemplate", testCaseReconcilePlacementFD{
			HostFD:            "rack2",
			AssignedFD:        "rack1",
			M3MSpecFD:         "rack1",
			ClaimExists:       true,
			ClaimTemplate:     &infrav1.Metal3ObjectRef{Name: "m3dt-rack1", Namespace: namespaceName},
			ExpectMachineFD:   "rack1",
			ExpectM3MFD:       "rack2",
			ExpectM3MStatusFD: "rack2",
			ExpectDT:          &infrav1.Metal3ObjectRef{Name: "m3dt-rack1", Namespace: namespaceName},
		}),
		Entry("Label removed: status cleared, assignment mirror kept", testCaseReconcilePlacementFD{
			HostFD:            "",
			AssignedFD:        "rack1",
			M3MSpecFD:         "rack1",
			InitialStatusFD:   "rack2",
			ExpectMachineFD:   "rack1",
			ExpectM3MFD:       "rack1",
			ExpectM3MStatusFD: "",
			ExpectDT:          defaultDataTemplate,
		}),
	)
})
