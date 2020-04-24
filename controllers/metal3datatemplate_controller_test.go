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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Metal3DataTemplate manager", func() {

	type testCaseReconcile struct {
		expectError          bool
		expectRequeue        bool
		expectManager        bool
		m3dt                 *infrav1.Metal3DataTemplate
		cluster              *capi.Cluster
		managerError         bool
		reconcileNormal      bool
		reconcileNormalError bool
		reconcileDeleteError bool
	}

	DescribeTable("Test Reconcile",
		func(tc testCaseReconcile) {
			gomockCtrl := gomock.NewController(GinkgoT())
			f := baremetal_mocks.NewMockManagerFactoryInterface(gomockCtrl)
			m := baremetal_mocks.NewMockDataTemplateManagerInterface(gomockCtrl)

			objects := []runtime.Object{}
			if tc.m3dt != nil {
				objects = append(objects, tc.m3dt)
			}
			if tc.cluster != nil {
				objects = append(objects, tc.cluster)
			}
			c := fake.NewFakeClientWithScheme(setupScheme(), objects...)

			if tc.managerError {
				f.EXPECT().NewDataTemplateManager(gomock.Any(), gomock.Any()).Return(nil, errors.New(""))
			} else if tc.expectManager {
				f.EXPECT().NewDataTemplateManager(gomock.Any(), gomock.Any()).Return(m, nil)
			}
			if tc.m3dt != nil && !tc.m3dt.DeletionTimestamp.IsZero() && tc.reconcileDeleteError {
				m.EXPECT().DeleteDatas(context.TODO()).Return(errors.New(""))
			} else if tc.m3dt != nil && !tc.m3dt.DeletionTimestamp.IsZero() {
				m.EXPECT().DeleteDatas(context.TODO()).Return(nil)
				m.EXPECT().DeleteReady().Return(true, nil)
				m.EXPECT().UnsetFinalizer()
			}

			if tc.m3dt != nil && tc.m3dt.DeletionTimestamp.IsZero() &&
				tc.reconcileNormal {
				m.EXPECT().SetFinalizer()
				m.EXPECT().RecreateStatusConditionally(context.TODO()).Return(nil)
				m.EXPECT().DeleteDatas(context.TODO()).Return(nil)
				if tc.reconcileNormalError {
					m.EXPECT().CreateDatas(context.TODO()).Return(errors.New(""))
				} else {
					m.EXPECT().CreateDatas(context.TODO()).Return(nil)
				}
			}

			dataTemplateReconcile := &Metal3DataTemplateReconciler{
				Client:         c,
				ManagerFactory: f,
				Log:            klogr.New(),
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "abc",
					Namespace: "myns",
				},
			}

			result, err := dataTemplateReconcile.Reconcile(req)

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
		Entry("Metal3DataTemplate not found", testCaseReconcile{}),
		Entry("Missing cluster label", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
			},
		}),
		Entry("Cluster not found", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMetaWithLabel,
			},
		}),
		Entry("Deletion, Cluster not found", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
					Labels: map[string]string{
						capi.ClusterLabelName: "abc",
					},
					DeletionTimestamp: &timestampNow,
				},
			},
			expectManager: true,
		}),
		Entry("Deletion, Cluster not found, error", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
					Labels: map[string]string{
						capi.ClusterLabelName: "abc",
					},
					DeletionTimestamp: &timestampNow,
				},
			},
			expectManager:        true,
			reconcileDeleteError: true,
			expectError:          true,
		}),
		Entry("Paused cluster", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMetaWithLabel,
			},
			cluster: &capi.Cluster{
				ObjectMeta: testObjectMeta,
				Spec: capi.ClusterSpec{
					Paused: true,
				},
			},
			expectRequeue: true,
		}),
		Entry("Error in manager", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMetaWithLabel,
			},
			cluster: &capi.Cluster{
				ObjectMeta: testObjectMeta,
			},
			managerError: true,
		}),
		Entry("Reconcile normal error", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMetaWithLabel,
			},
			cluster: &capi.Cluster{
				ObjectMeta: testObjectMeta,
			},
			reconcileNormal:      true,
			reconcileNormalError: true,
			expectManager:        true,
		}),
		Entry("Reconcile normal no error", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMetaWithLabel,
			},
			cluster: &capi.Cluster{
				ObjectMeta: testObjectMeta,
			},
			reconcileNormal: true,
			expectManager:   true,
		}),
	)

	type reconcileNormalTestCase struct {
		ExpectError   bool
		ExpectRequeue bool
		RecreateError bool
		DeleteError   bool
		CreateError   bool
	}

	DescribeTable("ReconcileNormal tests",
		func(tc reconcileNormalTestCase) {
			gomockCtrl := gomock.NewController(GinkgoT())

			c := fake.NewFakeClientWithScheme(setupScheme())

			dataTemplateReconcile := &Metal3DataTemplateReconciler{
				Client:         c,
				ManagerFactory: baremetal.NewManagerFactory(c),
				Log:            klogr.New(),
			}
			m := baremetal_mocks.NewMockDataTemplateManagerInterface(gomockCtrl)

			m.EXPECT().SetFinalizer()

			if !tc.RecreateError && !tc.DeleteError && !tc.CreateError {
				m.EXPECT().RecreateStatusConditionally(context.TODO()).Return(nil)
				m.EXPECT().DeleteDatas(context.TODO()).Return(nil)
				m.EXPECT().CreateDatas(context.TODO()).Return(nil)
			} else if !tc.RecreateError && !tc.DeleteError {
				m.EXPECT().RecreateStatusConditionally(context.TODO()).Return(nil)
				m.EXPECT().DeleteDatas(context.TODO()).Return(nil)
				m.EXPECT().CreateDatas(context.TODO()).Return(errors.New(""))
			} else if !tc.RecreateError {
				m.EXPECT().RecreateStatusConditionally(context.TODO()).Return(nil)
				m.EXPECT().DeleteDatas(context.TODO()).Return(errors.New(""))
			} else {
				m.EXPECT().RecreateStatusConditionally(context.TODO()).Return(errors.New(""))
			}

			res, err := dataTemplateReconcile.reconcileNormal(context.TODO(), m)
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
		Entry("No error", reconcileNormalTestCase{
			ExpectError:   false,
			ExpectRequeue: false,
		}),
		Entry("Create error", reconcileNormalTestCase{
			CreateError:   true,
			ExpectError:   true,
			ExpectRequeue: false,
		}),
		Entry("Delete error", reconcileNormalTestCase{
			DeleteError:   true,
			ExpectError:   true,
			ExpectRequeue: false,
		}),
		Entry("Recreate error", reconcileNormalTestCase{
			RecreateError: true,
			ExpectError:   true,
			ExpectRequeue: false,
		}),
	)

	type reconcileDeleteTestCase struct {
		ExpectError      bool
		ExpectRequeue    bool
		DeleteReady      bool
		DeleteError      bool
		DeleteReadyError bool
	}

	DescribeTable("ReconcileDelete tests",
		func(tc reconcileDeleteTestCase) {
			gomockCtrl := gomock.NewController(GinkgoT())

			c := fake.NewFakeClientWithScheme(setupScheme())

			dataTemplateReconcile := &Metal3DataTemplateReconciler{
				Client:         c,
				ManagerFactory: baremetal.NewManagerFactory(c),
				Log:            klogr.New(),
			}
			m := baremetal_mocks.NewMockDataTemplateManagerInterface(gomockCtrl)

			if !tc.DeleteError && !tc.DeleteReadyError && tc.DeleteReady {
				m.EXPECT().DeleteDatas(context.TODO()).Return(nil)
				m.EXPECT().DeleteReady().Return(true, nil)
				m.EXPECT().UnsetFinalizer()
			} else if !tc.DeleteError && !tc.DeleteReadyError {
				m.EXPECT().DeleteDatas(context.TODO()).Return(nil)
				m.EXPECT().DeleteReady().Return(false, nil)
			} else if !tc.DeleteError {
				m.EXPECT().DeleteDatas(context.TODO()).Return(nil)
				m.EXPECT().DeleteReady().Return(false, errors.New(""))
			} else {
				m.EXPECT().DeleteDatas(context.TODO()).Return(errors.New(""))
			}

			res, err := dataTemplateReconcile.reconcileDelete(context.TODO(), m)
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
		Entry("No error", reconcileDeleteTestCase{
			ExpectError:   false,
			ExpectRequeue: false,
		}),
		Entry("DeleteReady error", reconcileDeleteTestCase{
			DeleteReadyError: true,
			ExpectError:      true,
			ExpectRequeue:    false,
		}),
		Entry("Delete error", reconcileDeleteTestCase{
			DeleteError:   true,
			ExpectError:   true,
			ExpectRequeue: false,
		}),
		Entry("Delete ready", reconcileDeleteTestCase{
			ExpectError:   false,
			ExpectRequeue: false,
			DeleteReady:   true,
		}),
	)

	It("Test checkMetadataError", func() {
		result, err := checkMetadataError(nil, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		result, err = checkMetadataError(errors.New("def"), "abc")
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		result, err = checkMetadataError(&baremetal.RequeueAfterError{}, "abc")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{Requeue: true}))

		result, err = checkMetadataError(
			&baremetal.RequeueAfterError{RequeueAfter: requeueAfter}, "abc",
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}))
	})

})
