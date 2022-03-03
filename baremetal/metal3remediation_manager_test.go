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

package baremetal

import (
	"context"

	"github.com/go-logr/logr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type testCaseRemediationManager struct {
	Metal3Remediation *capm3.Metal3Remediation
	Metal3Machine     *capm3.Metal3Machine
	Machine           *clusterv1.Machine
	ExpectSuccess     bool
}

var _ = Describe("Metal3Remediation manager", func() {

	var fakeClient client.Client

	BeforeEach(func() {
		fakeClient = fake.NewClientBuilder().WithScheme(setupScheme()).Build()
	})

	Describe("Test New Remediation Manager", func() {

		DescribeTable("Test NewRemediationManager",
			func(tc testCaseRemediationManager) {
				_, err := NewRemediationManager(fakeClient,
					tc.Metal3Remediation,
					tc.Metal3Machine,
					tc.Machine,
					logr.Discard(),
				)
				if tc.ExpectSuccess {
					Expect(err).NotTo(HaveOccurred())
				} else {
					Expect(err).To(HaveOccurred())
				}
			},
			Entry("All fields defined", testCaseRemediationManager{
				Metal3Remediation: &capm3.Metal3Remediation{},
				Metal3Machine:     &capm3.Metal3Machine{},
				Machine:           &clusterv1.Machine{},
				ExpectSuccess:     true,
			}),
			Entry("None of the fields defined", testCaseRemediationManager{
				Metal3Remediation: nil,
				Metal3Machine:     nil,
				Machine:           nil,
				ExpectSuccess:     true,
			}),
		)
	})

	DescribeTable("Test Finalizers",
		func(tc testCaseRemediationManager) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			remediationMgr.SetFinalizer()

			Expect(tc.Metal3Remediation.ObjectMeta.Finalizers).To(ContainElement(
				capm3.RemediationFinalizer,
			))

			remediationMgr.UnsetFinalizer()

			Expect(tc.Metal3Remediation.ObjectMeta.Finalizers).NotTo(ContainElement(
				capm3.RemediationFinalizer,
			))
		},
		Entry("No finalizers", testCaseRemediationManager{
			Metal3Remediation: &capm3.Metal3Remediation{},
		}),
		Entry("Additional finalizers", testCaseRemediationManager{
			Metal3Remediation: &capm3.Metal3Remediation{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"foo"},
				},
			},
		}),
	)

	type testCaseRetryLimitSet struct {
		Metal3Remediation *capm3.Metal3Remediation
		ExpectTrue        bool
	}

	DescribeTable("Test if Retry Limit is set",
		func(tc testCaseRetryLimitSet) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			retryLimitIsSet := remediationMgr.RetryLimitIsSet()

			Expect(retryLimitIsSet).To(Equal(tc.ExpectTrue))
		},
		Entry("retry limit is set", testCaseRetryLimitSet{
			Metal3Remediation: &capm3.Metal3Remediation{
				Spec: capm3.Metal3RemediationSpec{
					Strategy: &capm3.RemediationStrategy{
						Type:       "",
						RetryLimit: 1,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			ExpectTrue: true,
		}),
		Entry("retry limit is set to 0", testCaseRetryLimitSet{
			Metal3Remediation: &capm3.Metal3Remediation{
				Spec: capm3.Metal3RemediationSpec{
					Strategy: &capm3.RemediationStrategy{
						Type:       "",
						RetryLimit: 0,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			ExpectTrue: false,
		}),
		Entry("retry limit is set to less than 0", testCaseRetryLimitSet{
			Metal3Remediation: &capm3.Metal3Remediation{
				Spec: capm3.Metal3RemediationSpec{
					Strategy: &capm3.RemediationStrategy{
						Type:       "",
						RetryLimit: -1,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			ExpectTrue: false,
		}),
		Entry("retry limit is not set", testCaseRetryLimitSet{
			Metal3Remediation: &capm3.Metal3Remediation{
				Spec: capm3.Metal3RemediationSpec{
					Strategy: &capm3.RemediationStrategy{
						Type:    "",
						Timeout: &metav1.Duration{},
					},
				},
			},
			ExpectTrue: false,
		}),
	)

	DescribeTable("Test if Retry Limit is reached",
		func(tc testCaseRetryLimitSet) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			retryLimitIsSet := remediationMgr.HasReachRetryLimit()

			Expect(retryLimitIsSet).To(Equal(tc.ExpectTrue))
		},
		Entry("retry limit is reached", testCaseRetryLimitSet{
			Metal3Remediation: &capm3.Metal3Remediation{
				Spec: capm3.Metal3RemediationSpec{
					Strategy: &capm3.RemediationStrategy{
						Type:       "",
						RetryLimit: 1,
						Timeout:    &metav1.Duration{},
					},
				},
				Status: capm3.Metal3RemediationStatus{
					Phase:          "",
					RetryCount:     1,
					LastRemediated: &metav1.Time{},
				},
			},
			ExpectTrue: true,
		}),
		Entry("retry limit is not reached", testCaseRetryLimitSet{
			Metal3Remediation: &capm3.Metal3Remediation{
				Spec: capm3.Metal3RemediationSpec{
					Strategy: &capm3.RemediationStrategy{
						Type:       "",
						RetryLimit: 1,
						Timeout:    &metav1.Duration{},
					},
				},
				Status: capm3.Metal3RemediationStatus{
					Phase:          "",
					RetryCount:     0,
					LastRemediated: &metav1.Time{},
				},
			},
			ExpectTrue: false,
		}),
		Entry("retry limit is not set so limit is reached", testCaseRetryLimitSet{
			Metal3Remediation: &capm3.Metal3Remediation{
				Spec: capm3.Metal3RemediationSpec{
					Strategy: &capm3.RemediationStrategy{
						Type:       "",
						RetryLimit: 0,
						Timeout:    &metav1.Duration{},
					},
				},
				Status: capm3.Metal3RemediationStatus{
					Phase:          "",
					RetryCount:     0,
					LastRemediated: &metav1.Time{},
				},
			},
			ExpectTrue: true,
		}),
	)

	type testCaseEnsureRebootAnnotation struct {
		Host              *bmh.BareMetalHost
		Metal3Remediation *capm3.Metal3Remediation
		ExpectTrue        bool
	}

	DescribeTable("Test OnlineStatus",
		func(tc testCaseEnsureRebootAnnotation) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			onlineStatus := remediationMgr.OnlineStatus(tc.Host)
			if tc.ExpectTrue {
				Expect(onlineStatus).To(BeTrue())
			} else {
				Expect(onlineStatus).To(BeFalse())
			}
		},
		Entry(" Online field in spec is set to false", testCaseEnsureRebootAnnotation{
			Host: &bmh.BareMetalHost{
				Spec: bmh.BareMetalHostSpec{
					Online: false,
				},
			},
			ExpectTrue: false,
		}),
		Entry(" Online field in spec is set to true", testCaseEnsureRebootAnnotation{
			Host: &bmh.BareMetalHost{
				Spec: bmh.BareMetalHostSpec{
					Online: true,
				},
			},
			ExpectTrue: true,
		}),
	)

	type testCaseGetUnhealthyHost struct {
		M3Machine         *capm3.Metal3Machine
		Metal3Remediation *capm3.Metal3Remediation
		ExpectPresent     bool
	}

	DescribeTable("Test GetUnhealthyHost",
		func(tc testCaseGetUnhealthyHost) {
			host := bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: namespaceName,
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(&host).Build()

			remediationMgr, err := NewRemediationManager(fakeClient, nil, tc.M3Machine, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			result, helper, err := remediationMgr.GetUnhealthyHost(context.TODO())
			if tc.ExpectPresent {
				Expect(result).NotTo(BeNil())
				Expect(helper).NotTo(BeNil())
				Expect(err).To(BeNil())
			} else {
				Expect(result).To(BeNil())
				Expect(helper).To(BeNil())
				Expect(err).NotTo(BeNil())
			}
		},
		Entry("Should find the unhealthy host", testCaseGetUnhealthyHost{
			M3Machine: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       namespaceName,
					OwnerReferences: []metav1.OwnerReference{},
					Annotations: map[string]string{
						HostAnnotation: namespaceName + "/myhost",
					},
				},
			},
			ExpectPresent: true,
		}),
		Entry("Should not find the unhealthy host", testCaseGetUnhealthyHost{
			M3Machine: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       namespaceName,
					OwnerReferences: []metav1.OwnerReference{},
					Annotations: map[string]string{
						HostAnnotation: namespaceName + "/wronghostname",
					},
				},
			},
			ExpectPresent: false,
		}),
		Entry("Should not find the host, annotation not present", testCaseGetUnhealthyHost{
			M3Machine: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       namespaceName,
					OwnerReferences: []metav1.OwnerReference{},
					Annotations:     map[string]string{},
				},
			},
			ExpectPresent: false,
		}),
	)

	type testCaseGetRemediationType struct {
		Metal3Remediation  *capm3.Metal3Remediation
		RemediationType    *capm3.RemediationType
		RemediationTypeSet bool
	}

	DescribeTable("Test GetRemediationType",
		func(tc testCaseGetRemediationType) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			remediationType := remediationMgr.GetRemediationType()
			if tc.RemediationTypeSet {
				Expect(remediationType).To(Equal(capm3.RebootRemediationStrategy))
			} else {
				Expect(remediationType).To(Equal(capm3.RemediationType("")))
			}
		},
		Entry("Remediation strategy type is set to Reboot, should return strategy", testCaseGetRemediationType{
			Metal3Remediation: &capm3.Metal3Remediation{
				Spec: capm3.Metal3RemediationSpec{
					Strategy: &capm3.RemediationStrategy{
						Type:       "Reboot",
						RetryLimit: 0,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			RemediationTypeSet: true,
		}),
		Entry("Remediation strategy type is not set should return empty string", testCaseGetRemediationType{
			Metal3Remediation: &capm3.Metal3Remediation{
				Spec: capm3.Metal3RemediationSpec{
					Strategy: &capm3.RemediationStrategy{
						Type:       "",
						RetryLimit: 0,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			RemediationTypeSet: false,
		}),
	)

	type testCaseGetRemediatedTime struct {
		Metal3Remediation *capm3.Metal3Remediation
		Remediated        bool
	}

	DescribeTable("Test GetLastRemediatedTime",
		func(tc testCaseGetRemediatedTime) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			remediationTime := remediationMgr.GetLastRemediatedTime()
			if tc.Remediated {
				Expect(remediationTime).NotTo(BeNil())
			} else {
				Expect(remediationTime).To(BeNil())
			}
		},
		Entry("Host is not yet remediated and last remediation timestamp is not set yet", testCaseGetRemediatedTime{
			Metal3Remediation: &capm3.Metal3Remediation{
				Status: capm3.Metal3RemediationStatus{},
			},
			Remediated: false,
		}),
		Entry("Host is remediated and controller has set the last remediation timestamp", testCaseGetRemediatedTime{
			Metal3Remediation: &capm3.Metal3Remediation{
				Status: capm3.Metal3RemediationStatus{
					LastRemediated: &metav1.Time{},
				},
			},
			Remediated: true,
		}),
	)

	type testCaseGetTimeout struct {
		Metal3Remediation *capm3.Metal3Remediation
		TimeoutSet        bool
	}

	DescribeTable("Test GetTimeout",
		func(tc testCaseGetTimeout) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			timeout := remediationMgr.GetTimeout()
			if tc.TimeoutSet {
				Expect(timeout).NotTo(BeNil())
			} else {
				Expect(timeout).To(BeNil())
			}
		},
		Entry("Timeout is not set", testCaseGetTimeout{
			Metal3Remediation: &capm3.Metal3Remediation{
				Spec: capm3.Metal3RemediationSpec{
					Strategy: &capm3.RemediationStrategy{},
				},
			},
			TimeoutSet: false,
		}),
		Entry("Timeout is not set", testCaseGetTimeout{
			Metal3Remediation: &capm3.Metal3Remediation{
				Spec: capm3.Metal3RemediationSpec{
					Strategy: &capm3.RemediationStrategy{
						Type:       "",
						RetryLimit: 0,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			TimeoutSet: true,
		}),
	)

	DescribeTable("Test SetRemediationPhase",
		func(tc testCaseRemediationManager) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			remediationMgr.SetRemediationPhase(capm3.PhaseRunning)

			Expect(tc.Metal3Remediation.Status.Phase).To(Equal("Running"))
		},
		Entry("No phase", testCaseRemediationManager{
			Metal3Remediation: &capm3.Metal3Remediation{},
		}),
		Entry("Overwride excisting phase", testCaseRemediationManager{
			Metal3Remediation: &capm3.Metal3Remediation{
				Status: capm3.Metal3RemediationStatus{
					Phase: "Waiting",
				},
			},
		}),
	)

	DescribeTable("Test SetLastRemediationTime",
		func(tc testCaseRemediationManager) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			now := metav1.Now()

			remediationMgr.SetLastRemediationTime(&now)

			Expect(tc.Metal3Remediation.Status.LastRemediated).ShouldNot(BeNil())
		},
		Entry("No timestamp set", testCaseRemediationManager{
			Metal3Remediation: &capm3.Metal3Remediation{},
		}),
		Entry("Overwride excisting time", testCaseRemediationManager{
			Metal3Remediation: &capm3.Metal3Remediation{
				Status: capm3.Metal3RemediationStatus{
					LastRemediated: &metav1.Time{},
				},
			},
		}),
	)

	DescribeTable("Test IncreaseRetryCount",
		func(tc testCaseRemediationManager) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			oldCount := tc.Metal3Remediation.Status.RetryCount

			remediationMgr.IncreaseRetryCount()
			newCount := oldCount + 1
			Expect(tc.Metal3Remediation.Status.RetryCount).To(Equal(newCount))
		},
		Entry("RetryCount is not set", testCaseRemediationManager{
			Metal3Remediation: &capm3.Metal3Remediation{},
		}),
		Entry("RetryCount is 0", testCaseRemediationManager{
			Metal3Remediation: &capm3.Metal3Remediation{
				Status: capm3.Metal3RemediationStatus{
					RetryCount: 0,
				},
			},
		}),
		Entry("RetryCount is 2", testCaseRemediationManager{
			Metal3Remediation: &capm3.Metal3Remediation{
				Status: capm3.Metal3RemediationStatus{
					RetryCount: 2,
				},
			},
		}),
	)

	type testCaseGetRemediationPhase struct {
		Metal3Remediation *capm3.Metal3Remediation
		Succeed           bool
	}

	DescribeTable("Test GetRemediationPhase",
		func(tc testCaseGetRemediationPhase) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			phase := remediationMgr.GetRemediationPhase()

			if tc.Succeed {
				if phase == capm3.PhaseRunning {
					Expect(tc.Metal3Remediation.Status.Phase).Should(ContainSubstring("Running"))
				}
				if phase == capm3.PhaseWaiting {
					Expect(tc.Metal3Remediation.Status.Phase).Should(ContainSubstring("Waiting"))
				}
				if phase == capm3.PhaseDeleting {
					Expect(tc.Metal3Remediation.Status.Phase).Should(ContainSubstring("Deleting machine"))
				}
			} else {
				Expect(phase).ShouldNot(ContainSubstring("Deleting machine"))
				Expect(phase).ShouldNot(ContainSubstring("Waiting"))
				Expect(phase).ShouldNot(ContainSubstring("Running"))
			}

		},
		Entry("No phase set", testCaseGetRemediationPhase{
			Metal3Remediation: &capm3.Metal3Remediation{},
			Succeed:           false,
		}),
		Entry("Phase is set to Running", testCaseGetRemediationPhase{
			Metal3Remediation: &capm3.Metal3Remediation{
				Status: capm3.Metal3RemediationStatus{
					Phase: "Running",
				},
			},
			Succeed: true,
		}),
		Entry("Phase is set to Waiting", testCaseGetRemediationPhase{
			Metal3Remediation: &capm3.Metal3Remediation{
				Status: capm3.Metal3RemediationStatus{
					Phase: "Waiting",
				},
			},
			Succeed: true,
		}),
		Entry("Phase is set to Deleting", testCaseGetRemediationPhase{
			Metal3Remediation: &capm3.Metal3Remediation{
				Status: capm3.Metal3RemediationStatus{
					Phase: "Deleting machine",
				},
			},
			Succeed: true,
		}),
		Entry("Phase is set to something else", testCaseGetRemediationPhase{
			Metal3Remediation: &capm3.Metal3Remediation{
				Status: capm3.Metal3RemediationStatus{
					Phase: "test",
				},
			},
			Succeed: false,
		}),
	)

})
