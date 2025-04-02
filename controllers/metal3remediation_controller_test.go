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
	"errors"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	ownerMachineErrorMsg     = "metal3Remediation's owner Machine could not be retrieved"
	ownerMachineNotSetMsg    = "metal3Remediation's owner Machine not set"
	metal3MachineNotFoundMsg = "metal3machine not found"
)

type reconcileNormalRemediationTestCase struct {
	ExpectError                  bool
	ExpectRequeue                bool
	GetUnhealthyHostFails        bool
	GetRemediationTypeFails      bool
	HostStatusOffline            bool
	RemediationPhase             string
	IsFinalizerSet               bool
	IsPowerOffRequested          bool
	IsPoweredOn                  bool
	IsNodeForbidden              bool
	IsNodeBackedUp               bool
	IsNodeDeleted                bool
	IsTimedOut                   bool
	IsRetryLimitReached          bool
	IsOutOfServiceTaintSupported bool
	IsOutOfServiceTaintAdded     bool
	IsNodeDrained                bool
}

type reconcileRemediationTestCase struct {
	TestRequest                      ctrl.Request
	ExpectedError                    *string
	Metal3RemediationCantBeFound     bool
	FailedToCreateRemediationManager bool
	Metal3Remediation                *infrav1.Metal3Remediation
	Machine                          *clusterv1.Machine
}
type marshallRemediationTestCase struct {
	Map map[string]string
}
type unMarshallRemediationTestCase struct {
	Marshaled   string
	ExpectError bool
}

func setReconcileNormalRemediationExpectations(ctrl *gomock.Controller,
	tc reconcileNormalRemediationTestCase) *baremetal_mocks.MockRemediationManagerInterface {
	m := baremetal_mocks.NewMockRemediationManagerInterface(ctrl)

	bmh := &bmov1alpha1.BareMetalHost{}
	if tc.GetUnhealthyHostFails {
		m.EXPECT().GetUnhealthyHost(context.TODO()).Return(nil, nil, errors.New("can't find foo_bmh"))
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

	node := &corev1.Node{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"foo": "bar"},
			Labels:      map[string]string{"answer": "42"},
		},
		Spec:   corev1.NodeSpec{},
		Status: corev1.NodeStatus{},
	}

	expectGetNode := func() {
		m.EXPECT().GetClusterClient(context.TODO())
		if tc.IsNodeForbidden {
			m.EXPECT().GetNode(context.TODO(), gomock.Any()).Return(nil, &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonForbidden}})
		} else if tc.IsNodeDeleted {
			m.EXPECT().GetNode(context.TODO(), gomock.Any()).Return(nil, &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}})
		} else {
			m.EXPECT().GetNode(context.TODO(), gomock.Any()).Return(node, nil)
		}
	}

	if tc.GetRemediationTypeFails {
		const wrongRemediationStrategy infrav1.RemediationType = "wrongRemediationStrategy"
		m.EXPECT().GetRemediationType().Return(wrongRemediationStrategy)
		return m
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
		if tc.IsOutOfServiceTaintSupported {
			if !tc.IsOutOfServiceTaintAdded {
				m.EXPECT().HasOutOfServiceTaint(gomock.Any()).Return(false)
				m.EXPECT().AddOutOfServiceTaint(context.TODO(), gomock.Any(), gomock.Any()).Return(nil)
				return m
			}

			m.EXPECT().HasOutOfServiceTaint(gomock.Any()).Return(true)
			m.EXPECT().IsNodeDrained(context.TODO(), gomock.Any(), gomock.Any()).Return(tc.IsNodeDrained)
			if !tc.IsNodeDrained {
				return m
			}
		} else if !tc.IsNodeForbidden && !tc.IsNodeDeleted {
			m.EXPECT().SetNodeBackupAnnotations("{\"foo\":\"bar\"}", "{\"answer\":\"42\"}").Return(!tc.IsNodeBackedUp)
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
			if tc.IsOutOfServiceTaintSupported {
				if tc.IsOutOfServiceTaintAdded {
					m.EXPECT().HasOutOfServiceTaint(gomock.Any()).Return(true)
					m.EXPECT().RemoveOutOfServiceTaint(context.TODO(), gomock.Any(), gomock.Any()).Return(nil)
					m.EXPECT().UnsetFinalizer()
					return m
				}
			} else {
				if !tc.IsNodeDeleted {
					m.EXPECT().GetNodeBackupAnnotations().Return("{\"foo\":\"bar\"}", "{\"answer\":\"42\"}")
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

	case infrav1.PhaseDeleting:
		expectGetNode()

	case infrav1.PhaseFailed:
		expectGetNode()
	}
	return m
}

var _ = Describe("Metal3Remediation controller", func() {
	var goMockCtrl *gomock.Controller
	var testReconciler *Metal3RemediationReconciler
	var fakeClient client.WithWatch
	var objects []client.Object

	defaultTestRequest := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      metal3RemediationName,
			Namespace: namespaceName,
		},
	}
	defaultCluster := newCluster(clusterName, nil, nil)

	BeforeEach(func() {
		goMockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		goMockCtrl.Finish()
	})

	DescribeTable("Metal3Remediation Reconcile test",
		func(tc reconcileRemediationTestCase) {
			objects = []client.Object{}

			if tc.Metal3RemediationCantBeFound {
				fakeClient = fake.NewClientBuilder().WithScheme(setupScheme()).Build()
			} else {
				objects = append(objects, defaultCluster)
				if tc.Metal3Remediation != nil {
					objects = append(objects, tc.Metal3Remediation)
				}
				if tc.Machine != nil {
					objects = append(objects, tc.Machine)
				}
				fakeClient = fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			}
			testReconciler = &Metal3RemediationReconciler{
				Client:         fakeClient,
				ManagerFactory: baremetal.NewManagerFactory(fakeClient),
				Log:            logr.Discard(),
			}
			_, err := testReconciler.Reconcile(context.TODO(), tc.TestRequest)
			evaluateTestError(tc.ExpectedError, err)
		},
		Entry("Metal3Remediation haven't been found",
			reconcileRemediationTestCase{
				TestRequest:                  defaultTestRequest,
				ExpectedError:                nil,
				Metal3RemediationCantBeFound: true,
			}),
		Entry("Error retrieving owner machine",
			reconcileRemediationTestCase{
				TestRequest:                      defaultTestRequest,
				ExpectedError:                    &ownerMachineErrorMsg,
				FailedToCreateRemediationManager: true,
				Metal3Remediation: &infrav1.Metal3Remediation{ObjectMeta: metav1.ObjectMeta{
					Name:      metal3RemediationName,
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "wrongName",
						},
					},
				}},
				Machine: newMachine(clusterName, machineName, metal3machineName, "mynode"),
			}),
		Entry("Owner Reference kind is wrong",
			reconcileRemediationTestCase{
				TestRequest:                      defaultTestRequest,
				ExpectedError:                    &ownerMachineNotSetMsg,
				FailedToCreateRemediationManager: true,
				Metal3Remediation: &infrav1.Metal3Remediation{ObjectMeta: metav1.ObjectMeta{
					Name:      metal3RemediationName,
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "wrongKind",
							Name:       machineName,
						},
					},
				}},
				Machine: newMachine(clusterName, machineName, metal3machineName, "mynode"),
			}),
		Entry("Failed to retrieve metal3machine",
			reconcileRemediationTestCase{
				TestRequest:                      defaultTestRequest,
				ExpectedError:                    &metal3MachineNotFoundMsg,
				FailedToCreateRemediationManager: true,
				Metal3Remediation: &infrav1.Metal3Remediation{ObjectMeta: metav1.ObjectMeta{
					Name:      metal3RemediationName,
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       machineName,
						},
					},
				}},
				Machine: newMachine(clusterName, machineName, "", "mynode"),
			}),
	)

	DescribeTable("ReconcileNormal tests", func(tc reconcileNormalRemediationTestCase) {
		fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).Build()
		testReconciler = &Metal3RemediationReconciler{
			Client:                     fakeClient,
			ManagerFactory:             baremetal.NewManagerFactory(fakeClient),
			Log:                        logr.Discard(),
			IsOutOfServiceTaintEnabled: tc.IsOutOfServiceTaintSupported,
		}
		m := setReconcileNormalRemediationExpectations(goMockCtrl, tc)
		res, err := testReconciler.reconcileNormal(context.TODO(), m)

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
		Entry("Should stop without remediating if remediation type is not RebootRemediationStrategy", reconcileNormalRemediationTestCase{
			ExpectError:             false,
			ExpectRequeue:           false,
			GetRemediationTypeFails: true,
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
		Entry("[OOST] Should set out-of-service taint, and then requeue", reconcileNormalRemediationTestCase{
			ExpectError:                  false,
			ExpectRequeue:                true,
			RemediationPhase:             infrav1.PhaseRunning,
			IsFinalizerSet:               true,
			IsPowerOffRequested:          true,
			IsPoweredOn:                  false,
			IsOutOfServiceTaintSupported: true,
			IsOutOfServiceTaintAdded:     false,
		}),
		Entry("[OOST] Should requeue if Node is not drained yet", reconcileNormalRemediationTestCase{
			ExpectError:                  false,
			ExpectRequeue:                true,
			RemediationPhase:             infrav1.PhaseRunning,
			IsFinalizerSet:               true,
			IsPowerOffRequested:          true,
			IsPoweredOn:                  false,
			IsOutOfServiceTaintSupported: true,
			IsOutOfServiceTaintAdded:     true,
			IsNodeDrained:                false,
		}),
		Entry("[OOST] Should update phase when node is drained", reconcileNormalRemediationTestCase{
			ExpectError:                  false,
			ExpectRequeue:                true,
			RemediationPhase:             infrav1.PhaseRunning,
			IsFinalizerSet:               true,
			IsPowerOffRequested:          true,
			IsPoweredOn:                  false,
			IsOutOfServiceTaintSupported: true,
			IsOutOfServiceTaintAdded:     true,
			IsNodeDrained:                true,
		}),
		Entry("[OOST] Should remove out-of-service taint and requeue", reconcileNormalRemediationTestCase{
			ExpectError:                  false,
			ExpectRequeue:                true,
			RemediationPhase:             infrav1.PhaseWaiting,
			IsFinalizerSet:               true,
			IsPowerOffRequested:          false,
			IsPoweredOn:                  true,
			IsOutOfServiceTaintSupported: true,
			IsOutOfServiceTaintAdded:     true,
			IsNodeDrained:                true,
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
		Entry("Should update phase when node is deleted", reconcileNormalRemediationTestCase{
			ExpectError:         false,
			ExpectRequeue:       true,
			RemediationPhase:    infrav1.PhaseRunning,
			IsFinalizerSet:      true,
			IsPowerOffRequested: true,
			IsPoweredOn:         false,
			IsNodeBackedUp:      true,
			IsNodeDeleted:       true,
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
		Entry("Should not requeue for Phase Deleting", reconcileNormalRemediationTestCase{
			ExpectError:      false,
			ExpectRequeue:    false,
			RemediationPhase: infrav1.PhaseDeleting,
		}),
		Entry("Should not requeue for Phase Failed", reconcileNormalRemediationTestCase{
			ExpectError:      false,
			ExpectRequeue:    false,
			RemediationPhase: infrav1.PhaseFailed,
		}),
	)

	DescribeTable("Metal3Remediation marshal test",
		func(tc marshallRemediationTestCase) {
			nodeAnnotations, err := marshal(tc.Map)
			if tc.Map == nil {
				Expect(nodeAnnotations).To(Equal(""))
			} else {
				Expect(nodeAnnotations).ToNot(Equal(""))
			}
			Expect(err).ToNot(HaveOccurred())

		},
		Entry("Should return an empty string",
			marshallRemediationTestCase{
				Map: nil,
			}),
		Entry("Should return marshaled string",
			marshallRemediationTestCase{
				Map: map[string]string{"answer": "42"},
			}),
	)

	DescribeTable("Metal3Remediation unmarshal test",
		func(tc unMarshallRemediationTestCase) {
			_, err := unmarshal(tc.Marshaled)
			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Should unmarshal string",
			unMarshallRemediationTestCase{
				Marshaled:   `{"bar":"2","baz":"3","foo":"1"}`,
				ExpectError: false,
			}),
		Entry("Should return error",
			unMarshallRemediationTestCase{
				Marshaled:   `{"bar":"2","baz":"3","foo"}`,
				ExpectError: true,
			}),
	)

})
