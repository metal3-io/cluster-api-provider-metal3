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
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"

	baremetal_mocks "sigs.k8s.io/cluster-api-provider-baremetal/baremetal/mocks"
)

func TestReconcileNormal(t *testing.T) {
	testCases := map[string]struct {
		CreateError   bool
		UpdateError   bool
		ExpectError   bool
		ExpectRequeue bool
	}{
		"No errors": {
			CreateError:   false,
			UpdateError:   false,
			ExpectError:   false,
			ExpectRequeue: false,
		},
		"Create error": {
			CreateError:   true,
			UpdateError:   false,
			ExpectError:   true,
			ExpectRequeue: false,
		},
		"Update error": {
			CreateError:   false,
			UpdateError:   true,
			ExpectError:   true,
			ExpectRequeue: false,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var returnedError error
			ctrl := gomock.NewController(t)

			// Defer call to Finish
			defer ctrl.Finish()

			m := baremetal_mocks.NewMockClusterManagerInterface(ctrl)

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
				if err == nil {
					t.Error("Expected an error")
				}
			} else {
				if err != nil {
					t.Error("Did not expect an error")
				}
			}
			if tc.ExpectRequeue {
				if res.Requeue == false {
					t.Error("Expected a requeue")
				}
			} else {
				if res.Requeue != false {
					t.Error("Did not expect a requeue")
				}
			}
		})
	}
}

func TestReconcileDelete(t *testing.T) {
	testCases := map[string]struct {
		DescendantsCount int
		DescendantsError bool
		DeleteError      bool
		ExpectError      bool
		ExpectRequeue    bool
	}{
		"No errors": {
			DescendantsCount: 0,
			DescendantsError: false,
			DeleteError:      false,
			ExpectError:      false,
			ExpectRequeue:    false,
		},
		"Descendants left": {
			DescendantsCount: 1,
			DescendantsError: false,
			DeleteError:      false,
			ExpectError:      false,
			ExpectRequeue:    true,
		},
		"Descendants error": {
			DescendantsCount: 0,
			DescendantsError: true,
			DeleteError:      false,
			ExpectError:      true,
			ExpectRequeue:    false,
		},
		"Delete error": {
			DescendantsCount: 0,
			DescendantsError: false,
			DeleteError:      true,
			ExpectError:      true,
			ExpectRequeue:    false,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var returnedError error
			ctrl := gomock.NewController(t)

			// Defer call to Finish
			defer ctrl.Finish()

			m := baremetal_mocks.NewMockClusterManagerInterface(ctrl)

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
				CountDescendants(context.TODO()).Return(tc.DescendantsCount, returnedError)

			res, err := reconcileDelete(context.TODO(), m)

			if tc.ExpectError {
				if err == nil {
					t.Error("Expected an error")
				}
			} else {
				if err != nil {
					t.Error("Did not expect an error")
				}
			}
			if tc.ExpectRequeue {
				if res.Requeue == false {
					t.Error("Expected a requeue")
				}
			} else {
				if res.Requeue != false {
					t.Error("Did not expect a requeue")
				}
			}
		})
	}
}
