/*
Copyright 2021 The Kubernetes Authors.
Copyright 2025 The Metal3 Authors.
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

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetal3DataClaimDefault(t *testing.T) {
	g := NewWithT(t)
	webhook := &Metal3DataClaim{}

	c := &infrav1.Metal3DataClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fooboo",
		},
	}

	_ = webhook.Default(ctx, c)

	g.Expect(c.Spec).To(Equal(infrav1.Metal3DataClaimSpec{}))
	g.Expect(c.Status).To(Equal(infrav1.Metal3DataClaimStatus{}))
}

func TestMetal3DataClaimValidation(t *testing.T) {
	valid := &infrav1.Metal3DataClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: infrav1.Metal3DataClaimSpec{
			Template: corev1.ObjectReference{
				Name:      "abc",
				Namespace: "abc",
			},
		},
	}

	invalidHost1 := valid.DeepCopy()
	invalidHost1.Spec.Template.Name = ""

	tests := []struct {
		name      string
		expectErr bool
		c         *infrav1.Metal3DataClaim
	}{
		{
			name:      "should return error when template unset",
			expectErr: true,
			c:         invalidHost1,
		},
		{
			name:      "should succeed when endpoint correct",
			expectErr: false,
			c:         valid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			webhook := &Metal3DataClaim{}

			if tt.expectErr {
				_, err := webhook.ValidateCreate(ctx, tt.c)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateCreate(ctx, tt.c)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestMetal3DataClaimUpdateValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		new       *infrav1.Metal3DataClaimSpec
		old       *infrav1.Metal3DataClaimSpec
	}{
		{
			name:      "should succeed when values are the same",
			expectErr: false,
			new: &infrav1.Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: &infrav1.Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
			},
		},
		{
			name:      "should fail with nil old",
			expectErr: true,
			new: &infrav1.Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: nil,
		},
		{
			name:      "should fail when dataTemplate is unset",
			expectErr: true,
			new: &infrav1.Metal3DataClaimSpec{
				Template: corev1.ObjectReference{},
			},
			old: &infrav1.Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
			},
		},
		{
			name:      "should fail when dataTemplate name changes",
			expectErr: true,
			new: &infrav1.Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: &infrav1.Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abcd",
				},
			},
		},
		{
			name:      "should fail when datatemplate Namespace changes",
			expectErr: true,
			new: &infrav1.Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abc",
				},
			},
			old: &infrav1.Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abcd",
				},
			},
		},
		{
			name:      "should fail when datatemplate kind changes",
			expectErr: true,
			new: &infrav1.Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
					Kind: "abc",
				},
			},
			old: &infrav1.Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
					Kind: "abcd",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var newDataClaim, oldDataClaim *infrav1.Metal3DataClaim
			g := NewWithT(t)
			webhook := &Metal3DataClaim{}

			newDataClaim = &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "abc-1",
				},
				Spec: *tt.new,
			}

			if tt.old != nil {
				oldDataClaim = &infrav1.Metal3DataClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
						Name:      "abc-1",
					},
					Spec: *tt.old,
				}
			} else {
				oldDataClaim = nil
			}

			if tt.expectErr {
				_, err := webhook.ValidateUpdate(ctx, oldDataClaim, newDataClaim)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateUpdate(ctx, oldDataClaim, newDataClaim)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
