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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetal3ClusterDefault(t *testing.T) {
	g := NewWithT(t)

	c := &Metal3Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fooboo",
		},
		Spec: Metal3ClusterSpec{
			ControlPlaneEndpoint: APIEndpoint{},
		},
	}
	c.Default()

	g.Expect(c.Spec.ControlPlaneEndpoint.Port).To(BeEquivalentTo(6443))
}

func TestMetal3ClusterValidation(t *testing.T) {
	valid := &Metal3Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: Metal3ClusterSpec{
			ControlPlaneEndpoint: APIEndpoint{
				Host: "abc.com",
				Port: 443,
			},
		},
	}
	invalidHost := valid.DeepCopy()
	invalidHost.Spec.ControlPlaneEndpoint.Host = ""

	tests := []struct {
		name      string
		expectErr bool
		c         *Metal3Cluster
	}{
		{
			name:      "should return error when endpoint empty",
			expectErr: true,
			c:         invalidHost,
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
				g.Expect(tt.c.ValidateUpdate(nil)).NotTo(Succeed())
			} else {
				g.Expect(tt.c.ValidateCreate()).To(Succeed())
				g.Expect(tt.c.ValidateUpdate(nil)).To(Succeed())
			}
		})
	}
}
