/*
Copyright The Kubernetes Authors.

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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type reconcileNormalRemediationTestCase struct {
	ExpectError           bool
	ExpectRequeue         bool
	GetUnhealthyHostFails bool
	HostStatusOffline     bool
	RemediationPhase      string
	IsFinalizerSet        bool
	IsPowerOffRequested   bool
	IsPoweredOn           bool
	IsNodeForbidden       bool
	IsNodeBackedUp        bool
	IsNodeDeleted         bool
	IsTimedOut            bool
	IsRetryLimitReached   bool
}

func setReconcileNormalRemediationExpectations(ctrl *gomock.Controller,
	tc reconcileNormalRemediationTestCase) *baremetal_mocks.MockRemediationManagerInterface {
	m := baremetal_mocks.NewMockRemediationManagerInterface(ctrl)

	bmh := &bmov1alpha1.BareMetalHost{}
	if tc.GetUnhealthyHostFails {
		m.EXPECT().GetUnhealthyHost(context.TODO()).Return(nil, nil, fmt.Errorf("can't find foo_bmh"))
		return m
	}
	m.EXPECT().GetUnhealthyHost(context.TODO()).Return(bmh, nil, nil)

	// If user has set bmh.Spec.Online to false, do not try to remediate the host and set remediation phase to failed
	if tc.HostStatusOffline {
		m.EXPECT().OnlineStatus(bmh).Return(false)
		m.EXPECT().SetRemediationPhase(infrav1.PhaseFailed)
		return m
	}
	m.EXPECT().OnlineStatus(bmh).Return(true)

	expectGetNode := func() {
		m.EXPECT().GetClusterClient(context.TODO())
		if tc.IsNodeForbidden {
			m.EXPECT().GetNode(context.TODO(), gomock.Any()).Return(nil, &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonForbidden}})
		} else if tc.IsNodeDeleted {
			m.EXPECT().GetNode(context.TODO(), gomock.Any()).Return(nil, &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}})
		} else {
			m.EXPECT().GetNode(context.TODO(), gomock.Any()).Return(&corev1.Node{}, nil)
		}
	}

	m.EXPECT().GetRemediationType().Return(infrav1.RebootRemediationStrategy)
	m.EXPECT().GetRemediationPhase().Return(tc.RemediationPhase).MinTimes(1)

	switch tc.RemediationPhase {
	case "":
		m.EXPECT().SetRemediationPhase(infrav1.PhaseRunning)
		m.EXPECT().SetLastRemediationTime(gomock.Any())

	case infrav1.PhaseRunning:

		expectGetNode()

		m.EXPECT().HasFinalizer().Return(tc.IsFinalizerSet)
		if !tc.IsFinalizerSet {
			m.EXPECT().SetFinalizer().Return()
			return m
		}

		m.EXPECT().IsPowerOffRequested(context.TODO()).Return(tc.IsPowerOffRequested, nil)
		if !tc.IsPowerOffRequested {
			m.EXPECT().SetPowerOffAnnotation(context.TODO())
			return m
		}

		m.EXPECT().IsPoweredOn(context.TODO()).Return(tc.IsPoweredOn, nil)
		if tc.IsPoweredOn {
			return m
		}

		if !tc.IsNodeForbidden && !tc.IsNodeDeleted {
			m.EXPECT().SetNodeBackupAnnotations(gomock.Any(), gomock.Any()).Return(!tc.IsNodeBackedUp)
			if !tc.IsNodeBackedUp {
				return m
			}
			m.EXPECT().DeleteNode(context.TODO(), gomock.Any(), gomock.Any())
			return m
		}

		m.EXPECT().SetRemediationPhase(infrav1.PhaseWaiting)
		return m

	case infrav1.PhaseWaiting:

		expectGetNode()

		m.EXPECT().IsPowerOffRequested(context.TODO()).Return(tc.IsPowerOffRequested, nil)
		if tc.IsPowerOffRequested {
			m.EXPECT().RemovePowerOffAnnotation(context.TODO())
		}

		m.EXPECT().IsPoweredOn(context.TODO()).Return(tc.IsPoweredOn, nil)
		if !tc.IsPoweredOn {
			return m
		}

		m.EXPECT().HasFinalizer().Return(tc.IsFinalizerSet)
		if tc.IsFinalizerSet {
			if !tc.IsNodeDeleted {
				m.EXPECT().GetNodeBackupAnnotations().Return("{\"foo\":\"bar\"}", "")
				m.EXPECT().UpdateNode(context.TODO(), gomock.Any(), gomock.Any())
				m.EXPECT().RemoveNodeBackupAnnotations()
				m.EXPECT().UnsetFinalizer()
				return m
			}
			if tc.IsNodeForbidden {
				m.EXPECT().UnsetFinalizer()
				return m
			}
		}

		m.EXPECT().GetTimeout().Return(&metav1.Duration{Duration: time.Second})
		m.EXPECT().TimeToRemediate(gomock.Any()).Return(tc.IsTimedOut, time.Second)
		if tc.IsTimedOut {
			m.EXPECT().RetryLimitIsSet().Return(true)
			m.EXPECT().HasReachRetryLimit().Return(tc.IsRetryLimitReached)
			if !tc.IsRetryLimitReached {
				m.EXPECT().SetRemediationPhase(infrav1.PhaseRunning)
				m.EXPECT().SetLastRemediationTime(gomock.Any())
				m.EXPECT().IncreaseRetryCount()
				return m
			}
			m.EXPECT().SetOwnerRemediatedConditionNew(context.TODO())
			m.EXPECT().SetUnhealthyAnnotation(context.TODO())
			m.EXPECT().SetRemediationPhase(infrav1.PhaseDeleting)
		}
	}
	return m
}

var _ = Describe("Metal3Remediation controller", func() {
	Describe(
		"Test MachineRemediationReconcileNormal",
		func() {
			var ctrl *gomock.Controller
			var remReconcile *Metal3RemediationReconciler

			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).Build()

				remReconcile = &Metal3RemediationReconciler{
					Client:         fakeClient,
					ManagerFactory: baremetal.NewManagerFactory(fakeClient),
					Log:            logr.Discard(),
				}
			})

			AfterEach(func() {
				ctrl.Finish()
			})

			DescribeTable("ReconcileNormal tests", func(tc reconcileNormalRemediationTestCase) {
				m := setReconcileNormalRemediationExpectations(ctrl, tc)
				res, err := remReconcile.reconcileNormal(context.TODO(), m)

				if tc.ExpectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
				if tc.ExpectRequeue {
					Expect(res.Requeue || res.RequeueAfter > 0).To(BeTrue())
				} else {
					Expect(res.Requeue || res.RequeueAfter > 0).To(BeFalse())
				}
			},
				Entry("Should error if unhealthy host not found", reconcileNormalRemediationTestCase{
					ExpectError:           true,
					ExpectRequeue:         false,
					GetUnhealthyHostFails: true,
				}),
				Entry("Should stop without remediating and set remediation phase to failed if bmh is set offline", reconcileNormalRemediationTestCase{
					ExpectError:       false,
					ExpectRequeue:     false,
					HostStatusOffline: true,
				}),
				Entry("Should set last remediation time, and then requeue", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    "",
					IsFinalizerSet:      false,
					IsPowerOffRequested: false,
					IsPoweredOn:         true,
					IsNodeBackedUp:      false,
					IsNodeDeleted:       false,
					IsTimedOut:          false,
				}),
				Entry("Should set finalizer, last remediation time and retry count, and then requeue", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseRunning,
					IsFinalizerSet:      false,
					IsPowerOffRequested: false,
					IsPoweredOn:         true,
					IsNodeBackedUp:      false,
					IsNodeDeleted:       false,
					IsTimedOut:          false,
				}),
				Entry("Should request power off and then requeue", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseRunning,
					IsFinalizerSet:      true,
					IsPowerOffRequested: false,
					IsPoweredOn:         true,
					IsNodeBackedUp:      false,
					IsNodeDeleted:       false,
					IsTimedOut:          false,
				}),
				Entry("Should requeue while still powered on", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseRunning,
					IsFinalizerSet:      true,
					IsPowerOffRequested: true,
					IsPoweredOn:         true,
					IsNodeBackedUp:      false,
					IsNodeDeleted:       false,
					IsTimedOut:          false,
				}),
				Entry("Should backup node when powered off, and then requeue", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseRunning,
					IsFinalizerSet:      true,
					IsPowerOffRequested: true,
					IsPoweredOn:         false,
					IsNodeBackedUp:      false,
					IsNodeDeleted:       false,
					IsTimedOut:          false,
				}),
				Entry("Should delete node when backed up, and then requeue", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseRunning,
					IsFinalizerSet:      true,
					IsPowerOffRequested: true,
					IsPoweredOn:         false,
					IsNodeBackedUp:      true,
					IsNodeDeleted:       false,
					IsTimedOut:          false,
				}),
				Entry("Should request power on, and then requeue", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseWaiting,
					IsFinalizerSet:      true,
					IsPowerOffRequested: true,
					IsPoweredOn:         false,
					IsNodeBackedUp:      true,
					IsNodeDeleted:       true,
					IsTimedOut:          false,
				}),
				Entry("Should requeue while still powered off", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseWaiting,
					IsFinalizerSet:      true,
					IsPowerOffRequested: false,
					IsPoweredOn:         false,
					IsNodeBackedUp:      true,
					IsNodeDeleted:       true,
					IsTimedOut:          false,
				}),
				Entry("Should requeue until node exists if not timed out", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseWaiting,
					IsFinalizerSet:      true,
					IsPowerOffRequested: false,
					IsPoweredOn:         true,
					IsNodeBackedUp:      true,
					IsNodeDeleted:       true,
					IsTimedOut:          false,
				}),
				Entry("Should detect timeout and requeue", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseWaiting,
					IsFinalizerSet:      true,
					IsPowerOffRequested: false,
					IsPoweredOn:         true,
					IsNodeBackedUp:      true,
					IsNodeDeleted:       true,
					IsTimedOut:          true,
				}),
				Entry("Should restore node and clean up and requeue", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseWaiting,
					IsFinalizerSet:      true,
					IsPowerOffRequested: false,
					IsPoweredOn:         true,
					IsNodeBackedUp:      true,
					IsNodeDeleted:       false,
					IsTimedOut:          false,
				}),
				Entry("Should skip restore node if forbidden and clean up and requeue", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseWaiting,
					IsFinalizerSet:      true,
					IsPowerOffRequested: false,
					IsPoweredOn:         true,
					IsNodeBackedUp:      true,
					IsNodeDeleted:       true,
					IsNodeForbidden:     true,
					IsTimedOut:          false,
				}),
				Entry("Should check if retry limit is reached, and restart remediation if false, and then requeue", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       true,
					RemediationPhase:    infrav1.PhaseWaiting,
					IsFinalizerSet:      true,
					IsPowerOffRequested: false,
					IsPoweredOn:         true,
					IsNodeBackedUp:      true,
					IsNodeDeleted:       true,
					IsTimedOut:          true,
					IsRetryLimitReached: false,
				}),
				Entry("Should check if retry limit is reached, and trigger machine deletion if true, and don't requeue", reconcileNormalRemediationTestCase{
					ExpectError:         false,
					ExpectRequeue:       false,
					RemediationPhase:    infrav1.PhaseWaiting,
					IsFinalizerSet:      true,
					IsPowerOffRequested: false,
					IsPoweredOn:         true,
					IsNodeBackedUp:      true,
					IsNodeDeleted:       true,
					IsTimedOut:          true,
					IsRetryLimitReached: true,
				}),
			)
		})
})
