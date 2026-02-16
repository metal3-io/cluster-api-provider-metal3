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
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetal3DataTemplateValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		c         *infrav1.Metal3DataTemplate
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,
			c: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3DataTemplateSpec{},
			},
		},
		{
			name:      "should succeed with fromPoolAnnotation in IPv4",
			expectErr: false,
			c: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3DataTemplateSpec{
					NetworkData: &infrav1.NetworkData{
						Networks: infrav1.NetworkDataNetwork{
							IPv4: []infrav1.NetworkDataIPv4{
								{
									ID:   "test",
									Link: "eth0",
									FromPoolAnnotation: &infrav1.FromPoolAnnotation{
										Object:     "baremetalhost",
										Annotation: "ippool.metal3.io/provisioning",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "should succeed with fromPoolAnnotation in IPv6",
			expectErr: false,
			c: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3DataTemplateSpec{
					NetworkData: &infrav1.NetworkData{
						Networks: infrav1.NetworkDataNetwork{
							IPv6: []infrav1.NetworkDataIPv6{
								{
									ID:   "test",
									Link: "eth0",
									FromPoolAnnotation: &infrav1.FromPoolAnnotation{
										Object:     "machine",
										Annotation: "ippool.metal3.io/provisioning",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "should fail when IPv4 network has no pool reference",
			expectErr: true,
			c: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3DataTemplateSpec{
					NetworkData: &infrav1.NetworkData{
						Networks: infrav1.NetworkDataNetwork{
							IPv4: []infrav1.NetworkDataIPv4{
								{
									ID:   "test",
									Link: "eth0",
									// No FromPoolRef, FromPoolAnnotation, or IPAddressFromIPPool
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "should fail when IPv6 network has no pool reference",
			expectErr: true,
			c: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3DataTemplateSpec{
					NetworkData: &infrav1.NetworkData{
						Networks: infrav1.NetworkDataNetwork{
							IPv6: []infrav1.NetworkDataIPv6{
								{
									ID:   "test",
									Link: "eth0",
									// No FromPoolRef, FromPoolAnnotation, or IPAddressFromIPPool
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			webhook := &Metal3DataTemplate{}

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

func TestMetal3DataTemplateUpdateValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		new       *infrav1.Metal3DataTemplateSpec
		old       *infrav1.Metal3DataTemplateSpec
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,
			new:       &infrav1.Metal3DataTemplateSpec{},
			old:       &infrav1.Metal3DataTemplateSpec{},
		},
		{
			name:      "should fail when old is nil",
			expectErr: true,
			new:       &infrav1.Metal3DataTemplateSpec{},
			old:       nil,
		},
		{
			name:      "should fail when Metadata value changes",
			expectErr: true,
			new: &infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{
					Strings: []infrav1.MetaDataString{{
						Key:   "abc",
						Value: "def",
					}},
				},
			},
			old: &infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{
					Strings: []infrav1.MetaDataString{{
						Key:   "abc",
						Value: "defg",
					}},
				},
			},
		},
		{
			name:      "should fail when Metadata types changes",
			expectErr: true,
			new: &infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{
					Strings: []infrav1.MetaDataString{{
						Key:   "abc",
						Value: "def",
					}},
				},
			},
			old: &infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{
					Namespaces: []infrav1.MetaDataNamespace{{
						Key: "abc",
					}},
				},
			},
		},

		{
			name:      "should fail when Networkdata value changes",
			expectErr: true,
			new: &infrav1.Metal3DataTemplateSpec{
				NetworkData: &infrav1.NetworkData{
					Services: infrav1.NetworkDataService{
						DNS: []ipamv1.IPAddressStr{
							"abc",
						},
					},
				},
			},
			old: &infrav1.Metal3DataTemplateSpec{
				NetworkData: &infrav1.NetworkData{
					Services: infrav1.NetworkDataService{
						DNS: []ipamv1.IPAddressStr{
							"abcd",
						},
					},
				},
			},
		},
		{
			name:      "should fail when Networkdata type changes",
			expectErr: true,
			new: &infrav1.Metal3DataTemplateSpec{
				NetworkData: &infrav1.NetworkData{
					Services: infrav1.NetworkDataService{
						DNS: []ipamv1.IPAddressStr{
							"abc",
						},
					},
				},
			},
			old: &infrav1.Metal3DataTemplateSpec{
				NetworkData: &infrav1.NetworkData{
					Networks: infrav1.NetworkDataNetwork{
						IPv4DHCP: []infrav1.NetworkDataIPv4DHCP{
							{
								ID:   "abc",
								Link: "abc",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var newDT, oldDT *infrav1.Metal3DataTemplate
			g := NewWithT(t)
			webhook := &Metal3DataTemplate{}

			newDT = &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: *tt.new,
			}

			if tt.old != nil {
				oldDT = &infrav1.Metal3DataTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
					},
					Spec: *tt.old,
				}
			} else {
				oldDT = nil
			}

			if tt.expectErr {
				_, err := webhook.ValidateUpdate(ctx, oldDT, newDT)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateUpdate(ctx, oldDT, newDT)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
