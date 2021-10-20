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

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetal3DataTemplateDefault(t *testing.T) {
	g := NewWithT(t)

	c := &Metal3DataTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: Metal3DataTemplateSpec{},
	}
	c.Default()

	g.Expect(c.Spec).To(Equal(Metal3DataTemplateSpec{}))
	g.Expect(c.Status).To(Equal(Metal3DataTemplateStatus{}))
}

func TestMetal3DataTemplateValidation(t *testing.T) {

	tests := []struct {
		name      string
		expectErr bool
		c         *Metal3DataTemplate
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,
			c: &Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: Metal3DataTemplateSpec{},
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

func TestMetal3DataTemplateUpdateValidation(t *testing.T) {

	tests := []struct {
		name      string
		expectErr bool
		new       *Metal3DataTemplateSpec
		old       *Metal3DataTemplateSpec
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,
			new:       &Metal3DataTemplateSpec{},
			old:       &Metal3DataTemplateSpec{},
		},
		{
			name:      "should fail when old is nil",
			expectErr: true,
			new:       &Metal3DataTemplateSpec{},
			old:       nil,
		},
		{
			name:      "should fail when Metadata value changes",
			expectErr: true,
			new: &Metal3DataTemplateSpec{
				MetaData: &MetaData{
					Strings: []MetaDataString{{
						Key:   "abc",
						Value: "def",
					}},
				},
			},
			old: &Metal3DataTemplateSpec{
				MetaData: &MetaData{
					Strings: []MetaDataString{{
						Key:   "abc",
						Value: "defg",
					}},
				},
			},
		},
		{
			name:      "should fail when Metadata types changes",
			expectErr: true,
			new: &Metal3DataTemplateSpec{
				MetaData: &MetaData{
					Strings: []MetaDataString{{
						Key:   "abc",
						Value: "def",
					}},
				},
			},
			old: &Metal3DataTemplateSpec{
				MetaData: &MetaData{
					Namespaces: []MetaDataNamespace{{
						Key: "abc",
					}},
				},
			},
		},

		{
			name:      "should fail when Networkdata value changes",
			expectErr: true,
			new: &Metal3DataTemplateSpec{
				NetworkData: &NetworkData{
					Services: NetworkDataService{
						DNS: []ipamv1.IPAddressStr{
							"abc",
						},
					},
				},
			},
			old: &Metal3DataTemplateSpec{
				NetworkData: &NetworkData{
					Services: NetworkDataService{
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
			new: &Metal3DataTemplateSpec{
				NetworkData: &NetworkData{
					Services: NetworkDataService{
						DNS: []ipamv1.IPAddressStr{
							"abc",
						},
					},
				},
			},
			old: &Metal3DataTemplateSpec{
				NetworkData: &NetworkData{
					Networks: NetworkDataNetwork{
						IPv4DHCP: []NetworkDataIPv4DHCP{
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
			var new, old *Metal3DataTemplate
			g := NewWithT(t)
			new = &Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: *tt.new,
			}

			if tt.old != nil {
				old = &Metal3DataTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
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
