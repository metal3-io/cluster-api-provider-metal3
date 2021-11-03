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
	"k8s.io/utils/pointer"
)

func TestMetal3MachineDefault(t *testing.T) {
	// No-op because we do not default anything in M3M yet
	c := &Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fooboo",
		},
		Spec: Metal3MachineSpec{},
	}
	c.Default()
}

func TestMetal3MachineValidation(t *testing.T) {
	valid := &Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: Metal3MachineSpec{
			Image: Image{
				URL:      "http://abc.com/image",
				Checksum: "http://abc.com/image.md5sum",
			},
		},
	}
	invalidURL := valid.DeepCopy()
	invalidURL.Spec.Image.URL = ""

	invalidChecksum := valid.DeepCopy()
	invalidChecksum.Spec.Image.Checksum = ""

	validIso := valid.DeepCopy()
	validIso.Spec.Image.Checksum = ""
	validIso.Spec.Image.DiskFormat = pointer.StringPtr("live-iso")

	tests := []struct {
		name      string
		expectErr bool
		c         *Metal3Machine
	}{
		{
			name:      "should return error when url empty",
			expectErr: true,
			c:         invalidURL,
		},
		{
			name:      "should return error when checksum empty",
			expectErr: true,
			c:         invalidChecksum,
		},
		{
			name:      "should succeed when image correct",
			expectErr: false,
			c:         valid,
		},
		{
			name:      "should succeed when disk format is 'live-iso' even when checksum is empty",
			expectErr: false,
			c:         validIso,
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
