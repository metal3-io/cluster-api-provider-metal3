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

package v1alpha2

import (
	"math/rand"
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/metal3-io/cluster-api-provider-baremetal/api/v1alpha3"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func apiEndpointFuzzerFuncs(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(i *v1alpha3.APIEndpoint, c fuzz.Continue) {
			b := make([]byte, seededRand.Intn(264))
			for i := range b {
				b[i] = charset[seededRand.Intn(len(charset))]
			}
			i.Host = string(b)
			i.Port = seededRand.Intn(65535)
		},
	}
}

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1alpha3.AddToScheme(scheme)).To(Succeed())

	t.Run("for BareMetalCluster", utilconversion.FuzzTestFunc(scheme, &v1alpha3.BareMetalCluster{}, &BareMetalCluster{}, apiEndpointFuzzerFuncs))
	t.Run("for BareMetalMachine", utilconversion.FuzzTestFunc(scheme, &v1alpha3.BareMetalMachine{}, &BareMetalMachine{}))
}

func TestConvertBareMetalCluster(t *testing.T) {
	g := NewWithT(t)

	t.Run("to hub", func(t *testing.T) {
		t.Run("should convert the first value in Status.APIEndpoints to Spec.ControlPlaneEndpoint", func(t *testing.T) {
			src := &BareMetalCluster{
				Spec: BareMetalClusterSpec{
					APIEndpoint: "https://example.com:6443",
				},
				Status: BareMetalClusterStatus{
					APIEndpoints: []APIEndpoint{
						{
							Host: "example.com",
							Port: 6443,
						},
					},
				},
			}
			dst := &v1alpha3.BareMetalCluster{}

			g.Expect(src.ConvertTo(dst)).To(Succeed())
			g.Expect(dst.Spec.ControlPlaneEndpoint.Host).To(Equal("example.com"))
			g.Expect(dst.Spec.ControlPlaneEndpoint.Port).To(BeEquivalentTo(6443))
		})
	})

	t.Run("from hub", func(t *testing.T) {
		t.Run("preserves fields from hub version", func(t *testing.T) {
			src := &v1alpha3.BareMetalCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "hub",
				},
				Spec: v1alpha3.BareMetalClusterSpec{
					ControlPlaneEndpoint: v1alpha3.APIEndpoint{
						Host: "example.com",
						Port: 6443,
					},
				},
				Status: v1alpha3.BareMetalClusterStatus{},
			}
			dst := &BareMetalCluster{}

			g.Expect(dst.ConvertFrom(src)).To(Succeed())
			restored := &v1alpha3.BareMetalCluster{}
			g.Expect(dst.ConvertTo(restored)).To(Succeed())

			// Test field restored fields.
			g.Expect(restored.Name).To(Equal(src.Name))
			g.Expect(restored.Spec.ControlPlaneEndpoint.Host).To(Equal(src.Spec.ControlPlaneEndpoint.Host))
			g.Expect(restored.Spec.ControlPlaneEndpoint.Port).To(Equal(src.Spec.ControlPlaneEndpoint.Port))
		})

		t.Run("should convert Spec.ControlPlaneEndpoint to Status.APIEndpoints[0]", func(t *testing.T) {
			src := &v1alpha3.BareMetalCluster{
				Spec: v1alpha3.BareMetalClusterSpec{
					ControlPlaneEndpoint: v1alpha3.APIEndpoint{
						Host: "example.com",
						Port: 6443,
					},
				},
			}
			dst := &BareMetalCluster{}

			g.Expect(dst.ConvertFrom(src)).To(Succeed())
			g.Expect(dst.Status.APIEndpoints[0].Host).To(Equal("example.com"))
			g.Expect(dst.Status.APIEndpoints[0].Port).To(BeEquivalentTo(6443))
		})
	})
}

// BareMetalMachine does not need specific testing aside of fuzzing for now,
// since no changes other than ErrorReason and ErrorMessage renaming were done.
// The fuzzing verifies those.
