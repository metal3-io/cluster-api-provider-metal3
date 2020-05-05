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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetal3IPPoolDefault(t *testing.T) {
	g := NewWithT(t)

	c := &Metal3IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: Metal3IPPoolSpec{},
	}
	c.Default()

	g.Expect(c.Spec).To(Equal(Metal3IPPoolSpec{}))
	g.Expect(c.Status).To(Equal(Metal3IPPoolStatus{}))
}

func TestMetal3IPPoolValidation(t *testing.T) {

	tests := []struct {
		name      string
		expectErr bool
		c         *Metal3IPPool
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,
			c: &Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: Metal3IPPoolSpec{},
			},
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

			g.Expect(tt.c.ValidateDelete()).To(Succeed())
		})
	}
}

func TestMetal3IPPoolUpdateValidation(t *testing.T) {

	tests := []struct {
		name      string
		expectErr bool
		newPool   *Metal3IPPoolSpec
		oldPool   *Metal3IPPoolSpec
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,
			newPool:   &Metal3IPPoolSpec{},
			oldPool:   &Metal3IPPoolSpec{},
		},
		{
			name:      "should fail when oldPool is nil",
			expectErr: true,
			newPool:   &Metal3IPPoolSpec{},
			oldPool:   nil,
		},
		{
			name:      "should fail when namePrefix value changes",
			expectErr: true,
			newPool: &Metal3IPPoolSpec{
				NamePrefix: "abcde",
			},
			oldPool: &Metal3IPPoolSpec{
				NamePrefix: "abcd",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var newPool, oldPool *Metal3IPPool
			g := NewWithT(t)
			newPool = &Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: *tt.newPool,
			}

			if tt.oldPool != nil {
				oldPool = &Metal3IPPool{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
					},
					Spec: *tt.oldPool,
				}
			} else {
				oldPool = nil
			}

			if tt.expectErr {
				g.Expect(newPool.ValidateUpdate(oldPool)).NotTo(Succeed())
			} else {
				g.Expect(newPool.ValidateUpdate(oldPool)).To(Succeed())
			}
		})
	}
}
