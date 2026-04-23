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
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3DataFuzzFuncs},
	}))
	t.Run("for Metal3DataClaim", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3DataClaim{},
		Spoke:       &Metal3DataClaim{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3DataClaimFuzzFuncs},
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
		hubMetal3MachineSpec,
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

// hubMetal3MachineSpec normalizes v1beta2 Metal3MachineSpec for hub objects.
func hubMetal3MachineSpec(in *infrav1.Metal3MachineSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize nil HostSelector to nil if empty
	if in.HostSelector != nil && len(in.HostSelector.MatchLabels) == 0 && len(in.HostSelector.MatchExpressions) == 0 {
		in.HostSelector = nil
	}

	// Normalize empty DataTemplate to nil
	if in.DataTemplate != nil && in.DataTemplate.Name == "" && in.DataTemplate.Namespace == "" {
		in.DataTemplate = nil
	}
}

// hubMetal3MachineTemplateSpec normalizes v1beta2 Metal3MachineTemplateSpec for hub objects.
func hubMetal3MachineTemplateSpec(in *infrav1.Metal3MachineTemplateSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize NodeReuse: &false → nil (omitempty semantic)
	if in.NodeReuse != nil && !*in.NodeReuse {
		in.NodeReuse = nil
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

	if in.DataTemplate != nil {
		in.DataTemplate = &corev1.ObjectReference{
			APIVersion: "",
			Kind:       "",
			Name:       in.DataTemplate.Name,
			Namespace:  in.DataTemplate.Namespace,
		}
	} else {
		in.DataTemplate = nil
	}
	// Normalize Image pointer fields: nil → &"" (since v1beta2 uses plain strings,
	// round-trip conversion cannot distinguish nil from &"")
	if in.Image.ChecksumType == nil {
		in.Image.ChecksumType = ptr.To("")
	}
	if in.Image.DiskFormat == nil {
		in.Image.DiskFormat = ptr.To("")
	}

	// Normalize MatchLabels: empty map → nil (JSON omitempty behavior)
	if len(in.HostSelector.MatchLabels) == 0 {
		in.HostSelector.MatchLabels = nil
	}

	// Normalize CustomDeploy: empty Method → nil
	if in.CustomDeploy != nil && in.CustomDeploy.Method == "" {
		in.CustomDeploy = nil
	}

	// Normalize AutomatedCleaningMode: empty string → nil
	if in.AutomatedCleaningMode != nil && *in.AutomatedCleaningMode == "" {
		in.AutomatedCleaningMode = nil
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

	if in.RenderedData != nil {
		in.RenderedData = &corev1.ObjectReference{
			APIVersion: "",
			Kind:       "",
			Name:       in.RenderedData.Name,
			Namespace:  in.RenderedData.Namespace,
		}
	} else {
		in.RenderedData = nil
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

func Metal3DataFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		spokeMetal3DataSpec,
		spokeMetal3DataStatus,
		hubMetal3DataSpec,
		hubMetal3DataStatus,
	}
}

func hubMetal3DataSpec(in *infrav1.Metal3DataSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize empty pointer fields to nil - empty Metal3ObjectRef{} should become nil
	// This matches the JSON round-trip behavior with omitempty tags
	if in.Claim != nil && in.Claim.Name == "" && in.Claim.Namespace == "" {
		in.Claim = nil
	}
	if in.Template != nil && in.Template.Name == "" && in.Template.Namespace == "" {
		in.Template = nil
	}
	// Normalize Index: nil → &0 (v1beta1 int cannot represent nil, so 0 and nil are equivalent)
	if in.Index == nil {
		in.Index = ptr.To[int32](0)
	}
}

// spokeMetal3DataStatus normalizes v1beta1 Metal3DataStatus for spoke objects.
func spokeMetal3DataStatus(in *Metal3DataStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize ErrorMessage: nil → &"" (v1beta2 uses string, empty converts back to &"")
	if in.ErrorMessage == nil {
		in.ErrorMessage = ptr.To("")
	}
}

// hubMetal3DataStatus normalizes v1beta2 Metal3DataStatus for hub objects.
func hubMetal3DataStatus(in *infrav1.Metal3DataStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize Ready: nil → &false (v1beta1 uses bool, false converts back to &false)
	if in.Ready == nil {
		in.Ready = ptr.To(false)
	}
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

func hubNetworkData(in *infrav1.NetworkData, c randfill.Continue) {
	c.FillNoCustom(in)
	// Links/Networks/Services are pointers in v1beta2 but values in v1beta1.
	// Empty structs can't round-trip through value types; normalize to nil.
	if in.Links != nil && reflect.DeepEqual(*in.Links, infrav1.NetworkDataLink{}) {
		in.Links = nil
	}
	if in.Networks != nil && reflect.DeepEqual(*in.Networks, infrav1.NetworkDataNetwork{}) {
		in.Networks = nil
	}
	if in.Services != nil && reflect.DeepEqual(*in.Services, infrav1.NetworkDataService{}) {
		in.Services = nil
	}
}

// hubMetaDataHostInterface normalizes FromBootMAC: *bool in v1beta2 vs bool in v1beta1.
// nil and &false both become false in spoke, then &false in hub; normalize nil → &false.
func hubMetaDataHostInterface(in *infrav1.MetaDataHostInterface, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.FromBootMAC == nil {
		in.FromBootMAC = ptr.To(false)
	}
}

func hubNetworkDataLinkVlan(in *infrav1.NetworkDataLinkVlan, c randfill.Continue) {
	c.FillNoCustom(in)
	// VlanID is required; if nil after fuzzing, set to 0 to match the round-trip conversion behavior.
	if in.VlanID == nil {
		in.VlanID = ptr.To(int32(0))
	}
}

// hubNetworkDataRoutev4 normalizes fields that can't round-trip through v1beta1 value types.
func hubNetworkDataRoutev4(in *infrav1.NetworkDataRoutev4, c randfill.Continue) {
	c.FillNoCustom(in)
	// Prefix: *int32 in v1beta2 vs int in v1beta1
	if in.Prefix != nil && *in.Prefix == 0 {
		in.Prefix = nil
	}
	// Services: *NetworkDataServicev4 in v1beta2 vs NetworkDataServicev4 in v1beta1
	if in.Services != nil && reflect.DeepEqual(*in.Services, infrav1.NetworkDataServicev4{}) {
		in.Services = nil
	}
}

// hubNetworkDataRoutev6 normalizes fields that can't round-trip through v1beta1 value types.
func hubNetworkDataRoutev6(in *infrav1.NetworkDataRoutev6, c randfill.Continue) {
	c.FillNoCustom(in)
	// Prefix: *int32 in v1beta2 vs int in v1beta1
	if in.Prefix != nil && *in.Prefix == 0 {
		in.Prefix = nil
	}
	// Services: *NetworkDataServicev6 in v1beta2 vs NetworkDataServicev6 in v1beta1
	if in.Services != nil && reflect.DeepEqual(*in.Services, infrav1.NetworkDataServicev6{}) {
		in.Services = nil
	}
}

func spokeMetal3DataSpec(in *Metal3DataSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Clean TemplateReference
	in.TemplateReference = ""

	if in.Claim.Name != "" || in.Claim.Namespace != "" {
		in.Claim = corev1.ObjectReference{
			APIVersion: "",
			Kind:       "",
			Name:       in.Claim.Name,
			Namespace:  in.Claim.Namespace,
		}
	} else {
		in.Claim = corev1.ObjectReference{}
	}

	if in.Template.Name != "" || in.Template.Namespace != "" {
		in.Template = corev1.ObjectReference{
			APIVersion: "",
			Kind:       "",
			Name:       in.Template.Name,
			Namespace:  in.Template.Namespace,
		}
	} else {
		in.Template = corev1.ObjectReference{}
	}
}

func Metal3DataClaimFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		spokeMetal3DataClaimSpec,
		spokeMetal3DataClaimStatus,
		hubMetal3DataClaimSpec,
	}
}

func hubMetal3DataClaimSpec(in *infrav1.Metal3DataClaimSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize empty pointer fields to nil - empty Metal3ObjectRef{} should become nil
	// This matches the JSON round-trip behavior with omitempty tags
	if in.Template != nil && in.Template.Name == "" && in.Template.Namespace == "" {
		in.Template = nil
	}
}

func spokeMetal3DataClaimSpec(in *Metal3DataClaimSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.Template.Name != "" || in.Template.Namespace != "" {
		in.Template = corev1.ObjectReference{
			APIVersion: "",
			Kind:       "",
			Name:       in.Template.Name,
			Namespace:  in.Template.Namespace,
		}
	} else {
		in.Template = corev1.ObjectReference{}
	}
}

func spokeMetal3DataClaimStatus(in *Metal3DataClaimStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.RenderedData != nil && (in.RenderedData.Name != "" || in.RenderedData.Namespace != "") {
		in.RenderedData = &corev1.ObjectReference{
			APIVersion: "",
			Kind:       "",
			Name:       in.RenderedData.Name,
			Namespace:  in.RenderedData.Namespace,
		}
	} else {
		in.RenderedData = nil
	}
	// Normalize ErrorMessage: nil → &"" (since v1beta2 uses string type,
	// round-trip conversion cannot distinguish nil from &"")
	if in.ErrorMessage == nil {
		in.ErrorMessage = ptr.To("")
	}
}

func Metal3DataTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		spokeMetal3DataTemplateSpec,
		spokeTypedLocalObjectReference,
		spokeMetal3DataTemplateStatus,
		spokeNetworkLinkEthernetMac,
		spokeNetworkDataIPv4,
		spokeNetworkDataIPv6,
		spokeNetworkGatewayv4,
		spokeNetworkGatewayv6,
		spokeNetworkDataService,
		spokeNetworkDataServicev4,
		spokeNetworkDataServicev6,
		hubMetal3DataTemplateStatus,
		hubMetaDataHostInterface,
		hubNetworkData,
		hubNetworkDataLinkVlan,
		hubNetworkDataRoutev4,
		hubNetworkDataRoutev6,
	}
}

func spokeMetal3DataTemplateSpec(in *Metal3DataTemplateSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Clean TemplateReference
	in.TemplateReference = ""
}

// spokeNetworkLinkEthernetMac normalizes MacAddress fields that can't round-trip properly.
// String and FromHostInterface pointer fields cannot distinguish nil from &"" through v1beta2 string type.
// FromAnnotation empty pointer can't round-trip through the v1beta2 value type.
func spokeNetworkLinkEthernetMac(in *NetworkLinkEthernetMac, c randfill.Continue) {
	c.FillNoCustom(in)
	// Normalize nil pointer fields: nil → &"" (since v1beta2 uses plain strings,
	// round-trip conversion cannot distinguish nil from &"")
	if in.String == nil {
		in.String = ptr.To("")
	}
	if in.FromHostInterface == nil {
		in.FromHostInterface = ptr.To("")
	}
	if in.FromAnnotation != nil && in.FromAnnotation.Object == "" && in.FromAnnotation.Annotation == "" {
		in.FromAnnotation = nil
	}
}

// spokeNetworkDataIPv4 normalizes NetworkDataIPv4 fields that can't round-trip.
func spokeNetworkDataIPv4(in *NetworkDataIPv4, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.FromPoolAnnotation != nil && in.FromPoolAnnotation.Object == "" && in.FromPoolAnnotation.Annotation == "" {
		in.FromPoolAnnotation = nil
	}
}

// spokeNetworkDataIPv6 normalizes NetworkDataIPv6 fields that can't round-trip.
func spokeNetworkDataIPv6(in *NetworkDataIPv6, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.FromPoolAnnotation != nil && in.FromPoolAnnotation.Object == "" && in.FromPoolAnnotation.Annotation == "" {
		in.FromPoolAnnotation = nil
	}
}

// spokeNetworkGatewayv4 normalizes NetworkGatewayv4 fields that can't round-trip.
// FromIPPool is a *string in v1beta1 but a string in v1beta2, so empty pointers (&"")
// cannot be preserved through round-trip conversion. Normalize &"" to nil.
func spokeNetworkGatewayv4(in *NetworkGatewayv4, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.FromPoolAnnotation != nil && in.FromPoolAnnotation.Object == "" && in.FromPoolAnnotation.Annotation == "" {
		in.FromPoolAnnotation = nil
	}
	if in.FromIPPool != nil && *in.FromIPPool == "" {
		in.FromIPPool = nil
	}
}

// spokeNetworkGatewayv6 normalizes NetworkGatewayv6 fields that can't round-trip.
// FromIPPool is a *string in v1beta1 but a string in v1beta2, so empty pointers (&"")
// cannot be preserved through round-trip conversion. Normalize &"" to nil.
func spokeNetworkGatewayv6(in *NetworkGatewayv6, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.FromPoolAnnotation != nil && in.FromPoolAnnotation.Object == "" && in.FromPoolAnnotation.Annotation == "" {
		in.FromPoolAnnotation = nil
	}
	if in.FromIPPool != nil && *in.FromIPPool == "" {
		in.FromIPPool = nil
	}
}

// spokeNetworkDataService normalizes NetworkDataService fields that can't round-trip.
// DNSFromIPPool is a *string in v1beta1 but a string in v1beta2, so empty pointers (&"")
// cannot be preserved through round-trip conversion. Normalize &"" to nil.
func spokeNetworkDataService(in *NetworkDataService, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.DNSFromIPPool != nil && *in.DNSFromIPPool == "" {
		in.DNSFromIPPool = nil
	}
}

// spokeNetworkDataServicev4 normalizes NetworkDataServicev4 fields that can't round-trip.
// DNSFromIPPool is a *string in v1beta1 but a string in v1beta2, so empty pointers (&"")
// cannot be preserved through round-trip conversion. Normalize &"" to nil.
func spokeNetworkDataServicev4(in *NetworkDataServicev4, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.DNSFromIPPool != nil && *in.DNSFromIPPool == "" {
		in.DNSFromIPPool = nil
	}
}

// spokeNetworkDataServicev6 normalizes NetworkDataServicev6 fields that can't round-trip.
// DNSFromIPPool is a *string in v1beta1 but a string in v1beta2, so empty pointers (&"")
// cannot be preserved through round-trip conversion. Normalize &"" to nil.
func spokeNetworkDataServicev6(in *NetworkDataServicev6, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.DNSFromIPPool != nil && *in.DNSFromIPPool == "" {
		in.DNSFromIPPool = nil
	}
}

func Metal3MachineTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []any {
	return []any{
		spokeMetal3MachineSpec,
		spokeObjectMeta,
		hubClusterv1ObjectMeta,
		hubMetal3MachineSpec,
		hubMetal3MachineTemplateSpec,
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
	// Normalize: &0s → nil (TimeoutSeconds 0 in v1beta2 converts back to nil)
	if in.Timeout != nil && in.Timeout.Duration == 0 {
		in.Timeout = nil
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
