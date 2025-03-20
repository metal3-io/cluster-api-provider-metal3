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

package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetal3ClusterTemplateDefault(t *testing.T) {
	g := NewWithT(t)

	c := &Metal3ClusterTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: Metal3ClusterTemplateSpec{},
	}

	g.Expect(c.Default(ctx, c)).To(Succeed())
	g.Expect(c.Spec).To(Equal(Metal3ClusterTemplateSpec{}))
}

func TestMetal3ClusterTemplateValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		c         *Metal3ClusterTemplate
	}{
		{
			name:      "should succeed when values and templates are correct",
			expectErr: false,
			c: &Metal3ClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: Metal3ClusterTemplateSpec{
					Template: Metal3ClusterTemplateResource{
						Spec: Metal3ClusterSpec{
							ControlPlaneEndpoint: APIEndpoint{
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
			c: &Metal3ClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: Metal3ClusterTemplateSpec{
					Template: Metal3ClusterTemplateResource{
						Spec: Metal3ClusterSpec{
							ControlPlaneEndpoint: APIEndpoint{},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			if tt.expectErr {
				_, err := tt.c.ValidateCreate(ctx, tt.c)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := tt.c.ValidateCreate(ctx, tt.c)
				g.Expect(err).NotTo(HaveOccurred())
			}
			_, err := tt.c.ValidateDelete(ctx, tt.c)
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestMetal3ClusterTemplateUpdateValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		new       *Metal3ClusterTemplateSpec
		old       *Metal3ClusterTemplateSpec
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,

			new: &Metal3ClusterTemplateSpec{
				Template: Metal3ClusterTemplateResource{
					Spec: Metal3ClusterSpec{
						ControlPlaneEndpoint: APIEndpoint{
							Host: "127.0.0.1",
							Port: 4242,
						},
					},
				},
			},

			old: &Metal3ClusterTemplateSpec{
				Template: Metal3ClusterTemplateResource{
					Spec: Metal3ClusterSpec{
						ControlPlaneEndpoint: APIEndpoint{
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

			new: &Metal3ClusterTemplateSpec{
				Template: Metal3ClusterTemplateResource{
					Spec: Metal3ClusterSpec{
						ControlPlaneEndpoint: APIEndpoint{
							Host: "127.0.0.1",
							Port: 8080,
						},
					},
				},
			},
			old: &Metal3ClusterTemplateSpec{},
		},
		{
			name:      "should fail if new template is invalid (e.g. missing host or port)",
			expectErr: true,

			new: &Metal3ClusterTemplateSpec{},
			old: &Metal3ClusterTemplateSpec{
				Template: Metal3ClusterTemplateResource{
					Spec: Metal3ClusterSpec{
						ControlPlaneEndpoint: APIEndpoint{
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
			var newCT, oldCT *Metal3ClusterTemplate
			g := NewWithT(t)

			newCT = &Metal3ClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: *tt.new,
			}

			if tt.old != nil {
				oldCT = &Metal3ClusterTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
					},
					Spec: *tt.old,
				}
			} else {
				oldCT = nil
			}

			if tt.expectErr {
				_, err := newCT.ValidateUpdate(ctx, oldCT, newCT)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := newCT.ValidateUpdate(ctx, oldCT, newCT)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
