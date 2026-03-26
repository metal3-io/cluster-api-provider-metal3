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
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/randfill"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())

	t.Run("for Metal3Cluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3Cluster{},
		Spoke:       &Metal3Cluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3ClusterFuzzFuncs},
	}))
	t.Run("for Metal3ClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3ClusterTemplate{},
		Spoke:       &Metal3ClusterTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3ClusterTemplateFuzzFuncs},
	}))
	t.Run("for Metal3Machine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3Machine{},
		Spoke:       &Metal3Machine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3MachineFuzzFuncs},
	}))
	t.Run("for Metal3MachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3MachineTemplate{},
		Spoke:       &Metal3MachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3MachineTemplateFuzzFuncs},
	}))
	t.Run("for Metal3DataTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3DataTemplate{},
		Spoke:       &Metal3DataTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3DataTemplateFuzzFuncs},
	}))
	t.Run("for Metal3Data", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3Data{},
		Spoke:       &Metal3Data{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3DataTemplateFuzzFuncs},
	}))
	t.Run("for Metal3DataClaim", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3DataClaim{},
		Spoke:       &Metal3DataClaim{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for Metal3Remediation", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3Remediation{},
		Spoke:       &Metal3Remediation{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3RemediationFuzzFuncs},
	}))
	t.Run("for Metal3RemediationTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3RemediationTemplate{},
		Spoke:       &Metal3RemediationTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3RemediationTemplateFuzzFuncs},
	}))
}

func Metal3ClusterFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubMetal3ClusterStatus,
		hubMetal3FailureDomain,
		hubMetal3ClusterSpec,
		spokeMetal3ClusterSpec,
		spokeMetal3ClusterStatus,
	}
}

func hubMetal3ClusterStatus(in *infrav1.Metal3ClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta1 == nil || reflect.DeepEqual(in.Deprecated.V1Beta1, &infrav1.Metal3ClusterV1Beta1DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func hubMetal3FailureDomain(in *clusterv1.FailureDomain, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize ControlPlane: &false to nil (omitempty semantic)
	if in.ControlPlane != nil && !*in.ControlPlane {
		in.ControlPlane = nil
	}
}

func spokeMetal3ClusterStatus(in *Metal3ClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &Metal3ClusterV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}

func Metal3MachineFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubMetal3MachineStatus,
		spokeMetal3MachineSpec,
		spokeMetal3MachineStatus,
		spokeObjectMeta,
	}
}

func spokeObjectMeta(in *metav1.ObjectMeta, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize nil Annotations to empty map - MarshalData initializes the map
	// during hub->spoke conversion even if no data is stored.
	if in.Annotations == nil {
		in.Annotations = map[string]string{}
	}
}

func hubMetal3MachineStatus(in *infrav1.Metal3MachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.Deprecated != nil {
		if in.Deprecated.V1Beta1 == nil || reflect.DeepEqual(in.Deprecated.V1Beta1, &infrav1.Metal3MachineV1Beta1DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func spokeMetal3MachineSpec(in *Metal3MachineSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.ProviderID != nil && *in.ProviderID == "" {
		in.ProviderID = nil
	}

	// Normalize Image pointer fields: nil → &"" (since v1beta2 uses plain strings,
	// round-trip conversion cannot distinguish nil from &"")
	if in.Image.ChecksumType == nil {
		in.Image.ChecksumType = ptr.To("")
	}
	if in.Image.DiskFormat == nil {
		in.Image.DiskFormat = ptr.To("")
	}
}

func spokeMetal3MachineStatus(in *Metal3MachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if len(in.V1Beta2.Conditions) == 0 || reflect.DeepEqual(in.V1Beta2, &Metal3MachineV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
	in.Phase = "" // Phase is deprecated and it was never used in v1beta1, so we don't want to populate it during conversion.
}

func Metal3ClusterTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubMetal3FailureDomain,
		spokeMetal3ClusterSpec,
	}
}

func hubMetal3ClusterSpec(in *infrav1.Metal3ClusterSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize: &false → nil (omitempty semantic)
	if in.CloudProviderEnabled == nil {
		in.CloudProviderEnabled = ptr.To(true)
	}
}

func spokeMetal3ClusterSpec(in *Metal3ClusterSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize: &false → nil (omitempty semantic)
	if in.CloudProviderEnabled == nil && in.NoCloudProvider == nil {
		in.NoCloudProvider = ptr.To(true)
		in.CloudProviderEnabled = ptr.To(false)
	}
	if in.CloudProviderEnabled == nil && in.NoCloudProvider != nil {
		in.CloudProviderEnabled = ptr.To(!*in.NoCloudProvider)
	}
	if in.CloudProviderEnabled != nil && in.NoCloudProvider == nil {
		in.NoCloudProvider = ptr.To(!*in.CloudProviderEnabled)
	}
	if in.CloudProviderEnabled != nil && in.NoCloudProvider != nil {
		in.CloudProviderEnabled = ptr.To(!*in.NoCloudProvider)
	}
}

func spokeMetal3DataTemplateSpec(in *Metal3DataTemplateSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Clean TemplateReference
	in.TemplateReference = ""
}

func spokeMetal3DataTemplateStatus(in *Metal3DataTemplateStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// Clear invalid entries from Indexes map (entries without valid names are filtered during conversion)
	if in.Indexes != nil {
		validIndexes := make(map[string]int)
		for name, index := range in.Indexes {
			if name != "" {
				validIndexes[name] = index
			}
		}
		if len(validIndexes) > 0 {
			in.Indexes = validIndexes
		} else {
			in.Indexes = nil
		}
	}
}

func hubMetal3DataTemplateStatus(in *infrav1.Metal3DataTemplateStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// Clear invalid entries from Indexes list (entries without valid names are filtered during conversion)
	if in.Indexes != nil {
		validIndexes := make([]infrav1.IndexEntry, 0, len(in.Indexes))
		for _, entry := range in.Indexes {
			if entry.Name != "" && entry.Index != nil {
				validIndexes = append(validIndexes, entry)
			}
		}
		if len(validIndexes) > 0 {
			in.Indexes = validIndexes
		} else {
			in.Indexes = nil
		}
	}
}

func hubNetworkDataLinkVlan(in *infrav1.NetworkDataLinkVlan, c randfill.Continue) {
	c.FillNoCustom(in)
	// VlanID is required; if nil after fuzzing, set to 0 to match the round-trip conversion behavior.
	if in.VlanID == nil {
		in.VlanID = ptr.To(int32(0))
	}
}

func spokeMetal3DataSpec(in *Metal3DataSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Clean TemplateReference
	in.TemplateReference = ""
}

func Metal3DataTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		spokeMetal3DataSpec,
		spokeMetal3DataTemplateSpec,
		spokeTypedLocalObjectReference,
		spokeMetal3DataTemplateStatus,
		hubMetal3DataTemplateStatus,
		hubNetworkDataLinkVlan,
	}
}

func Metal3MachineTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []any {
	return []any{
		spokeMetal3MachineSpec,
		spokeObjectMeta,
		hubClusterv1ObjectMeta,
	}
}

// hubClusterv1ObjectMeta normalizes clusterv1.ObjectMeta for hub objects.
// Empty maps become nil after JSON marshal/unmarshal (omitempty).
func hubClusterv1ObjectMeta(in *clusterv1.ObjectMeta, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize empty map to nil - JSON round-trip with omitempty converts {} to nil.
	if len(in.Labels) == 0 {
		in.Labels = nil
	}
	if len(in.Annotations) == 0 {
		in.Annotations = nil
	}
}

func Metal3RemediationFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		spokeRemediationStrategy,
	}
}

func spokeRemediationStrategy(in *RemediationStrategy, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize Timeout to whole seconds only to match v1beta2 conversion precision.
	// v1beta1 uses *metav1.Duration (full nanosecond precision) but v1beta2 uses *int32
	// TimeoutSeconds (seconds only), so nanoseconds are lost during round-trip conversion.
	if in.Timeout != nil {
		in.Timeout = ptr.To(metav1.Duration{Duration: time.Duration(c.Int31()) * time.Second})
	}
}

func Metal3RemediationTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		spokeRemediationStrategy,
	}
}

func spokeTypedLocalObjectReference(in *corev1.TypedLocalObjectReference, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.APIGroup != nil && *in.APIGroup == "" {
		in.APIGroup = nil
	}
}
