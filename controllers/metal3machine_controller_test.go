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

	"github.com/go-logr/logr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type reconcileNormalTestCase struct {
	ExpectError            bool
	ExpectRequeue          bool
	Provisioned            bool
	BootstrapNotReady      bool
	Annotated              bool
	AssociateFails         bool
	GetProviderIDFails     bool
	GetBMHIDFails          bool
	BMHIDSet               bool
	SetNodeProviderIDFails bool
}

func setReconcileNormalExpectations(ctrl *gomock.Controller,
	tc reconcileNormalTestCase,
) *baremetal_mocks.MockMachineManagerInterface {
	m := baremetal_mocks.NewMockMachineManagerInterface(ctrl)

	m.EXPECT().SetFinalizer()

	// provisioned, we should only call Update, nothing else
	m.EXPECT().IsProvisioned().Return(tc.Provisioned)
	if tc.Provisioned {
		m.EXPECT().Update(context.TODO()).Return(nil)
		m.EXPECT().IsBootstrapReady().MaxTimes(0)
		m.EXPECT().AssociateM3Metadata(context.TODO()).MaxTimes(0)
		m.EXPECT().HasAnnotation().MaxTimes(0)
		m.EXPECT().GetProviderIDAndBMHID().MaxTimes(0)
		m.EXPECT().GetBaremetalHostID(context.TODO()).MaxTimes(0)
		return m
	}

	// Bootstrap data not ready, we'll requeue, not call anything else
	m.EXPECT().IsBootstrapReady().Return(!tc.BootstrapNotReady)
	if tc.BootstrapNotReady {
		m.EXPECT().SetConditionMetal3MachineToFalse(capm3.AssociateBMHCondition,
			capm3.WaitingForBootstrapReadyReason, clusterv1.ConditionSeverityInfo, "")
		m.EXPECT().AssociateM3Metadata(context.TODO()).MaxTimes(0)
		m.EXPECT().HasAnnotation().MaxTimes(0)
		m.EXPECT().GetProviderIDAndBMHID().MaxTimes(0)
		m.EXPECT().GetBaremetalHostID(context.TODO()).MaxTimes(0)
		m.EXPECT().Update(context.TODO()).MaxTimes(0)
		return m
	}

	// Bootstrap data is ready and node is not annotated, i.e. not associated
	m.EXPECT().HasAnnotation().Return(tc.Annotated)
	if !tc.Annotated {
		// if associate fails, we do not go further
		if tc.AssociateFails {
			m.EXPECT().Associate(context.TODO()).Return(errors.New("Failed"))
			m.EXPECT().SetConditionMetal3MachineToFalse(capm3.AssociateBMHCondition, capm3.AssociateBMHFailedReason, clusterv1.ConditionSeverityError, gomock.Any())
			m.EXPECT().AssociateM3Metadata(context.TODO()).MaxTimes(0)
			m.EXPECT().Update(context.TODO()).MaxTimes(0)
			m.EXPECT().GetProviderIDAndBMHID().MaxTimes(0)
			m.EXPECT().GetBaremetalHostID(context.TODO()).MaxTimes(0)
			m.EXPECT().SetError(gomock.Any(), gomock.Any())
			return m
		}
		m.EXPECT().Associate(context.TODO()).Return(nil)
	}

	m.EXPECT().SetConditionMetal3MachineToTrue(capm3.AssociateBMHCondition)
	m.EXPECT().AssociateM3Metadata(context.TODO()).Return(nil)
	m.EXPECT().Update(context.TODO())

	// if node is now associated, if getting the ID fails, we do not go further
	if tc.GetBMHIDFails {
		m.EXPECT().GetProviderIDAndBMHID().Return("", nil)
		m.EXPECT().GetBaremetalHostID(context.TODO()).Return(nil,
			errors.New("Failed"),
		)
		m.EXPECT().SetProviderID(bmhuid).MaxTimes(0)
		m.EXPECT().SetError(gomock.Any(), gomock.Any())
		m.EXPECT().SetConditionMetal3MachineToFalse(capm3.KubernetesNodeReadyCondition, capm3.MissingBMHReason, clusterv1.ConditionSeverityError, gomock.Any())
		return m
	}

	// The ID is available (GetBaremetalHostID did not return nil)
	if tc.BMHIDSet {
		if tc.GetProviderIDFails {
			m.EXPECT().GetProviderIDAndBMHID().Return("", nil)
			m.EXPECT().GetBaremetalHostID(context.TODO()).Return(
				pointer.StringPtr(string(bmhuid)), nil,
			)
		} else {
			m.EXPECT().GetProviderIDAndBMHID().Return(
				providerID, pointer.StringPtr(string(bmhuid)),
			)
			m.EXPECT().GetBaremetalHostID(context.TODO()).MaxTimes(0)
		}

		// if we fail to set it on the node, we do not go further
		if tc.SetNodeProviderIDFails {
			m.EXPECT().
				SetNodeProviderID(context.TODO(), string(bmhuid), providerID, nil).
				Return(errors.New("Failed"))
			m.EXPECT().SetProviderID(string(bmhuid)).MaxTimes(0)
			m.EXPECT().SetError(gomock.Any(), gomock.Any())
			m.EXPECT().SetConditionMetal3MachineToFalse(capm3.KubernetesNodeReadyCondition,
				capm3.SettingProviderIDOnNodeFailedReason, clusterv1.ConditionSeverityError, gomock.Any())
			return m
		}

		// we successfully set it on the node
		m.EXPECT().
			SetNodeProviderID(context.TODO(), string(bmhuid), providerID, nil).
			Return(nil)
		m.EXPECT().SetProviderID(providerID)

		// We did not get an id (got nil), so we'll requeue and not go further
	} else {
		m.EXPECT().GetProviderIDAndBMHID().Return("", nil)
		m.EXPECT().GetBaremetalHostID(context.TODO()).Return(nil, nil)

		m.EXPECT().
			SetNodeProviderID(context.TODO(), bmhuid, providerID, nil).
			MaxTimes(0)
	}

	return m
}

type reconcileDeleteTestCase struct {
	ExpectError   bool
	ExpectRequeue bool
	DeleteFails   bool
	DeleteRequeue bool
}

func setReconcileDeleteExpectations(ctrl *gomock.Controller,
	tc reconcileDeleteTestCase,
) *baremetal_mocks.MockMachineManagerInterface {
	m := baremetal_mocks.NewMockMachineManagerInterface(ctrl)

	if tc.DeleteFails {
		m.EXPECT().Delete(context.TODO()).Return(errors.New("failed"))
		m.EXPECT().UnsetFinalizer().MaxTimes(0)
		m.EXPECT().DissociateM3Metadata(context.TODO()).MaxTimes(0)
		m.EXPECT().SetError(gomock.Any(), gomock.Any())
		return m
	} else if tc.DeleteRequeue {
		m.EXPECT().Delete(context.TODO()).Return(&baremetal.RequeueAfterError{})
		m.EXPECT().UnsetFinalizer().MaxTimes(0)
		m.EXPECT().DissociateM3Metadata(context.TODO()).MaxTimes(0)
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
				res, err := bmReconcile.reconcileNormal(context.TODO(), m)

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
			Entry("Not Annotated", reconcileNormalTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
				Annotated:     false,
			}),
			Entry("Not Annotated, Associate fails", reconcileNormalTestCase{
				ExpectError:    true,
				ExpectRequeue:  false,
				Annotated:      false,
				AssociateFails: true,
			}),
			Entry("Annotated", reconcileNormalTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
				Annotated:     true,
			}),
			Entry("GetBMHID Fails", reconcileNormalTestCase{
				ExpectError:   true,
				ExpectRequeue: false,
				GetBMHIDFails: true,
			}),
			Entry("BMH ID set", reconcileNormalTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
				BMHIDSet:      true,
			}),
			Entry("BMH ID set", reconcileNormalTestCase{
				ExpectError:        false,
				ExpectRequeue:      false,
				BMHIDSet:           true,
				GetProviderIDFails: true,
			}),
			Entry("BMH ID set, SetNodeProviderID fails", reconcileNormalTestCase{
				ExpectError:            true,
				ExpectRequeue:          false,
				BMHIDSet:               true,
				SetNodeProviderIDFails: true,
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
				res, err := bmReconcile.reconcileDelete(context.TODO(), m)

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
		)
	})

	type TestCaseMetal3ClusterToM3M struct {
		Cluster       *clusterv1.Cluster
		M3Cluster     *capm3.Metal3Cluster
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
			reqs := r.Metal3ClusterToMetal3Machines(obj)

			m3machineNames := make([]string, len(reqs))
			for i := range reqs {
				m3machineNames[i] = reqs[i].Name
			}

			if tc.ExpectRequest {
				Expect(len(reqs)).To(Equal(2), "Expected 2 Metal3 machines to reconcile but got %d", len(reqs))
				for _, expectedName := range []string{"my-metal3-machine-0", "my-metal3-machine-1"} {
					Expect(contains(m3machineNames, expectedName)).To(BeTrue(), "expected %q in slice %v", expectedName, m3machineNames)
				}
			} else {
				Expect(len(reqs)).To(Equal(0), "Expected 0 request, found %d", len(reqs))
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
				M3Cluster:     &capm3.Metal3Cluster{},
				Machine0:      newMachine(clusterName, "my-machine-0", "my-metal3-machine-0", ""),
				Machine1:      newMachine(clusterName, "my-machine-1", "my-metal3-machine-1", ""),
				Machine2:      newMachine(clusterName, "my-machine-2", "", ""),
				ExpectRequest: false,
			},
		),
	)

	type TestCaseBMHToM3M struct {
		Host          *bmh.BareMetalHost
		ExpectRequest bool
	}

	DescribeTable("BareMetalHost To Metal3Machines tests",
		func(tc TestCaseBMHToM3M) {
			r := Metal3MachineReconciler{}
			obj := client.Object(tc.Host)
			reqs := r.BareMetalHostToMetal3Machines(obj)

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
		// Given machine, but no metal3machine resource
		Entry("BareMetalHost To Metal3Machines",
			TestCaseBMHToM3M{
				Host: &bmh.BareMetalHost{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host1",
						Namespace: namespaceName,
					},
					Spec: bmh.BareMetalHostSpec{
						ConsumerRef: &corev1.ObjectReference{
							Name:       "someothermachine",
							Namespace:  namespaceName,
							Kind:       "Metal3Machine",
							APIVersion: capm3.GroupVersion.String(),
						},
					},
				},
				ExpectRequest: true,
			},
		),
		// Given machine, but no metal3machine resource
		Entry("BareMetalHost To Metal3Machines, no ConsumerRef",
			TestCaseBMHToM3M{
				Host: &bmh.BareMetalHost{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host1",
						Namespace: namespaceName,
					},
					Spec: bmh.BareMetalHostSpec{},
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
			dataClaim := &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: ownerRefs,
				},
				Spec: capm3.Metal3DataClaimSpec{},
			}
			obj := client.Object(dataClaim)
			reqs := r.Metal3DataClaimToMetal3Machines(obj)

			if tc.ExpectRequest {
				Expect(len(reqs)).To(Equal(1), "Expected 1 request, found %d", len(reqs))

				req := reqs[0]
				Expect(req.NamespacedName.Name).To(Equal(tc.OwnerRef.Name),
					"Expected name %s, found %s", tc.OwnerRef.Name, req.NamespacedName.Name)
				Expect(req.NamespacedName.Namespace).To(Equal(dataClaim.Namespace),
					"Expected namespace %s, found %s", dataClaim.Namespace, req.NamespacedName.Namespace)

			} else {
				Expect(len(reqs)).To(Equal(0), "Expected 0 request, found %d", len(reqs))

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
					Kind:       "Metal3Machine",
					APIVersion: capm3.GroupVersion.String(),
				},
				ExpectRequest: true,
			},
		),
		Entry("Wrong Kind",
			TestCaseM3DToM3M{
				OwnerRef: &metav1.OwnerReference{
					Name:       "abc",
					Kind:       "sdfousdf",
					APIVersion: capm3.GroupVersion.String(),
				},
				ExpectRequest: false,
			},
		),
		Entry("Wrong Version, should work",
			TestCaseM3DToM3M{
				OwnerRef: &metav1.OwnerReference{
					Name:       "abc",
					Kind:       "Metal3Machine",
					APIVersion: capm3.GroupVersion.Group + "/v1blah1",
				},
				ExpectRequest: true,
			},
		),
		Entry("Wrong Group, should not work",
			TestCaseM3DToM3M{
				OwnerRef: &metav1.OwnerReference{
					Name:       "abc",
					Kind:       "Metal3Machine",
					APIVersion: "foo.bar/" + capm3.GroupVersion.Version,
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
		M3Machine     *capm3.Metal3Machine
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
			reqs := r.ClusterToMetal3Machines(obj)

			if tc.ExpectRequest {
				Expect(len(reqs)).To(Equal(1), "Expected 1 request, found %d", len(reqs))
				req := capm3.Metal3Machine{}
				err := fakeClient.Get(context.TODO(), reqs[0].NamespacedName, &req)
				Expect(err).NotTo(HaveOccurred())

				Expect(req.Labels[clusterv1.ClusterLabelName]).To(Equal(tc.Cluster.Name),
					"Expected label %s, found %s", tc.Cluster.Name, req.Labels[clusterv1.ClusterLabelName])
			} else {
				Expect(len(reqs)).To(Equal(0), "Expected 0 request, found %d", len(reqs))

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
			ipClaim := &capm3.Metal3Data{
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
			reqs := r.Metal3DataToMetal3Machines(obj)
			Expect(reqs).To(Equal(tc.expectedRequests))
		},
		Entry("No OwnerRefs", testCaseMetal3DataToMetal3Machines{
			expectedRequests: []ctrl.Request{},
		}),
		Entry("OwnerRefs", testCaseMetal3DataToMetal3Machines{
			ownerRefs: []metav1.OwnerReference{
				{
					APIVersion: capm3.GroupVersion.String(),
					Kind:       "Metal3Machine",
					Name:       "abc",
				},
				{
					APIVersion: capm3.GroupVersion.String(),
					Kind:       "Metal3DataClaim",
					Name:       "bcd",
				},
				{
					APIVersion: "foo.bar/v1",
					Kind:       "Metal3Machine",
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
