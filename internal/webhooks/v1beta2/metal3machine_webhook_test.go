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
	"k8s.io/utils/ptr"
)

func TestMetal3MachineValidation(t *testing.T) {
	valid := &infrav1.Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: infrav1.Metal3MachineSpec{
			Image: infrav1.Image{
				URL:      "http://abc.com/image",
				Checksum: ptr.To("http://abc.com/image.sha256sum"),
			},
		},
	}
	invalidURL := valid.DeepCopy()
	invalidURL.Spec.Image.URL = ""

	invalidChecksum := valid.DeepCopy()
	invalidChecksum.Spec.Image.Checksum = ptr.To("")

	validIso := valid.DeepCopy()
	validIso.Spec.Image.Checksum = ptr.To("")
	validIso.Spec.Image.DiskFormat = infrav1.LiveISODiskFormat

	validCustomDeploy := &infrav1.Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: infrav1.Metal3MachineSpec{
			CustomDeploy: infrav1.CustomDeploy{
				Method: "install_great_stuff",
			},
		},
	}

	neitherImageNorCustomDeploy := &infrav1.Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: infrav1.Metal3MachineSpec{
			// Neither Image nor CustomDeploy specified
		},
	}

	crossNsUserData := valid.DeepCopy()
	crossNsUserData.Spec.UserData = &corev1.SecretReference{Name: "secret", Namespace: "other-ns"}

	crossNsMetaData := valid.DeepCopy()
	crossNsMetaData.Spec.MetaData = &corev1.SecretReference{Name: "secret", Namespace: "other-ns"}

	crossNsNetworkData := valid.DeepCopy()
	crossNsNetworkData.Spec.NetworkData = &corev1.SecretReference{Name: "secret", Namespace: "other-ns"}

	sameNsUserData := valid.DeepCopy()
	sameNsUserData.Spec.UserData = &corev1.SecretReference{Name: "secret", Namespace: "foo"}

	sameNsMetaData := valid.DeepCopy()
	sameNsMetaData.Spec.MetaData = &corev1.SecretReference{Name: "secret", Namespace: "foo"}

	sameNsNetworkData := valid.DeepCopy()
	sameNsNetworkData.Spec.NetworkData = &corev1.SecretReference{Name: "secret", Namespace: "foo"}

	noNsUserData := valid.DeepCopy()
	noNsUserData.Spec.UserData = &corev1.SecretReference{Name: "secret"}

	noNsMetaData := valid.DeepCopy()
	noNsMetaData.Spec.MetaData = &corev1.SecretReference{Name: "secret"}

	noNsNetworkData := valid.DeepCopy()
	noNsNetworkData.Spec.NetworkData = &corev1.SecretReference{Name: "secret"}

	tests := []struct {
		name      string
		expectErr bool
		c         *infrav1.Metal3Machine
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
		{
			name:      "should return error when both image and customDeploy are missing",
			expectErr: true,
			c:         neitherImageNorCustomDeploy,
		},
		{
			name:      "should return error when userData references a different namespace",
			expectErr: true,
			c:         crossNsUserData,
		},
		{
			name:      "should return error when metaData references a different namespace",
			expectErr: true,
			c:         crossNsMetaData,
		},
		{
			name:      "should return error when networkData references a different namespace",
			expectErr: true,
			c:         crossNsNetworkData,
		},
		{
			name:      "should succeed when userData references the same namespace",
			expectErr: false,
			c:         sameNsUserData,
		},
		{
			name:      "should succeed when metaData references the same namespace",
			expectErr: false,
			c:         sameNsMetaData,
		},
		{
			name:      "should succeed when networkData references the same namespace",
			expectErr: false,
			c:         sameNsNetworkData,
		},
		{
			name:      "should succeed when userData has no namespace",
			expectErr: false,
			c:         noNsUserData,
		},
		{
			name:      "should succeed when metaData has no namespace",
			expectErr: false,
			c:         noNsMetaData,
		},
		{
			name:      "should succeed when networkData has no namespace",
			expectErr: false,
			c:         noNsNetworkData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			webhook := &Metal3Machine{}

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
