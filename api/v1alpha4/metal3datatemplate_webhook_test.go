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

func TestMetal3DataTemplateDefault(t *testing.T) {
	g := NewWithT(t)

	c := &Metal3DataTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fooboo",
		},
		Spec: Metal3DataTemplateSpec{},
	}
	c.Default()

	g.Expect(c.Spec.MetaData).To(BeNil())
}

func TestMetal3DataTemplateValidation(t *testing.T) {
	valid := &Metal3DataTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: Metal3DataTemplateSpec{},
	}

	tests := []struct {
		name      string
		expectErr bool
		c         *Metal3DataTemplate
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,
			c:         valid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			if tt.expectErr {
				g.Expect(tt.c.ValidateCreate()).NotTo(Succeed())
				g.Expect(tt.c.ValidateUpdate(nil)).NotTo(Succeed())
			} else {
				g.Expect(tt.c.ValidateCreate()).To(Succeed())
				g.Expect(tt.c.ValidateUpdate(nil)).To(Succeed())
			}
		})
	}
}
