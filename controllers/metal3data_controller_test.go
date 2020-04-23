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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Metal3Data manager", func() {

	Describe("Test Data Reconcile functions", func() {

		type testCaseReconcile struct {
			expectError          bool
			expectRequeue        bool
			expectManager        bool
			m3d                  *infrav1.Metal3Data
			cluster              *capi.Cluster
			managerError         bool
			reconcileNormal      bool
			reconcileNormalError bool
		}

		DescribeTable("Test Reconcile",
			func(tc testCaseReconcile) {
				gomockCtrl := gomock.NewController(GinkgoT())
				f := baremetal_mocks.NewMockManagerFactoryInterface(gomockCtrl)
				m := baremetal_mocks.NewMockDataManagerInterface(gomockCtrl)

				objects := []runtime.Object{}
				if tc.m3d != nil {
					objects = append(objects, tc.m3d)
				}
				if tc.cluster != nil {
					objects = append(objects, tc.cluster)
				}
				c := fake.NewFakeClientWithScheme(setupScheme(), objects...)

				if tc.managerError {
					f.EXPECT().NewDataManager(gomock.Any(), gomock.Any()).Return(nil, errors.New(""))
				} else if tc.expectManager {
					f.EXPECT().NewDataManager(gomock.Any(), gomock.Any()).Return(m, nil)
				} else {
					f.EXPECT().NewDataManager(gomock.Any(), gomock.Any()).MaxTimes(0)
				}
				if tc.m3d != nil && !tc.m3d.DeletionTimestamp.IsZero() {
					m.EXPECT().UnsetFinalizer()
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
					Client:         c,
					ManagerFactory: f,
					Log:            klogr.New(),
				}

				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "abc",
						Namespace: "def",
					},
				}

				result, err := dataReconcile.Reconcile(req)

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
				m3d: &infrav1.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "def",
					},
				},
			}),
			Entry("Cluster not found", testCaseReconcile{
				m3d: &infrav1.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "def",
						Labels: map[string]string{
							capi.ClusterLabelName: "abc",
						},
					},
				},
			}),
			Entry("Deletion, Cluster not found", testCaseReconcile{
				m3d: &infrav1.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "def",
						Labels: map[string]string{
							capi.ClusterLabelName: "abc",
						},
						DeletionTimestamp: &timestampNow,
					},
				},
				expectManager: true,
			}),
			Entry("Paused cluster", testCaseReconcile{
				m3d: &infrav1.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "def",
						Labels: map[string]string{
							capi.ClusterLabelName: "abc",
						},
					},
				},
				cluster: &capi.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "def",
					},
					Spec: capi.ClusterSpec{
						Paused: true,
					},
				},
				expectRequeue: true,
			}),
			Entry("Error in manager", testCaseReconcile{
				m3d: &infrav1.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "def",
						Labels: map[string]string{
							capi.ClusterLabelName: "abc",
						},
					},
				},
				cluster: &capi.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "def",
					},
				},
				managerError: true,
			}),
			Entry("Reconcile normal error", testCaseReconcile{
				m3d: &infrav1.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "def",
						Labels: map[string]string{
							capi.ClusterLabelName: "abc",
						},
					},
				},
				cluster: &capi.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "def",
					},
				},
				reconcileNormal:      true,
				reconcileNormalError: true,
				expectManager:        true,
			}),
			Entry("Reconcile normal no error", testCaseReconcile{
				m3d: &infrav1.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "def",
						Labels: map[string]string{
							capi.ClusterLabelName: "abc",
						},
					},
				},
				cluster: &capi.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "def",
					},
				},
				reconcileNormal: true,
				expectManager:   true,
			}),
		)

		type reconcileFunctionsTestCase struct {
			ExpectError          bool
			ExpectRequeue        bool
			createSecretsRequeue bool
			createSecretsError   bool
		}

		DescribeTable("Reconcile functions tests",
			func(tc reconcileFunctionsTestCase) {
				gomockCtrl := gomock.NewController(GinkgoT())

				c := fake.NewFakeClientWithScheme(setupScheme())

				dataReconcile := &Metal3DataReconciler{
					Client:         c,
					ManagerFactory: baremetal.NewManagerFactory(c),
					Log:            klogr.New(),
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

				gomockCtrl = gomock.NewController(GinkgoT())
				m = baremetal_mocks.NewMockDataManagerInterface(gomockCtrl)
				m.EXPECT().UnsetFinalizer()
				res, err = dataReconcile.reconcileDelete(context.TODO(), m)
				gomockCtrl.Finish()

				Expect(err).NotTo(HaveOccurred())
				Expect(res.Requeue).To(BeFalse())
			},
			Entry("Reconcile Succeeds", reconcileFunctionsTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
			}),
			Entry("Reconcile requeues", reconcileFunctionsTestCase{
				ExpectError:        true,
				ExpectRequeue:      false,
				createSecretsError: true,
			}),
			Entry("Reconcile fails", reconcileFunctionsTestCase{
				ExpectError:          false,
				ExpectRequeue:        true,
				createSecretsRequeue: true,
			}),
		)
	})

})
