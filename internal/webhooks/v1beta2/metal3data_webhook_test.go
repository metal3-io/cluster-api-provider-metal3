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

func TestMetal3DataDefault(t *testing.T) {
	g := NewWithT(t)
	webhook := &Metal3Data{}

	c := &infrav1.Metal3Data{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: infrav1.Metal3DataSpec{},
	}

	_ = webhook.Default(ctx, c)

	g.Expect(c.Spec).To(Equal(infrav1.Metal3DataSpec{}))
	g.Expect(c.Status).To(Equal(infrav1.Metal3DataStatus{}))
}

func TestMetal3DataCreateValidation(t *testing.T) {
	tests := []struct {
		name      string
		dataName  string
		expectErr bool
		template  corev1.ObjectReference
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,
			dataName:  "abc-1",
			template: corev1.ObjectReference{
				Name: "abc",
			},
		},
		{
			name:      "should fail when Name does not match datatemplate",
			expectErr: true,
			dataName:  "abcd-1",
			template: corev1.ObjectReference{
				Name: "abc",
			},
		},
		{
			name:      "should fail when Name does not match index",
			expectErr: true,
			dataName:  "abc-0",
			template: corev1.ObjectReference{
				Name: "abc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			webhook := &Metal3Data{}

			obj := &infrav1.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      tt.dataName,
				},
				Spec: infrav1.Metal3DataSpec{
					Template: tt.template,
					Index:    1,
				},
			}

			if tt.expectErr {
				_, err := webhook.ValidateCreate(ctx, obj)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateCreate(ctx, obj)
				g.Expect(err).NotTo(HaveOccurred())
			}

			obj.Spec.Index = -1

			_, err := webhook.ValidateCreate(ctx, obj)
			g.Expect(err).To(HaveOccurred())
			_, err = webhook.ValidateDelete(ctx, obj)
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestMetal3DataUpdateValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		new       *infrav1.Metal3DataSpec
		old       *infrav1.Metal3DataSpec
	}{
		{
			name:      "should succeed when values are the same",
			expectErr: false,
			new: &infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
				Index: 1,
			},
			old: &infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
				Index: 1,
			},
		},
		{
			name:      "should fail with nil old",
			expectErr: true,
			new: &infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
				Index: 1,
			},
			old: nil,
		},
		{
			name:      "should fail when index changes",
			expectErr: true,
			new: &infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
				Index: 1,
			},
			old: &infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
				Index: 2,
			},
		},
		{
			name:      "should fail when dataTemplate name changes",
			expectErr: true,
			new: &infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
				Index: 1,
			},
			old: &infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name: "abcd",
				},
				Index: 1,
			},
		},
		{
			name:      "should fail when datatemplate Namespace changes",
			expectErr: true,
			new: &infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abc",
				},
				Index: 1,
			},
			old: &infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abcd",
				},
				Index: 1,
			},
		},
		{
			name:      "should fail when datatemplate kind changes",
			expectErr: true,
			new: &infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
					Kind: "abc",
				},
				Index: 1,
			},
			old: &infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
					Kind: "abcd",
				},
				Index: 1,
			},
		},
		{
			name:      "should fail when Claim name changes",
			expectErr: true,
			new: &infrav1.Metal3DataSpec{
				Claim: corev1.ObjectReference{
					Name: "abc",
				},
				Index: 1,
			},
			old: &infrav1.Metal3DataSpec{
				Claim: corev1.ObjectReference{
					Name: "abcd",
				},
				Index: 1,
			},
		},
		{
			name:      "should fail when Claim Namespace changes",
			expectErr: true,
			new: &infrav1.Metal3DataSpec{
				Claim: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abc",
				},
				Index: 1,
			},
			old: &infrav1.Metal3DataSpec{
				Claim: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abcd",
				},
				Index: 1,
			},
		},
		{
			name:      "should fail when Claim kind changes",
			expectErr: true,
			new: &infrav1.Metal3DataSpec{
				Claim: corev1.ObjectReference{
					Name: "abc",
					Kind: "abc",
				},
				Index: 1,
			},
			old: &infrav1.Metal3DataSpec{
				Claim: corev1.ObjectReference{
					Name: "abc",
					Kind: "abcd",
				},
				Index: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var newData, oldData *infrav1.Metal3Data
			g := NewWithT(t)
			webhook := &Metal3Data{}

			newData = &infrav1.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "abc-1",
				},
				Spec: *tt.new,
			}

			if tt.old != nil {
				oldData = &infrav1.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
						Name:      "abc-1",
					},
					Spec: *tt.old,
				}
			} else {
				oldData = nil
			}

			if tt.expectErr {
				_, err := webhook.ValidateUpdate(ctx, oldData, newData)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateUpdate(ctx, oldData, newData)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
