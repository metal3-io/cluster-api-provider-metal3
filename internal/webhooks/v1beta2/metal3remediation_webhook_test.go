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

package webhooks

import (
	"testing"
	"time"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetal3RemediationValidation(t *testing.T) {
	zeroSeconds := metav1.Duration{Duration: 0}
	thirtySeconds := metav1.Duration{Duration: 30 * time.Second}
	threeMinutes := metav1.Duration{Duration: 3 * time.Minute}
	minusDuration := metav1.Duration{Duration: -1 * time.Minute}

	const WrongRemediationStrategy infrav1.RemediationType = "foo"

	tests := []struct {
		name      string
		timeout   *metav1.Duration
		limit     int
		strategy  infrav1.RemediationType
		expectErr bool
	}{
		{
			name:      "when the Timeout is not given",
			timeout:   nil,
			limit:     1,
			strategy:  infrav1.RebootRemediationStrategy,
			expectErr: false,
		},
		{
			name:      "when the Timeout is greater than 100s",
			timeout:   &threeMinutes,
			limit:     1,
			strategy:  infrav1.RebootRemediationStrategy,
			expectErr: false,
		},
		{
			name:      "when the Timeout is less than 100s",
			timeout:   &thirtySeconds,
			limit:     1,
			strategy:  infrav1.RebootRemediationStrategy,
			expectErr: true,
		},
		{
			name:      "when the Timeout is less than 0",
			timeout:   &minusDuration,
			limit:     1,
			strategy:  infrav1.RebootRemediationStrategy,
			expectErr: true,
		},
		{
			name:      "when the Timeout is 0",
			timeout:   &zeroSeconds,
			limit:     1,
			strategy:  infrav1.RebootRemediationStrategy,
			expectErr: true,
		},
		{
			name:      "when the Remediation Type is Reboot",
			timeout:   &threeMinutes,
			limit:     1,
			strategy:  infrav1.RebootRemediationStrategy,
			expectErr: false,
		},
		{
			name:      "when the Remediation Type is not Reboot",
			timeout:   &threeMinutes,
			limit:     1,
			strategy:  WrongRemediationStrategy,
			expectErr: true,
		},
		{
			name:      "when the RetryLimit is less than minRetryLimit",
			timeout:   &threeMinutes,
			limit:     0,
			strategy:  infrav1.RebootRemediationStrategy,
			expectErr: true,
		},
		{
			name:      "when the RetryLimit is minRetryLimit",
			timeout:   &threeMinutes,
			limit:     1,
			strategy:  infrav1.RebootRemediationStrategy,
			expectErr: false,
		},
		{
			name:      "when the RetryLimit is greater than minRetryLimit",
			timeout:   &threeMinutes,
			limit:     3,
			strategy:  infrav1.RebootRemediationStrategy,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		g := NewWithT(t)
		webhook := &Metal3Remediation{}

		m3r := &infrav1.Metal3Remediation{
			Spec: infrav1.Metal3RemediationSpec{
				Strategy: &infrav1.RemediationStrategy{
					Timeout:    tt.timeout,
					RetryLimit: tt.limit,
					Type:       tt.strategy,
				},
			},
		}

		if tt.expectErr {
			_, err := webhook.ValidateCreate(ctx, m3r)
			g.Expect(err).To(HaveOccurred())
			_, err = webhook.ValidateUpdate(ctx, nil, m3r)
			g.Expect(err).To(HaveOccurred())
		} else {
			_, err := webhook.ValidateCreate(ctx, m3r)
			g.Expect(err).NotTo(HaveOccurred())
			_, err = webhook.ValidateUpdate(ctx, nil, m3r)
			g.Expect(err).NotTo(HaveOccurred())
		}
	}
}
