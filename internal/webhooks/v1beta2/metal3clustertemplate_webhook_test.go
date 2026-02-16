/*
Copyright 2024 The Kubernetes Authors.
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
)

func TestMetal3ClusterTemplateValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		c         *infrav1.Metal3ClusterTemplate
	}{
		{
			name:      "should succeed when values and templates are correct",
			expectErr: false,
			c: &infrav1.Metal3ClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3ClusterTemplateSpec{
					Template: infrav1.Metal3ClusterTemplateResource{
						Spec: infrav1.Metal3ClusterSpec{
							ControlPlaneEndpoint: infrav1.APIEndpoint{
								Host: "127.0.0.1",
								Port: 4242,
							},
						},
					},
				},
			},
		},
		{
			name:      "should fail when values and templates are incorrect",
			expectErr: true,
			c: &infrav1.Metal3ClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3ClusterTemplateSpec{
					Template: infrav1.Metal3ClusterTemplateResource{
						Spec: infrav1.Metal3ClusterSpec{
							ControlPlaneEndpoint: infrav1.APIEndpoint{},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			webhook := &Metal3ClusterTemplate{}

			if tt.expectErr {
				_, err := webhook.ValidateCreate(ctx, tt.c)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateCreate(ctx, tt.c)
				g.Expect(err).NotTo(HaveOccurred())
			}
			_, err := webhook.ValidateDelete(ctx, tt.c)
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestMetal3ClusterTemplateUpdateValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		new       *infrav1.Metal3ClusterTemplateSpec
		old       *infrav1.Metal3ClusterTemplateSpec
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,

			new: &infrav1.Metal3ClusterTemplateSpec{
				Template: infrav1.Metal3ClusterTemplateResource{
					Spec: infrav1.Metal3ClusterSpec{
						ControlPlaneEndpoint: infrav1.APIEndpoint{
							Host: "127.0.0.1",
							Port: 4242,
						},
					},
				},
			},

			old: &infrav1.Metal3ClusterTemplateSpec{
				Template: infrav1.Metal3ClusterTemplateResource{
					Spec: infrav1.Metal3ClusterSpec{
						ControlPlaneEndpoint: infrav1.APIEndpoint{
							Host: "127.0.0.1",
							Port: 8080,
						},
					},
				},
			},
		},
		{
			name:      "should fail if old template is invalid (e.g. missing host or port)",
			expectErr: true,

			new: &infrav1.Metal3ClusterTemplateSpec{
				Template: infrav1.Metal3ClusterTemplateResource{
					Spec: infrav1.Metal3ClusterSpec{
						ControlPlaneEndpoint: infrav1.APIEndpoint{
							Host: "127.0.0.1",
							Port: 8080,
						},
					},
				},
			},
			old: &infrav1.Metal3ClusterTemplateSpec{},
		},
		{
			name:      "should fail if new template is invalid (e.g. missing host or port)",
			expectErr: true,

			new: &infrav1.Metal3ClusterTemplateSpec{},
			old: &infrav1.Metal3ClusterTemplateSpec{
				Template: infrav1.Metal3ClusterTemplateResource{
					Spec: infrav1.Metal3ClusterSpec{
						ControlPlaneEndpoint: infrav1.APIEndpoint{
							Host: "127.0.0.1",
							Port: 8080,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var newCT, oldCT *infrav1.Metal3ClusterTemplate
			g := NewWithT(t)
			webhook := &Metal3ClusterTemplate{}

			newCT = &infrav1.Metal3ClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: *tt.new,
			}

			if tt.old != nil {
				oldCT = &infrav1.Metal3ClusterTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
					},
					Spec: *tt.old,
				}
			} else {
				oldCT = nil
			}

			if tt.expectErr {
				_, err := webhook.ValidateUpdate(ctx, oldCT, newCT)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateUpdate(ctx, oldCT, newCT)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
