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
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	testObjectMeta = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: namespaceName,
	}
	testObjectMetaWithLabel = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: namespaceName,
		Labels: map[string]string{
			clusterv1.ClusterLabelName: "abc",
		},
	}
)

var _ = Describe("Metal3Data manager", func() {

	Describe("Test Data Reconcile functions", func() {

		type testCaseReconcile struct {
			expectError          bool
			expectRequeue        bool
			expectManager        bool
			m3d                  *capm3.Metal3Data
			cluster              *clusterv1.Cluster
			managerError         bool
			reconcileNormal      bool
			reconcileNormalError bool
			releaseLeasesRequeue bool
			releaseLeasesError   bool
		}

		DescribeTable("Test Reconcile",
			func(tc testCaseReconcile) {
				gomockCtrl := gomock.NewController(GinkgoT())
				mf := baremetal_mocks.NewMockManagerFactoryInterface(gomockCtrl)
				m := baremetal_mocks.NewMockDataManagerInterface(gomockCtrl)

				objects := []client.Object{}
				if tc.m3d != nil {
					objects = append(objects, tc.m3d)
				}
				if tc.cluster != nil {
					objects = append(objects, tc.cluster)
				}
				fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()

				if tc.managerError {
					mf.EXPECT().NewDataManager(gomock.Any(), gomock.Any()).Return(nil, errors.New(""))
				} else if tc.expectManager {
					mf.EXPECT().NewDataManager(gomock.Any(), gomock.Any()).Return(m, nil)
				} else {
					mf.EXPECT().NewDataManager(gomock.Any(), gomock.Any()).MaxTimes(0)
				}
				if tc.m3d != nil && !tc.m3d.DeletionTimestamp.IsZero() {
					if tc.releaseLeasesRequeue {
						m.EXPECT().ReleaseLeases(context.TODO()).Return(&baremetal.RequeueAfterError{})
					} else if tc.releaseLeasesError {
						m.EXPECT().ReleaseLeases(context.TODO()).Return(errors.New(""))
					} else {
						m.EXPECT().ReleaseLeases(context.TODO()).Return(nil)
						m.EXPECT().UnsetFinalizer()
					}
				}

				if tc.m3d != nil && tc.m3d.DeletionTimestamp.IsZero() &&
					tc.reconcileNormal {
					m.EXPECT().SetFinalizer()
					if tc.reconcileNormalError {
						m.EXPECT().Reconcile(context.TODO()).Return(errors.New(""))
					} else {
						m.EXPECT().Reconcile(context.TODO()).Return(nil)
					}
				}

				dataReconcile := &Metal3DataReconciler{
					Client:           fakeClient,
					ManagerFactory:   mf,
					Log:              logr.Discard(),
					WatchFilterValue: "",
				}

				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "abc",
						Namespace: namespaceName,
					},
				}
				ctx := context.Background()
				result, err := dataReconcile.Reconcile(ctx, req)

				if tc.expectError || tc.managerError || tc.reconcileNormalError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
				if tc.expectRequeue {
					Expect(result.Requeue).To(BeTrue())
				} else {
					Expect(result.Requeue).To(BeFalse())
				}
				gomockCtrl.Finish()
			},
			Entry("Metal3Data not found", testCaseReconcile{}),
			Entry("Missing cluster label", testCaseReconcile{
				m3d: &capm3.Metal3Data{
					ObjectMeta: testObjectMeta,
				},
			}),
			Entry("Cluster not found", testCaseReconcile{
				m3d: &capm3.Metal3Data{
					ObjectMeta: testObjectMetaWithLabel,
				},
			}),
			Entry("Deletion, Cluster not found", testCaseReconcile{
				m3d: &capm3.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: namespaceName,
						Labels: map[string]string{
							clusterv1.ClusterLabelName: "abc",
						},
						DeletionTimestamp: &timestampNow,
					},
				},
				expectManager: true,
			}),
			Entry("Deletion, release requeue", testCaseReconcile{
				m3d: &capm3.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: namespaceName,
						Labels: map[string]string{
							clusterv1.ClusterLabelName: "abc",
						},
						DeletionTimestamp: &timestampNow,
					},
				},
				expectManager:        true,
				expectRequeue:        true,
				releaseLeasesRequeue: true,
			}),
			Entry("Deletion, release error", testCaseReconcile{
				m3d: &capm3.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: namespaceName,
						Labels: map[string]string{
							clusterv1.ClusterLabelName: "abc",
						},
						DeletionTimestamp: &timestampNow,
					},
				},
				expectManager:      true,
				expectError:        true,
				releaseLeasesError: true,
			}),
			Entry("Paused cluster", testCaseReconcile{
				m3d: &capm3.Metal3Data{
					ObjectMeta: testObjectMetaWithLabel,
				},
				cluster: &clusterv1.Cluster{
					ObjectMeta: testObjectMeta,
					Spec: clusterv1.ClusterSpec{
						Paused: true,
					},
				},
				expectRequeue: true,
			}),
			Entry("Error in manager", testCaseReconcile{
				m3d: &capm3.Metal3Data{
					ObjectMeta: testObjectMetaWithLabel,
				},
				cluster: &clusterv1.Cluster{
					ObjectMeta: testObjectMeta,
				},
				managerError: true,
			}),
			Entry("Reconcile normal error", testCaseReconcile{
				m3d: &capm3.Metal3Data{
					ObjectMeta: testObjectMetaWithLabel,
				},
				cluster: &clusterv1.Cluster{
					ObjectMeta: testObjectMeta,
				},
				reconcileNormal:      true,
				reconcileNormalError: true,
				expectManager:        true,
			}),
			Entry("Reconcile normal no error", testCaseReconcile{
				m3d: &capm3.Metal3Data{
					ObjectMeta: testObjectMetaWithLabel,
				},
				cluster: &clusterv1.Cluster{
					ObjectMeta: testObjectMeta,
				},
				reconcileNormal: true,
				expectManager:   true,
			}),
		)

		type reconcileNormalTestCase struct {
			ExpectError          bool
			ExpectRequeue        bool
			createSecretsRequeue bool
			createSecretsError   bool
		}

		DescribeTable("ReconcileNormal tests",
			func(tc reconcileNormalTestCase) {
				gomockCtrl := gomock.NewController(GinkgoT())

				fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).Build()

				dataReconcile := &Metal3DataReconciler{
					Client:           fakeClient,
					ManagerFactory:   baremetal.NewManagerFactory(fakeClient),
					Log:              logr.Discard(),
					WatchFilterValue: "",
				}
				m := baremetal_mocks.NewMockDataManagerInterface(gomockCtrl)

				m.EXPECT().SetFinalizer()

				if tc.createSecretsRequeue {
					m.EXPECT().Reconcile(context.TODO()).Return(&baremetal.RequeueAfterError{})
				} else if tc.createSecretsError {
					m.EXPECT().Reconcile(context.TODO()).Return(errors.New(""))
				} else {
					m.EXPECT().Reconcile(context.TODO()).Return(nil)
				}

				res, err := dataReconcile.reconcileNormal(context.TODO(), m)
				gomockCtrl.Finish()

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
			Entry("Reconcile Succeeds", reconcileNormalTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
			}),
			Entry("Reconcile requeues", reconcileNormalTestCase{
				ExpectError:        true,
				ExpectRequeue:      false,
				createSecretsError: true,
			}),
			Entry("Reconcile fails", reconcileNormalTestCase{
				ExpectError:          false,
				ExpectRequeue:        true,
				createSecretsRequeue: true,
			}),
		)
	})

	type reconcileDeleteTestCase struct {
		ExpectError          bool
		ExpectRequeue        bool
		ReleaseLeasesRequeue bool
		ReleaseLeasesError   bool
	}

	DescribeTable("ReconcileDelete tests",
		func(tc reconcileDeleteTestCase) {
			gomockCtrl := gomock.NewController(GinkgoT())

			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).Build()

			dataReconcile := &Metal3DataReconciler{
				Client:           fakeClient,
				ManagerFactory:   baremetal.NewManagerFactory(fakeClient),
				Log:              logr.Discard(),
				WatchFilterValue: "",
			}
			m := baremetal_mocks.NewMockDataManagerInterface(gomockCtrl)

			if tc.ReleaseLeasesRequeue {
				m.EXPECT().ReleaseLeases(context.TODO()).Return(&baremetal.RequeueAfterError{})
			} else if tc.ReleaseLeasesError {
				m.EXPECT().ReleaseLeases(context.TODO()).Return(errors.New(""))
			} else {
				m.EXPECT().ReleaseLeases(context.TODO()).Return(nil)
				m.EXPECT().UnsetFinalizer()
			}

			res, err := dataReconcile.reconcileDelete(context.TODO(), m)
			gomockCtrl.Finish()

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
		Entry("Reconcile Succeeds", reconcileDeleteTestCase{
			ExpectError:   false,
			ExpectRequeue: false,
		}),
		Entry("Reconcile requeues", reconcileDeleteTestCase{
			ExpectError:        true,
			ExpectRequeue:      false,
			ReleaseLeasesError: true,
		}),
		Entry("Reconcile fails", reconcileDeleteTestCase{
			ExpectError:          false,
			ExpectRequeue:        true,
			ReleaseLeasesRequeue: true,
		}),
	)

	type testCaseMetal3IPClaimToMetal3Data struct {
		ownerRefs        []metav1.OwnerReference
		expectedRequests []ctrl.Request
	}

	DescribeTable("test Metal3IPClaimToMetal3Data",
		func(tc testCaseMetal3IPClaimToMetal3Data) {
			ipClaim := &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespaceName,
					OwnerReferences: tc.ownerRefs,
				},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(ipClaim).Build()
			m3DataReconciler := Metal3DataReconciler{
				Client: fakeClient,
			}
			obj := client.Object(ipClaim)
			reqs := m3DataReconciler.Metal3IPClaimToMetal3Data(obj)
			Expect(reqs).To(Equal(tc.expectedRequests))
		},
		Entry("No OwnerRefs", testCaseMetal3IPClaimToMetal3Data{
			expectedRequests: []ctrl.Request{},
		}),
		Entry("OwnerRefs", testCaseMetal3IPClaimToMetal3Data{
			ownerRefs: []metav1.OwnerReference{
				{
					APIVersion: capm3.GroupVersion.String(),
					Kind:       "Metal3Data",
					Name:       "abc",
				},
				{
					APIVersion: capm3.GroupVersion.String(),
					Kind:       "Metal3DataClaim",
					Name:       "bcd",
				},
				{
					APIVersion: "foo.bar/v1",
					Kind:       "Metal3Data",
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
