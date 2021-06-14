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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	"github.com/pkg/errors"
)

var _ = Describe("Metal3Cluster controller", func() {

	type testCaseClusterNormal struct {
		CreateError   bool
		UpdateError   bool
		ExpectError   bool
		ExpectRequeue bool
	}

	type testCaseClusterDelete struct {
		DescendantsCount int
		DescendantsError bool
		DeleteError      bool
		ExpectError      bool
		ExpectRequeue    bool
	}

	var gomockCtrl *gomock.Controller

	BeforeEach(func() {
		gomockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		gomockCtrl.Finish()
	})

	DescribeTable("Test ClusterReconcileNormal",
		func(tc testCaseClusterNormal) {
			var returnedError error
			m := baremetal_mocks.NewMockClusterManagerInterface(gomockCtrl)

			m.EXPECT().SetFinalizer()

			if tc.CreateError {
				returnedError = errors.New("Error")
				m.EXPECT().UpdateClusterStatus().MaxTimes(0)
			} else {
				if tc.UpdateError {
					returnedError = errors.New("Error")
				} else {
					returnedError = nil
				}
				m.EXPECT().UpdateClusterStatus().Return(returnedError)
				returnedError = nil
			}
			m.EXPECT().
				Create(context.TODO()).Return(returnedError)

			res, err := reconcileNormal(context.TODO(), m)

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
		Entry("No errors", testCaseClusterNormal{
			CreateError:   false,
			UpdateError:   false,
			ExpectError:   false,
			ExpectRequeue: false,
		}),
		Entry("Create error", testCaseClusterNormal{
			CreateError:   true,
			UpdateError:   false,
			ExpectError:   true,
			ExpectRequeue: false,
		}),
		Entry("Update error", testCaseClusterNormal{
			CreateError:   false,
			UpdateError:   true,
			ExpectError:   true,
			ExpectRequeue: false,
		}),
	)

	DescribeTable("Test ClusterReconcileDelete",
		func(tc testCaseClusterDelete) {
			var returnedError error
			m := baremetal_mocks.NewMockClusterManagerInterface(gomockCtrl)

			// If we get an error while listing descendants or some still exists,
			// we will exit with error or requeue.
			if tc.DescendantsError || tc.DescendantsCount != 0 {
				m.EXPECT().Delete().MaxTimes(0)
				m.EXPECT().UnsetFinalizer().MaxTimes(0)
			} else {
				// if no descendants are left, but we hit an error during delete,
				// we do not remove the finalizers
				if tc.DeleteError {
					m.EXPECT().UnsetFinalizer().MaxTimes(0)
					returnedError = errors.New("Error")
				} else {
					m.EXPECT().UnsetFinalizer()
					returnedError = nil
				}
				m.EXPECT().Delete().Return(returnedError)
			}

			if tc.DescendantsError {
				returnedError = errors.New("Error")
			} else {
				returnedError = nil
			}
			m.EXPECT().
				CountDescendants(context.TODO()).Return(tc.DescendantsCount,
				returnedError,
			)

			res, err := reconcileDelete(context.TODO(), m)

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
		Entry("No errors", testCaseClusterDelete{
			DescendantsCount: 0,
			DescendantsError: false,
			DeleteError:      false,
			ExpectError:      false,
			ExpectRequeue:    false,
		}),
		Entry("Descendants left", testCaseClusterDelete{
			DescendantsCount: 1,
			DescendantsError: false,
			DeleteError:      false,
			ExpectError:      false,
			ExpectRequeue:    true,
		}),
		Entry("Descendants error", testCaseClusterDelete{
			DescendantsCount: 0,
			DescendantsError: true,
			DeleteError:      false,
			ExpectError:      true,
			ExpectRequeue:    false,
		}),
		Entry("Delete error", testCaseClusterDelete{
			DescendantsCount: 0,
			DescendantsError: false,
			DeleteError:      true,
			ExpectError:      true,
			ExpectRequeue:    false,
		}),
	)
})
