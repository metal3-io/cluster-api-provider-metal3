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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestMetal3MachineTemplateDefault(_ *testing.T) {
	// No-op because we do not default anything in M3M yet
	c := &infrav1.Metal3MachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fooboo",
		},
		Spec: infrav1.Metal3MachineTemplateSpec{},
	}
	webhook := &Metal3MachineTemplate{}

	_ = webhook.Default(ctx, c)
}

func TestMetal3MachineTemplateValidation(t *testing.T) {
	valid := &infrav1.Metal3MachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: infrav1.Metal3MachineTemplateSpec{
			Template: infrav1.Metal3MachineTemplateResource{
				Spec: infrav1.Metal3MachineSpec{
					Image: infrav1.Image{
						URL:      "http://abc.com/image",
						Checksum: "http://abc.com/image.sha256sum",
					},
				},
			},
		},
	}
	invalidURL := valid.DeepCopy()
	invalidURL.Spec.Template.Spec.Image.URL = ""

	invalidChecksum := valid.DeepCopy()
	invalidChecksum.Spec.Template.Spec.Image.Checksum = ""

	validIso := valid.DeepCopy()
	validIso.Spec.Template.Spec.Image.Checksum = ""
	validIso.Spec.Template.Spec.Image.DiskFormat = ptr.To(infrav1.LiveISODiskFormat)

	validCustomDeploy := &infrav1.Metal3MachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: infrav1.Metal3MachineTemplateSpec{
			Template: infrav1.Metal3MachineTemplateResource{
				Spec: infrav1.Metal3MachineSpec{
					CustomDeploy: &infrav1.CustomDeploy{
						Method: "install_great_stuff",
					},
				},
			},
		},
	}

	tests := []struct {
		name      string
		expectErr bool
		c         *infrav1.Metal3MachineTemplate
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
		{
			name:      "should succeed with customDeploy",
			expectErr: false,
			c:         validCustomDeploy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			webhook := &Metal3MachineTemplate{}

			if tt.expectErr {
				_, err := webhook.ValidateCreate(ctx, tt.c)
				g.Expect(err).To(HaveOccurred())
				_, err = webhook.ValidateUpdate(ctx, nil, tt.c)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateCreate(ctx, tt.c)
				g.Expect(err).NotTo(HaveOccurred())
				_, err = webhook.ValidateUpdate(ctx, nil, tt.c)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
