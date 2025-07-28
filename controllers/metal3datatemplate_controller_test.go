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
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Metal3DataTemplate manager", func() {

	type testCaseReconcile struct {
		expectError          bool
		expectRequeue        bool
		expectManager        bool
		m3dt                 *infrav1.Metal3DataTemplate
		cluster              *clusterv1.Cluster
		managerError         bool
		reconcileNormal      bool
		reconcileNormalError bool
		reconcileDeleteError bool
		setOwnerRefError     bool
	}

	DescribeTable("Test Reconcile",
		func(tc testCaseReconcile) {
			gomockCtrl := gomock.NewController(GinkgoT())
			mf := baremetal_mocks.NewMockManagerFactoryInterface(gomockCtrl)
			m := baremetal_mocks.NewMockDataTemplateManagerInterface(gomockCtrl)

			objects := []client.Object{}
			if tc.m3dt != nil {
				objects = append(objects, tc.m3dt)
			}
			if tc.cluster != nil {
				objects = append(objects, tc.cluster)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()

			if tc.managerError {
				mf.EXPECT().NewDataTemplateManager(gomock.Any(), gomock.Any()).Return(nil, errors.New(""))
			} else if tc.expectManager {
				mf.EXPECT().NewDataTemplateManager(gomock.Any(), gomock.Any()).Return(m, nil)
			}
			if tc.expectManager {
				if tc.setOwnerRefError {
					m.EXPECT().SetClusterOwnerRef(gomock.Any()).Return(errors.New(""))
				} else {
					if tc.cluster != nil {
						m.EXPECT().SetClusterOwnerRef(gomock.Any()).Return(nil)
					}
				}
			}
			if tc.m3dt != nil && !tc.m3dt.DeletionTimestamp.IsZero() && tc.reconcileDeleteError {
				m.EXPECT().UpdateDatas(context.Background()).Return(false, false, errors.New(""))
			} else if tc.m3dt != nil && !tc.m3dt.DeletionTimestamp.IsZero() {
				m.EXPECT().UpdateDatas(context.Background()).Return(false, false, nil)
				m.EXPECT().UnsetFinalizer()
			}

			if tc.m3dt != nil && tc.m3dt.DeletionTimestamp.IsZero() &&
				tc.reconcileNormal {
				m.EXPECT().SetFinalizer()
				if tc.reconcileNormalError {
					m.EXPECT().UpdateDatas(context.Background()).Return(false, false, errors.New(""))
				} else {
					m.EXPECT().UpdateDatas(context.Background()).Return(true, true, nil)
				}
			}

			r := &Metal3DataTemplateReconciler{
				Client:           fakeClient,
				ManagerFactory:   mf,
				Log:              logr.Discard(),
				WatchFilterValue: "",
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      metal3DataTemplateName,
					Namespace: namespaceName,
				},
			}
			ctx := context.Background()
			result, err := r.Reconcile(ctx, req)

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
		Entry("Cluster not found", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
				Spec:       infrav1.Metal3DataTemplateSpec{ClusterName: clusterName},
			},
		}),
		Entry("Deletion, Cluster not found", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:              metal3DataTemplateName,
					Namespace:         namespaceName,
					DeletionTimestamp: &timestampNow,
					Finalizers:        []string{"foo"},
				},
				Spec: infrav1.Metal3DataTemplateSpec{ClusterName: clusterName},
			},
			expectManager: true,
		}),
		Entry("Deletion, Cluster not found, error", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:              metal3DataTemplateName,
					Namespace:         namespaceName,
					DeletionTimestamp: &timestampNow,
					Finalizers:        []string{"foo"},
				},
				Spec: infrav1.Metal3DataTemplateSpec{ClusterName: clusterName},
			},
			expectManager:        true,
			reconcileDeleteError: true,
			expectError:          true,
		}),
		Entry("Paused cluster", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
				Spec:       infrav1.Metal3DataTemplateSpec{ClusterName: clusterName},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: testObjectMeta(clusterName, namespaceName, ""),
				Spec: clusterv1.ClusterSpec{
					Paused: ptr.To(true),
				},
			},
			expectRequeue: true,
			expectManager: true,
		}),
		Entry("Error in manager", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
				Spec:       infrav1.Metal3DataTemplateSpec{ClusterName: clusterName},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: testObjectMeta(clusterName, namespaceName, ""),
			},
			managerError: true,
		}),
		Entry("Reconcile normal error", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
				Spec:       infrav1.Metal3DataTemplateSpec{ClusterName: clusterName},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: testObjectMeta(clusterName, namespaceName, ""),
			},
			reconcileNormal:      true,
			reconcileNormalError: true,
			expectManager:        true,
		}),
		Entry("Reconcile normal no error", testCaseReconcile{
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
				Spec:       infrav1.Metal3DataTemplateSpec{ClusterName: clusterName},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: testObjectMeta(clusterName, namespaceName, ""),
			},
			reconcileNormal: true,
			expectManager:   true,
		}),
	)

	type reconcileNormalTestCase struct {
		ExpectError   bool
		ExpectRequeue bool
		UpdateError   bool
	}

	DescribeTable("ReconcileNormal tests",
		func(tc reconcileNormalTestCase) {
			gomockCtrl := gomock.NewController(GinkgoT())

			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).Build()

			r := &Metal3DataTemplateReconciler{
				Client:           fakeClient,
				ManagerFactory:   baremetal.NewManagerFactory(fakeClient),
				Log:              logr.Discard(),
				WatchFilterValue: "",
			}
			m := baremetal_mocks.NewMockDataTemplateManagerInterface(gomockCtrl)

			m.EXPECT().SetFinalizer()

			if !tc.UpdateError {
				m.EXPECT().UpdateDatas(context.TODO()).Return(true, true, nil)
			} else {
				m.EXPECT().UpdateDatas(context.TODO()).Return(false, false, errors.New(""))
			}

			res, err := r.reconcileNormal(context.TODO(), m)
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
		Entry("Update error", reconcileNormalTestCase{
			UpdateError:   true,
			ExpectError:   true,
			ExpectRequeue: false,
		}),
	)

	type reconcileDeleteTestCase struct {
		ExpectError   bool
		ExpectRequeue bool
		DeleteReady   bool
		DeleteError   bool
	}

	DescribeTable("ReconcileDelete tests",
		func(tc reconcileDeleteTestCase) {
			gomockCtrl := gomock.NewController(GinkgoT())

			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).Build()

			r := &Metal3DataTemplateReconciler{
				Client:           fakeClient,
				ManagerFactory:   baremetal.NewManagerFactory(fakeClient),
				Log:              logr.Discard(),
				WatchFilterValue: "",
			}
			m := baremetal_mocks.NewMockDataTemplateManagerInterface(gomockCtrl)

			if !tc.DeleteError && tc.DeleteReady {
				m.EXPECT().UpdateDatas(context.TODO()).Return(false, false, nil)
				m.EXPECT().UnsetFinalizer()
			} else if !tc.DeleteError {
				m.EXPECT().UpdateDatas(context.TODO()).Return(true, true, nil)
			} else {
				m.EXPECT().UpdateDatas(context.TODO()).Return(false, false, errors.New(""))
			}

			res, err := r.reconcileDelete(context.TODO(), m)
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

	type TestCaseM3DCToM3DT struct {
		DataClaim     *infrav1.Metal3DataClaim
		ExpectRequest bool
	}

	DescribeTable("Metal3DataClaim To Metal3DataTemplate tests",
		func(tc TestCaseM3DCToM3DT) {
			r := Metal3DataTemplateReconciler{}
			obj := client.Object(tc.DataClaim)
			reqs := r.Metal3DataClaimToMetal3DataTemplate(context.Background(), obj)

			if tc.ExpectRequest {
				Expect(reqs).To(HaveLen(1), "Expected 1 request, found %d", len(reqs))
				req := reqs[0]
				Expect(req.NamespacedName.Name).To(Equal(tc.DataClaim.Spec.Template.Name),
					"Expected name %s, found %s", tc.DataClaim.Spec.Template.Name, req.NamespacedName.Name)
				if tc.DataClaim.Spec.Template.Namespace == "" {
					Expect(req.NamespacedName.Namespace).To(Equal(tc.DataClaim.Namespace),
						"Expected namespace %s, found %s", tc.DataClaim.Namespace, req.NamespacedName.Namespace)
				} else {
					Expect(req.NamespacedName.Namespace).To(Equal(tc.DataClaim.Spec.Template.Namespace),
						"Expected namespace %s, found %s", tc.DataClaim.Spec.Template.Namespace,
						req.NamespacedName.Namespace)
				}

			} else {
				Expect(reqs).To(BeEmpty(), "Expected 0 request, found %d", len(reqs))
			}
		},
		Entry("No Metal3DataTemplate in Spec",
			TestCaseM3DCToM3DT{
				DataClaim: &infrav1.Metal3DataClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      metal3DataClaimName,
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataClaimSpec{},
				},
				ExpectRequest: false,
			},
		),
		Entry("Metal3DataTemplate in Spec, with namespace",
			TestCaseM3DCToM3DT{
				DataClaim: &infrav1.Metal3DataClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      metal3DataClaimName,
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      metal3DataTemplateName,
							Namespace: namespaceName,
						},
					},
				},
				ExpectRequest: true,
			},
		),
		Entry("Metal3DataTemplate in Spec, no namespace",
			TestCaseM3DCToM3DT{
				DataClaim: &infrav1.Metal3DataClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      metal3DataClaimName,
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name: metal3DataTemplateName,
						},
					},
				},
				ExpectRequest: true,
			},
		),
	)

	It("Test checkReconcileError", func() {
		result, err := checkReconcileError(nil, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		result, err = checkReconcileError(errors.New("def"), "abc")
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		result, err = checkReconcileError(baremetal.WithTransientError(errors.New("Failed"), 0*time.Second), "abc")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{Requeue: true}))

		result, err = checkReconcileError(
			baremetal.WithTransientError(errors.New("Failed"), requeueAfter), "abc",
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}))
	})

})
