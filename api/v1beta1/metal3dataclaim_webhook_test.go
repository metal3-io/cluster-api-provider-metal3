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

package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetal3DataClaimDefault(t *testing.T) {
	g := NewWithT(t)

	c := &Metal3DataClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fooboo",
		},
	}
	c.Default()

	g.Expect(c.Spec).To(Equal(Metal3DataClaimSpec{}))
	g.Expect(c.Status).To(Equal(Metal3DataClaimStatus{}))
}

func TestMetal3DataClaimValidation(t *testing.T) {
	valid := &Metal3DataClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: Metal3DataClaimSpec{
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
		c         *Metal3DataClaim
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

			if tt.expectErr {
				g.Expect(tt.c.ValidateCreate()).NotTo(Succeed())
			} else {
				g.Expect(tt.c.ValidateCreate()).To(Succeed())
			}
		})
	}
}

func TestMetal3DataClaimUpdateValidation(t *testing.T) {

	tests := []struct {
		name      string
		expectErr bool
		new       *Metal3DataClaimSpec
		old       *Metal3DataClaimSpec
	}{
		{
			name:      "should succeed when values are the same",
			expectErr: false,
			new: &Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: &Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
			},
		},
		{
			name:      "should fail with nil old",
			expectErr: true,
			new: &Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: nil,
		},
		{
			name:      "should fail when dataTemplate is unset",
			expectErr: true,
			new: &Metal3DataClaimSpec{
				Template: corev1.ObjectReference{},
			},
			old: &Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
			},
		},
		{
			name:      "should fail when dataTemplate name changes",
			expectErr: true,
			new: &Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: &Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abcd",
				},
			},
		},
		{
			name:      "should fail when datatemplate Namespace changes",
			expectErr: true,
			new: &Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abc",
				},
			},
			old: &Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abcd",
				},
			},
		},
		{
			name:      "should fail when datatemplate kind changes",
			expectErr: true,
			new: &Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
					Kind: "abc",
				},
			},
			old: &Metal3DataClaimSpec{
				Template: corev1.ObjectReference{
					Name: "abc",
					Kind: "abcd",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var new, old *Metal3DataClaim
			g := NewWithT(t)
			new = &Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "abc-1",
				},
				Spec: *tt.new,
			}

			if tt.old != nil {
				old = &Metal3DataClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
						Name:      "abc-1",
					},
					Spec: *tt.old,
				}
			} else {
				old = nil
			}

			if tt.expectErr {
				g.Expect(new.ValidateUpdate(old)).NotTo(Succeed())
			} else {
				g.Expect(new.ValidateUpdate(old)).To(Succeed())
			}
		})
	}
}
