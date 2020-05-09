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

package v1alpha4

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetal3IPAddressDefault(t *testing.T) {
	g := NewWithT(t)

	c := &Metal3IPAddress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: Metal3IPAddressSpec{},
	}
	c.Default()

	g.Expect(c.Spec).To(Equal(Metal3IPAddressSpec{}))
}

func TestMetal3IPAddressCreateValidation(t *testing.T) {

	tests := []struct {
		name        string
		addressName string
		expectErr   bool
		ipPool      corev1.ObjectReference
		address     IPAddress
		claim       corev1.ObjectReference
	}{
		{
			name:        "should succeed when values and ipPools correct",
			expectErr:   false,
			addressName: "abc-1",
			ipPool: corev1.ObjectReference{
				Name: "abc",
			},
			claim: corev1.ObjectReference{
				Name: "abc",
			},
			address: "abcd",
		},
		{
			name:        "should fail without address",
			expectErr:   true,
			addressName: "abc-1",
			ipPool: corev1.ObjectReference{
				Name: "abc",
			},
			claim: corev1.ObjectReference{
				Name: "abc",
			},
		},
		{
			name:        "should fail without ipPool name",
			expectErr:   true,
			addressName: "abc-1",
			ipPool: corev1.ObjectReference{
				Namespace: "abc",
			},
			claim: corev1.ObjectReference{
				Name: "abc",
			},
			address: "abcd",
		},
		{
			name:        "should fail without claim name",
			expectErr:   true,
			addressName: "abc-1",
			ipPool: corev1.ObjectReference{
				Name: "abc",
			},
			claim: corev1.ObjectReference{
				Namespace: "abc",
			},
			address: "abcd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			obj := &Metal3IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      tt.addressName,
				},
				Spec: Metal3IPAddressSpec{
					Pool:    tt.ipPool,
					Address: tt.address,
					Claim:   tt.claim,
				},
			}

			if tt.expectErr {
				g.Expect(obj.ValidateCreate()).NotTo(Succeed())
			} else {
				g.Expect(obj.ValidateCreate()).To(Succeed())
			}

			g.Expect(obj.ValidateDelete()).To(Succeed())
		})
	}
}

func TestMetal3IPAddressUpdateValidation(t *testing.T) {

	tests := []struct {
		name      string
		expectErr bool
		new       *Metal3IPAddressSpec
		old       *Metal3IPAddressSpec
	}{
		{
			name:      "should succeed when values are the same",
			expectErr: false,
			new: &Metal3IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Claim: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
			old: &Metal3IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Claim: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail with nil old",
			expectErr: true,
			new: &Metal3IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
			old: nil,
		},
		{
			name:      "should fail when index changes",
			expectErr: true,
			new: &Metal3IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
			old: &Metal3IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcde",
			},
		},
		{
			name:      "should fail when pool name changes",
			expectErr: true,
			new: &Metal3IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
			old: &Metal3IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abcd",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail when Pool Namespace changes",
			expectErr: true,
			new: &Metal3IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abc",
				},
				Address: "abcd",
			},
			old: &Metal3IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abcd",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail when Pool kind changes",
			expectErr: true,
			new: &Metal3IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
					Kind: "abc",
				},
				Address: "abcd",
			},
			old: &Metal3IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
					Kind: "abcd",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail when Claim name changes",
			expectErr: true,
			new: &Metal3IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
			old: &Metal3IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name: "abcd",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail when Claim Namespace changes",
			expectErr: true,
			new: &Metal3IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abc",
				},
				Address: "abcd",
			},
			old: &Metal3IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abcd",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail when Claim kind changes",
			expectErr: true,
			new: &Metal3IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name: "abc",
					Kind: "abc",
				},
				Address: "abcd",
			},
			old: &Metal3IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name: "abc",
					Kind: "abcd",
				},
				Address: "abcd",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var new, old *Metal3IPAddress
			g := NewWithT(t)
			new = &Metal3IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "abc-1",
				},
				Spec: *tt.new,
			}

			if tt.old != nil {
				old = &Metal3IPAddress{
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
