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

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type reconcileNormalRemediationTestCase struct {
	ExpectError                    bool
	ExpectRequeue                  bool
	ExpectRebootRemediationApplied bool
	ExpectToStartMachineDeletion   bool
	GetUnhealthyHostFails          bool
	HostStatusOffline              bool
	RemediationPhase               string
	IsLastRemediatedTimeNil        bool
	IsRetryLimitZero               bool
	IsTimeToRemediate              bool
}

func setReconcileNormalRemediationExpectations(ctrl *gomock.Controller,
	tc reconcileNormalRemediationTestCase) *baremetal_mocks.MockRemediationManagerInterface {
	m := baremetal_mocks.NewMockRemediationManagerInterface(ctrl)

	// If the test case expects us to apply the reboot annotation at some point
	// we expect these calls to be made to the remediation manager
	if tc.ExpectRebootRemediationApplied {
		m.EXPECT().SetRebootAnnotation(context.TODO())
		m.EXPECT().SetLastRemediationTime(gomock.Any())
		m.EXPECT().IncreaseRetryCount()
	} else {
		m.EXPECT().SetRebootAnnotation(context.TODO()).MaxTimes(0)
		m.EXPECT().SetLastRemediationTime(gomock.Any()).MaxTimes(0)
		m.EXPECT().IncreaseRetryCount().MaxTimes(0)
	}

	if tc.ExpectToStartMachineDeletion {
		// Go to deleting remediation phase, hand over control of the Machine to CAPI
		// and annotate the node as unhealthy.
		m.EXPECT().SetRemediationPhase(capm3.PhaseDeleting)
		m.EXPECT().SetOwnerRemediatedConditionNew(context.TODO())
		m.EXPECT().SetUnhealthyAnnotation(context.TODO())
	} else {
		m.EXPECT().SetRemediationPhase(capm3.PhaseDeleting).MaxTimes(0)
		m.EXPECT().SetOwnerRemediatedConditionNew(context.TODO()).MaxTimes(0)
		m.EXPECT().SetUnhealthyAnnotation(context.TODO()).MaxTimes(0)
	}

	bmh := &v1alpha1.BareMetalHost{}
	if tc.GetUnhealthyHostFails {
		m.EXPECT().GetUnhealthyHost(context.TODO()).Return(nil, nil, fmt.Errorf("can't find foo_bmh"))
		return m
	}
	m.EXPECT().GetUnhealthyHost(context.TODO()).Return(bmh, nil, nil)

	// If user has set bmh.Spec.Online to false, do not try to remediate the host and set remediation phase to failed
	if tc.HostStatusOffline {
		m.EXPECT().OnlineStatus(bmh).Return(false)
		m.EXPECT().SetRemediationPhase(capm3.PhaseFailed)
		return m
	}
	m.EXPECT().OnlineStatus(bmh).Return(true)

	m.EXPECT().GetRemediationType().Return(capm3.RebootRemediationStrategy)
	m.EXPECT().GetRemediationPhase().Return(tc.RemediationPhase).MinTimes(1)

	if tc.RemediationPhase == capm3.PhaseRunning {
		if tc.IsLastRemediatedTimeNil {
			// No remediation time has been set on the host, so make sure the reboot annotation is added
			m.EXPECT().GetLastRemediatedTime().Return(nil)
		} else {
			m.EXPECT().GetLastRemediatedTime().Return(&metav1.Time{Time: time.Now()})
		}

		if tc.IsRetryLimitZero {
			m.EXPECT().RetryLimitIsSet().Return(false)
			// When there's no retrying to do, we don't do remediation and instead go to waiting phase
			m.EXPECT().SetRemediationPhase(capm3.PhaseWaiting)
			return m
		}
		m.EXPECT().RetryLimitIsSet().Return(true)
		m.EXPECT().HasReachRetryLimit().Return(false)
	}

	if tc.RemediationPhase == capm3.PhaseRunning || tc.RemediationPhase == capm3.PhaseWaiting {
		m.EXPECT().GetTimeout().Return(&metav1.Duration{Duration: time.Minute})
		if tc.IsTimeToRemediate {
			m.EXPECT().TimeToRemediate(time.Minute).Return(true, time.Duration(0))
		} else {
			m.EXPECT().TimeToRemediate(time.Minute).Return(false, time.Minute)
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
				Entry("Should attempt reboot remediation to BMH, then don't requeue, if LastRemediationTime is nil and no retry limit is set", reconcileNormalRemediationTestCase{
					ExpectError:                    false,
					ExpectRequeue:                  false,
					ExpectRebootRemediationApplied: true,
					RemediationPhase:               capm3.PhaseRunning,
					IsLastRemediatedTimeNil:        true,
					IsRetryLimitZero:               true,
				}),
				Entry("Should add reboot annotation to BMH if LastRemediationTime is nil and then requeue", reconcileNormalRemediationTestCase{
					ExpectError:                    false,
					ExpectRequeue:                  true,
					ExpectRebootRemediationApplied: true,
					RemediationPhase:               capm3.PhaseRunning,
					IsLastRemediatedTimeNil:        true,
				}),
				Entry("Should add reboot annotation to BMH if running remediation and it's time to remediate", reconcileNormalRemediationTestCase{
					ExpectError:                    false,
					ExpectRequeue:                  false,
					ExpectRebootRemediationApplied: true,
					RemediationPhase:               capm3.PhaseRunning,
					IsTimeToRemediate:              true,
				}),
				Entry("Should requeue if running remediation but it's not time to remediate yet", reconcileNormalRemediationTestCase{
					ExpectError:             false,
					ExpectRequeue:           true,
					RemediationPhase:        capm3.PhaseRunning,
					IsLastRemediatedTimeNil: false,
					IsTimeToRemediate:       false,
				}),
				Entry("Should start Machine deletion (handover to CAPI) if in PhaseWaiting and it is time to remediate", reconcileNormalRemediationTestCase{
					ExpectError:                  false,
					ExpectRequeue:                false,
					ExpectToStartMachineDeletion: true,
					RemediationPhase:             capm3.PhaseWaiting,
					IsTimeToRemediate:            true,
				}),
				Entry("Should requeue if waiting but it's not time to remediate yet", reconcileNormalRemediationTestCase{
					ExpectError:       false,
					ExpectRequeue:     true,
					RemediationPhase:  capm3.PhaseWaiting,
					IsTimeToRemediate: false,
				}),
			)
		})
})
