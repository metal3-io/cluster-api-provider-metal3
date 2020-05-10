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
	// ctrl "sigs.k8s.io/controller-runtime"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Metal3IPPool controller", func() {

	type testCaseReconcile struct {
		expectError          bool
		expectRequeue        bool
		expectManager        bool
		m3ipp                *infrav1.Metal3IPPool
		cluster              *capi.Cluster
		managerError         bool
		reconcileNormal      bool
		reconcileNormalError bool
		reconcileDeleteError bool
		setOwnerRefError     bool
	}

	DescribeTable("Test Reconcile",
		func(tc testCaseReconcile) {
			gomockCtrl := gomock.NewController(GinkgoT())
			f := baremetal_mocks.NewMockManagerFactoryInterface(gomockCtrl)
			m := baremetal_mocks.NewMockIPPoolManagerInterface(gomockCtrl)

			objects := []runtime.Object{}
			if tc.m3ipp != nil {
				objects = append(objects, tc.m3ipp)
			}
			if tc.cluster != nil {
				objects = append(objects, tc.cluster)
			}
			c := fake.NewFakeClientWithScheme(setupScheme(), objects...)

			if tc.managerError {
				f.EXPECT().NewIPPoolManager(gomock.Any(), gomock.Any()).Return(nil, errors.New(""))
			} else if tc.expectManager {
				f.EXPECT().NewIPPoolManager(gomock.Any(), gomock.Any()).Return(m, nil)
			}

			if tc.expectManager {
				if tc.setOwnerRefError {
					m.EXPECT().SetClusterOwnerRef(gomock.Any()).Return(errors.New(""))
				} else {
					m.EXPECT().SetClusterOwnerRef(gomock.Any()).Return(nil)
				}
			}

			if tc.m3ipp != nil && !tc.m3ipp.DeletionTimestamp.IsZero() && tc.reconcileDeleteError {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(0, errors.New(""))
			} else if tc.m3ipp != nil && !tc.m3ipp.DeletionTimestamp.IsZero() {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(0, nil)
				m.EXPECT().UnsetFinalizer()
			}

			if tc.m3ipp != nil && tc.m3ipp.DeletionTimestamp.IsZero() &&
				tc.reconcileNormal {
				m.EXPECT().SetFinalizer()
				if tc.reconcileNormalError {
					m.EXPECT().UpdateAddresses(context.TODO()).Return(0, errors.New(""))
				} else {
					m.EXPECT().UpdateAddresses(context.TODO()).Return(1, nil)
				}
			}

			ipPoolReconcile := &Metal3IPPoolReconciler{
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

			result, err := ipPoolReconcile.Reconcile(req)

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
		Entry("Metal3IPPool not found", testCaseReconcile{}),
		Entry("Cluster not found", testCaseReconcile{
			m3ipp: &infrav1.Metal3IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       infrav1.Metal3IPPoolSpec{ClusterName: "abc"},
			},
		}),
		Entry("Deletion, Cluster not found", testCaseReconcile{
			m3ipp: &infrav1.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "abc",
					Namespace:         "myns",
					DeletionTimestamp: &timestampNow,
				},
				Spec: infrav1.Metal3IPPoolSpec{ClusterName: "abc"},
			},
			expectManager: true,
		}),
		Entry("Deletion, Cluster not found, error", testCaseReconcile{
			m3ipp: &infrav1.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "abc",
					Namespace:         "myns",
					DeletionTimestamp: &timestampNow,
				},
				Spec: infrav1.Metal3IPPoolSpec{ClusterName: "abc"},
			},
			expectManager:        true,
			reconcileDeleteError: true,
			expectError:          true,
		}),
		Entry("Paused cluster", testCaseReconcile{
			m3ipp: &infrav1.Metal3IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       infrav1.Metal3IPPoolSpec{ClusterName: "abc"},
			},
			cluster: &capi.Cluster{
				ObjectMeta: testObjectMeta,
				Spec: capi.ClusterSpec{
					Paused: true,
				},
			},
			expectRequeue: true,
			expectManager: true,
		}),
		Entry("Error in manager", testCaseReconcile{
			m3ipp: &infrav1.Metal3IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       infrav1.Metal3IPPoolSpec{ClusterName: "abc"},
			},
			cluster: &capi.Cluster{
				ObjectMeta: testObjectMeta,
			},
			managerError: true,
		}),
		Entry("Reconcile normal error", testCaseReconcile{
			m3ipp: &infrav1.Metal3IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       infrav1.Metal3IPPoolSpec{ClusterName: "abc"},
			},
			cluster: &capi.Cluster{
				ObjectMeta: testObjectMeta,
			},
			reconcileNormal:      true,
			reconcileNormalError: true,
			expectManager:        true,
		}),
		Entry("Reconcile normal no cluster", testCaseReconcile{
			m3ipp: &infrav1.Metal3IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       infrav1.Metal3IPPoolSpec{ClusterName: "abc"},
			},
			reconcileNormal: false,
			expectManager:   false,
		}),
		Entry("Reconcile normal no error", testCaseReconcile{
			m3ipp: &infrav1.Metal3IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       infrav1.Metal3IPPoolSpec{ClusterName: "abc"},
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
		UpdateError   bool
	}

	DescribeTable("ReconcileNormal tests",
		func(tc reconcileNormalTestCase) {
			gomockCtrl := gomock.NewController(GinkgoT())

			c := fake.NewFakeClientWithScheme(setupScheme())

			ipPoolReconcile := &Metal3IPPoolReconciler{
				Client:         c,
				ManagerFactory: baremetal.NewManagerFactory(c),
				Log:            klogr.New(),
			}
			m := baremetal_mocks.NewMockIPPoolManagerInterface(gomockCtrl)

			m.EXPECT().SetFinalizer()

			if !tc.UpdateError {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(1, nil)
			} else {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(0, errors.New(""))
			}

			res, err := ipPoolReconcile.reconcileNormal(context.TODO(), m)
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

			c := fake.NewFakeClientWithScheme(setupScheme())

			ipPoolReconcile := &Metal3IPPoolReconciler{
				Client:         c,
				ManagerFactory: baremetal.NewManagerFactory(c),
				Log:            klogr.New(),
			}
			m := baremetal_mocks.NewMockIPPoolManagerInterface(gomockCtrl)

			if !tc.DeleteError && tc.DeleteReady {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(0, nil)
				m.EXPECT().UnsetFinalizer()
			} else if !tc.DeleteError {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(1, nil)
			} else {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(0, errors.New(""))
			}

			res, err := ipPoolReconcile.reconcileDelete(context.TODO(), m)
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

	type TestCaseM3IPCToM3IPP struct {
		IPClaim       *infrav1.Metal3IPClaim
		ExpectRequest bool
	}

	DescribeTable("Metal3IPClaim To Metal3IPPool tests",
		func(tc TestCaseM3IPCToM3IPP) {
			r := Metal3IPPoolReconciler{}
			obj := handler.MapObject{
				Object: tc.IPClaim,
			}
			reqs := r.Metal3IPClaimToMetal3IPPool(obj)

			if tc.ExpectRequest {
				Expect(len(reqs)).To(Equal(1), "Expected 1 request, found %d", len(reqs))

				req := reqs[0]
				Expect(req.NamespacedName.Name).To(Equal(tc.IPClaim.Spec.Pool.Name),
					"Expected name %s, found %s", tc.IPClaim.Spec.Pool.Name, req.NamespacedName.Name)
				if tc.IPClaim.Spec.Pool.Namespace == "" {
					Expect(req.NamespacedName.Namespace).To(Equal(tc.IPClaim.Namespace),
						"Expected namespace %s, found %s", tc.IPClaim.Namespace, req.NamespacedName.Namespace)
				} else {
					Expect(req.NamespacedName.Namespace).To(Equal(tc.IPClaim.Spec.Pool.Namespace),
						"Expected namespace %s, found %s", tc.IPClaim.Spec.Pool.Namespace, req.NamespacedName.Namespace)
				}

			} else {
				Expect(len(reqs)).To(Equal(0), "Expected 0 request, found %d", len(reqs))

			}
		},
		Entry("No Metal3IPPool in Spec",
			TestCaseM3IPCToM3IPP{
				IPClaim: &infrav1.Metal3IPClaim{
					ObjectMeta: testObjectMeta,
					Spec:       infrav1.Metal3IPClaimSpec{},
				},
				ExpectRequest: false,
			},
		),
		Entry("Metal3IPPool in Spec, with namespace",
			TestCaseM3IPCToM3IPP{
				IPClaim: &infrav1.Metal3IPClaim{
					ObjectMeta: testObjectMeta,
					Spec: infrav1.Metal3IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
				},
				ExpectRequest: true,
			},
		),
		Entry("Metal3IPPool in Spec, no namespace",
			TestCaseM3IPCToM3IPP{
				IPClaim: &infrav1.Metal3IPClaim{
					ObjectMeta: testObjectMeta,
					Spec: infrav1.Metal3IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name: "abc",
						},
					},
				},
				ExpectRequest: true,
			},
		),
	)
})
