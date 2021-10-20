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

package v1beta1

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetal3RemediationTemplateDefault(t *testing.T) {

	g := NewWithT(t)
	m3rt := &Metal3RemediationTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-m3r",
			Namespace: "fooboo",
		},
		Spec: Metal3RemediationTemplateSpec{
			Template: Metal3RemediationTemplateResource{
				Spec: Metal3RemediationSpec{
					Strategy: &RemediationStrategy{
						Type:       "",
						RetryLimit: -1,
					},
				},
			},
		},
	}

	m3rt.Default()
	g.Expect(m3rt.Spec.Template.Spec.Strategy.Type).ToNot(BeNil())
	g.Expect(m3rt.Spec.Template.Spec.Strategy.Type).To(Equal(RebootRemediationStrategy))
	g.Expect(m3rt.Spec.Template.Spec.Strategy.RetryLimit).ToNot(BeNil())
	g.Expect(m3rt.Spec.Template.Spec.Strategy.RetryLimit).To(Equal(1))
	g.Expect(m3rt.Spec.Template.Spec.Strategy.Timeout).ToNot(BeNil())
	g.Expect(*m3rt.Spec.Template.Spec.Strategy.Timeout).To(Equal(metav1.Duration{Duration: 600 * time.Second}))

	m3rt = &Metal3RemediationTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-m3r",
			Namespace: "fooboo",
		},
		Spec: Metal3RemediationTemplateSpec{
			Template: Metal3RemediationTemplateResource{
				Spec: Metal3RemediationSpec{
					Strategy: &RemediationStrategy{
						RetryLimit: 0,
					},
				},
			},
		},
	}

	m3rt.Default()
	g.Expect(m3rt.Spec.Template.Spec.Strategy.RetryLimit).ToNot(BeNil())
	g.Expect(m3rt.Spec.Template.Spec.Strategy.RetryLimit).To(Equal(1))
}

func TestMetal3RemediationTemplateValidation(t *testing.T) {
	zeroSeconds := metav1.Duration{Duration: 0}
	thirtySeconds := metav1.Duration{Duration: 30 * time.Second}
	threeMinutes := metav1.Duration{Duration: 3 * time.Minute}
	minusDuration := metav1.Duration{Duration: -1 * time.Minute}

	const WrongRemediationStrategy RemediationType = "foo"

	tests := []struct {
		name      string
		timeout   *metav1.Duration
		limit     int
		strategy  RemediationType
		expectErr bool
	}{
		{
			name:      "when the Timeout is not given",
			timeout:   nil,
			limit:     1,
			strategy:  RebootRemediationStrategy,
			expectErr: false,
		},
		{
			name:      "when the Timeout is greater than 100s",
			timeout:   &threeMinutes,
			limit:     1,
			strategy:  RebootRemediationStrategy,
			expectErr: false,
		},
		{
			name:      "when the Timeout is less than 100s",
			timeout:   &thirtySeconds,
			limit:     1,
			strategy:  RebootRemediationStrategy,
			expectErr: true,
		},
		{
			name:      "when the Timeout is less than 0",
			timeout:   &minusDuration,
			limit:     1,
			strategy:  RebootRemediationStrategy,
			expectErr: true,
		},
		{
			name:      "when the Timeout is 0",
			timeout:   &zeroSeconds,
			limit:     1,
			strategy:  RebootRemediationStrategy,
			expectErr: true,
		},
		{
			name:      "when the Remediation Type is Reboot",
			timeout:   &threeMinutes,
			limit:     1,
			strategy:  RebootRemediationStrategy,
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
			strategy:  RebootRemediationStrategy,
			expectErr: true,
		},
		{
			name:      "when the RetryLimit is minRetryLimit",
			timeout:   &threeMinutes,
			limit:     1,
			strategy:  RebootRemediationStrategy,
			expectErr: false,
		},
		{
			name:      "when the RetryLimit is greater than minRetryLimit",
			timeout:   &threeMinutes,
			limit:     3,
			strategy:  RebootRemediationStrategy,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		g := NewWithT(t)

		m3rt := &Metal3RemediationTemplate{
			Spec: Metal3RemediationTemplateSpec{
				Template: Metal3RemediationTemplateResource{
					Spec: Metal3RemediationSpec{
						Strategy: &RemediationStrategy{
							Timeout:    tt.timeout,
							RetryLimit: tt.limit,
							Type:       tt.strategy,
						},
					},
				},
			},
		}

		if tt.expectErr {
			g.Expect(m3rt.ValidateCreate()).NotTo(Succeed())
			g.Expect(m3rt.ValidateUpdate(m3rt)).NotTo(Succeed())
		} else {
			g.Expect(m3rt.ValidateCreate()).To(Succeed())
			g.Expect(m3rt.ValidateUpdate(m3rt)).To(Succeed())
		}
	}
}
