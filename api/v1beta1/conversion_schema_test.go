/*
Copyright 2025 The Kubernetes Authors.

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
	"context"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
)

var (
	hubClientOnce sync.Once
	hubClient     client.Client
	hubClientErr  error
)

// clientWithBothVersions returns a client that knows both API versions.
//
// The package-level client in TestMain deliberately registers only v1beta1 in
// the global scheme. envtest patches a CRD with a conversion webhook whenever
// the scheme makes its kind convertible, and no webhook is served here, so
// adding v1beta2 to that scheme would break every v1beta1 write in this
// package. Building a separate client after the environment has started gives
// access to v1beta2 without changing how the CRDs were installed.
func clientWithBothVersions(g *WithT) client.Client {
	hubClientOnce.Do(func() {
		s := runtime.NewScheme()
		if hubClientErr = AddToScheme(s); hubClientErr != nil {
			return
		}
		if hubClientErr = infrav1.AddToScheme(s); hubClientErr != nil {
			return
		}
		hubClient, hubClientErr = client.New(cfg, client.Options{Scheme: s})
	})
	g.Expect(hubClientErr).ToNot(HaveOccurred())
	return hubClient
}

// These tests write conversion output through the API server so that the real
// CRD schema validates the serialized form. The in-memory round-trip in
// TestFuzzyConversion cannot catch fields lost to json omitempty or to
// structural-schema pruning, because both happen during serialization.
// See https://github.com/metal3-io/cluster-api-provider-metal3/issues/3808.

// TestHostSelectorEmptyValuesAcceptedByV1Beta2Schema covers the case reported in
// issue 3808. A v1beta1 host selector using an operator that takes no values must
// stay writable once converted to v1beta2, both on create and on a later update.
func TestHostSelectorEmptyValuesAcceptedByV1Beta2Schema(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	cl := clientWithBothVersions(g)

	spoke := &Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{Name: "m3m-empty-values", Namespace: "default"},
		Spec: Metal3MachineSpec{
			Image: Image{
				URL:      "http://example.org/image.qcow2",
				Checksum: "http://example.org/image.qcow2.sha256",
			},
			HostSelector: HostSelector{
				MatchExpressions: []HostSelectorRequirement{
					{
						Key:      "example.org/available",
						Operator: selection.Exists,
						Values:   []string{},
					},
				},
			},
		},
	}

	hub := &infrav1.Metal3Machine{}
	g.Expect(spoke.ConvertTo(hub)).To(Succeed())
	g.Expect(hub.Spec.HostSelector).ToNot(BeNil())
	g.Expect(hub.Spec.HostSelector.MatchExpressions).To(HaveLen(1))

	// Create exercises the v1beta2 schema against the serialized conversion
	// output. Before the fix, omitempty dropped the empty values list and the
	// API server rejected this with "matchExpressions[*].values: Required value".
	g.Expect(cl.Create(ctx, hub)).To(Succeed())
	t.Cleanup(func() { _ = cl.Delete(ctx, hub) })

	// Read it back as v1beta2 and submit it again, which is the update path the
	// issue describes.
	fetched := &infrav1.Metal3Machine{}
	g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(hub), fetched)).To(Succeed())
	g.Expect(fetched.Spec.HostSelector.MatchExpressions).To(HaveLen(1))
	g.Expect(fetched.Spec.HostSelector.MatchExpressions[0].Operator).To(Equal(selection.Exists))
	g.Expect(fetched.Spec.HostSelector.MatchExpressions[0].Values).To(BeEmpty())

	fetched.Spec.AutomatedCleaningMode = "disabled"
	g.Expect(cl.Update(ctx, fetched)).To(Succeed())
}

// TestHostSelectorNilValuesAcceptedByV1Beta1Schema guards the reverse direction.
// v1beta1 still requires values to be present, and a nil slice would marshal to
// null and be pruned, so conversion down must emit an empty list.
func TestHostSelectorNilValuesAcceptedByV1Beta1Schema(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	cl := clientWithBothVersions(g)

	hub := &infrav1.Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{Name: "m3m-nil-values", Namespace: "default"},
		Spec: infrav1.Metal3MachineSpec{
			// checksumType and diskFormat are set because the downward Image
			// conversion always emits a non-nil pointer, so empty values would
			// fail the v1beta1 enum. That is a separate pre-existing issue and
			// is deliberately not under test here.
			Image: infrav1.Image{
				URL:          "http://example.org/image.qcow2",
				Checksum:     ptrTo("http://example.org/image.qcow2.sha256"),
				ChecksumType: "sha256",
				DiskFormat:   "qcow2",
			},
			HostSelector: &infrav1.HostSelector{
				MatchExpressions: []infrav1.HostSelectorRequirement{
					{
						Key:      "example.org/available",
						Operator: selection.Exists,
						Values:   nil,
					},
				},
			},
		},
	}

	spoke := &Metal3Machine{}
	g.Expect(spoke.ConvertFrom(hub)).To(Succeed())
	g.Expect(spoke.Spec.HostSelector.MatchExpressions).To(HaveLen(1))
	g.Expect(spoke.Spec.HostSelector.MatchExpressions[0].Values).ToNot(BeNil(),
		"nil values must be normalized to an empty slice, since v1beta1 requires the field to be present")

	g.Expect(cl.Create(ctx, spoke)).To(Succeed())
	t.Cleanup(func() { _ = cl.Delete(ctx, spoke) })
}

// TestBondWithoutParametersAcceptedByV1Beta2Schema covers the same class of break
// on Metal3DataTemplate. Bond parameters were optional in v1beta1, so a bond link
// that sets none must stay writable when converted to v1beta2.
func TestBondWithoutParametersAcceptedByV1Beta2Schema(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	cl := clientWithBothVersions(g)

	spoke := &Metal3DataTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "m3dt-no-bond-params", Namespace: "default"},
		Spec: Metal3DataTemplateSpec{
			NetworkData: &NetworkData{
				Links: NetworkDataLink{
					Bonds: []NetworkDataLinkBond{
						{
							Id:       "bond0",
							BondMode: "802.3ad",
							MACAddress: &NetworkLinkEthernetMac{
								String: ptrTo("00:11:22:33:44:55"),
							},
							BondLinks: []string{"eth0", "eth1"},
						},
					},
				},
			},
		},
	}

	hub := &infrav1.Metal3DataTemplate{}
	g.Expect(spoke.ConvertTo(hub)).To(Succeed())
	g.Expect(hub.Spec.NetworkData.Links.Bonds).To(HaveLen(1))
	g.Expect(hub.Spec.NetworkData.Links.Bonds[0].Parameters).To(BeEmpty())

	// Before the fix, parameters was required with minItems=1 in v1beta2, so this
	// was rejected even though v1beta1 allowed a bond link with no parameters.
	g.Expect(cl.Create(ctx, hub)).To(Succeed())
	t.Cleanup(func() { _ = cl.Delete(ctx, hub) })
}

func ptrTo[T any](v T) *T {
	return &v
}
