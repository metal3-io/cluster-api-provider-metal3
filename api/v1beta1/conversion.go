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
	"maps"
	"reflect"
	"slices"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

func (src *Metal3Cluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3Cluster)
	if err := Convert_v1beta1_Metal3Cluster_To_v1beta2_Metal3Cluster(src, dst, nil); err != nil {
		return err
	}
	restored := &infrav1.Metal3Cluster{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	// Recover intent for bool values converted to *bool.
	initialization := infrav1.Metal3ClusterInitializationStatus{}
	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Initialization.Provisioned, &initialization.Provisioned)
	if !reflect.DeepEqual(initialization, infrav1.Metal3ClusterInitializationStatus{}) {
		dst.Status.Initialization = initialization
	}
	return nil
}

func (dst *Metal3Cluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3Cluster)
	if err := Convert_v1beta2_Metal3Cluster_To_v1beta1_Metal3Cluster(src, dst, nil); err != nil {
		return err
	}

	return utilconversion.MarshalData(src, dst)
}

func (src *Metal3ClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3ClusterTemplate)
	if err := Convert_v1beta1_Metal3ClusterTemplate_To_v1beta2_Metal3ClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	restored := &infrav1.Metal3ClusterTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.Template.ObjectMeta = restored.Spec.Template.ObjectMeta
	return nil
}

func (dst *Metal3ClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3ClusterTemplate)
	if err := Convert_v1beta2_Metal3ClusterTemplate_To_v1beta1_Metal3ClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	return utilconversion.MarshalData(src, dst)
}

func (src *Metal3Machine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3Machine)
	if err := Convert_v1beta1_Metal3Machine_To_v1beta2_Metal3Machine(src, dst, nil); err != nil {
		return err
	}

	restored := &infrav1.Metal3Machine{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	// Recover intent for bool values converted to *bool.
	initialization := infrav1.Metal3MachineInitializationStatus{}
	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Initialization.Provisioned, &initialization.Provisioned)
	if !reflect.DeepEqual(initialization, infrav1.Metal3MachineInitializationStatus{}) {
		dst.Status.Initialization = initialization
	}

	// Restore Image.Checksum nil state from hub data (v1beta2 uses *string, can be nil)
	if ok && restored.Spec.Image.Checksum == nil {
		dst.Spec.Image.Checksum = nil
	}
	return nil
}

func (dst *Metal3Machine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3Machine)
	if err := Convert_v1beta2_Metal3Machine_To_v1beta1_Metal3Machine(src, dst, nil); err != nil {
		return err
	}

	if dst.Spec.ProviderID != nil && *dst.Spec.ProviderID == "" {
		dst.Spec.ProviderID = nil
	}

	return utilconversion.MarshalData(src, dst)
}

func (src *Metal3MachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3MachineTemplate)
	if err := Convert_v1beta1_Metal3MachineTemplate_To_v1beta2_Metal3MachineTemplate(src, dst, nil); err != nil {
		return err
	}

	restored := &infrav1.Metal3MachineTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.Template.ObjectMeta = restored.Spec.Template.ObjectMeta
	// Restore Image.Checksum nil state from hub data
	if restored.Spec.Template.Spec.Image.Checksum == nil {
		dst.Spec.Template.Spec.Image.Checksum = nil
	}

	return nil
}

func (dst *Metal3MachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3MachineTemplate)
	if err := Convert_v1beta2_Metal3MachineTemplate_To_v1beta1_Metal3MachineTemplate(src, dst, nil); err != nil {
		return err
	}

	if dst.Spec.Template.Spec.ProviderID != nil && *dst.Spec.Template.Spec.ProviderID == "" {
		dst.Spec.Template.Spec.ProviderID = nil
	}

	return utilconversion.MarshalData(src, dst)
}

func (src *Metal3DataTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3DataTemplate)
	if err := Convert_v1beta1_Metal3DataTemplate_To_v1beta2_Metal3DataTemplate(src, dst, nil); err != nil {
		return err
	}

	// Convert Indexes from map[string]int to []IndexEntry
	if err := Convert_v1beta1_Metal3DataTemplateStatus_To_v1beta2_Metal3DataTemplateStatus(&src.Status, &dst.Status, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3DataTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3DataTemplate)
	if err := Convert_v1beta2_Metal3DataTemplate_To_v1beta1_Metal3DataTemplate(src, dst, nil); err != nil {
		return err
	}

	// Convert Indexes from []IndexEntry to map[string]int
	if err := Convert_v1beta2_Metal3DataTemplateStatus_To_v1beta1_Metal3DataTemplateStatus(&src.Status, &dst.Status, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3Data) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3Data)
	if err := Convert_v1beta1_Metal3Data_To_v1beta2_Metal3Data(src, dst, nil); err != nil {
		return err
	}

	// Convert empty ObjectReference values to nil pointers in v1beta2
	// The hub fuzzer expects empty pointers to be nil, not empty structs
	if src.Spec.Claim.Name == "" && src.Spec.Claim.Namespace == "" {
		dst.Spec.Claim = nil
	}
	if src.Spec.Template.Name == "" && src.Spec.Template.Namespace == "" {
		dst.Spec.Template = nil
	}

	return nil
}

func (dst *Metal3Data) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3Data)
	if err := Convert_v1beta2_Metal3Data_To_v1beta1_Metal3Data(src, dst, nil); err != nil {
		return err
	}

	// Convert nil pointers from v1beta2 to empty ObjectReference values in v1beta1
	// v1beta1 requires non-nil ObjectReference values (value types)
	if src.Spec.Claim == nil {
		dst.Spec.Claim = corev1.ObjectReference{}
	}
	if src.Spec.Template == nil {
		dst.Spec.Template = corev1.ObjectReference{}
	}

	return nil
}

func (src *Metal3DataClaim) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3DataClaim)
	if err := Convert_v1beta1_Metal3DataClaim_To_v1beta2_Metal3DataClaim(src, dst, nil); err != nil {
		return err
	}

	// Convert empty ObjectReference values to nil pointers in v1beta2
	// The hub fuzzer expects empty pointers to be nil, not empty structs
	if src.Spec.Template.Name == "" && src.Spec.Template.Namespace == "" {
		dst.Spec.Template = nil
	}

	return nil
}

func (dst *Metal3DataClaim) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3DataClaim)
	if err := Convert_v1beta2_Metal3DataClaim_To_v1beta1_Metal3DataClaim(src, dst, nil); err != nil {
		return err
	}

	// Convert nil pointers from v1beta2 to empty ObjectReference values in v1beta1
	// v1beta1 requires non-nil ObjectReference values (value types)
	if src.Spec.Template == nil {
		dst.Spec.Template = corev1.ObjectReference{}
	}

	return nil
}

func (src *Metal3Remediation) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3Remediation)
	if err := Convert_v1beta1_Metal3Remediation_To_v1beta2_Metal3Remediation(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3Remediation) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3Remediation)
	if err := Convert_v1beta2_Metal3Remediation_To_v1beta1_Metal3Remediation(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3RemediationTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3RemediationTemplate)
	if err := Convert_v1beta1_Metal3RemediationTemplate_To_v1beta2_Metal3RemediationTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3RemediationTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3RemediationTemplate)
	if err := Convert_v1beta2_Metal3RemediationTemplate_To_v1beta1_Metal3RemediationTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func Convert_v1beta2_Metal3ClusterSpec_To_v1beta1_Metal3ClusterSpec(in *infrav1.Metal3ClusterSpec, out *Metal3ClusterSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_Metal3ClusterSpec_To_v1beta1_Metal3ClusterSpec(in, out, s); err != nil {
		return err
	}
	if in.CloudProviderEnabled != nil {
		out.CloudProviderEnabled = ptr.To(*in.CloudProviderEnabled)
		out.NoCloudProvider = ptr.To(!*in.CloudProviderEnabled)
	}
	// Move FailureDomains
	if in.FailureDomains != nil {
		out.FailureDomains = FailureDomains{}
		for _, fd := range in.FailureDomains {
			out.FailureDomains[fd.Name] = clusterv1beta1.FailureDomainSpec{
				ControlPlane: ptr.Deref(fd.ControlPlane, false),
				Attributes:   fd.Attributes,
			}
		}
	}

	return nil
}

func Convert_v1beta1_Metal3ClusterSpec_To_v1beta2_Metal3ClusterSpec(in *Metal3ClusterSpec, out *infrav1.Metal3ClusterSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_Metal3ClusterSpec_To_v1beta2_Metal3ClusterSpec(in, out, s); err != nil {
		return err
	}
	if in.CloudProviderEnabled != nil {
		out.CloudProviderEnabled = ptr.To(*in.CloudProviderEnabled)
	}
	// Move FailureDomains
	if in.FailureDomains != nil {
		out.FailureDomains = []clusterv1.FailureDomain{}
		domainNames := slices.Collect(maps.Keys(in.FailureDomains))
		sort.Strings(domainNames)
		for _, name := range domainNames {
			fd := in.FailureDomains[name]
			failureDomain := clusterv1.FailureDomain{
				Name:       name,
				Attributes: fd.Attributes,
			}
			// Only set ControlPlane pointer if true (omitempty semantic)
			if fd.ControlPlane {
				failureDomain.ControlPlane = ptr.To(true)
			}
			out.FailureDomains = append(out.FailureDomains, failureDomain)
		}
	}

	return nil
}

// Convert_v1beta2_Metal3ClusterStatus_To_v1beta1_Metal3ClusterStatus is an autogenerated conversion function.
func Convert_v1beta2_Metal3ClusterStatus_To_v1beta1_Metal3ClusterStatus(in *infrav1.Metal3ClusterStatus, out *Metal3ClusterStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_Metal3ClusterStatus_To_v1beta1_Metal3ClusterStatus(in, out, s); err != nil {
		return err
	}

	// Reset conditions from autogenerated conversions
	// NOTE: v1beta2 conditions should not automatically be converted into legacy conditions (v1beta1).
	out.Conditions = nil

	// Retrieve legacy conditions (v1beta1) from the deprecated field.
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		if in.Deprecated.V1Beta1.Conditions != nil {
			clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta1.Conditions, &out.Conditions)
		}
		// Retrieve FailureReason and FailureMessage from deprecated field
		out.FailureReason = in.Deprecated.V1Beta1.FailureReason
		out.FailureMessage = in.Deprecated.V1Beta1.FailureMessage
	}

	// Move initialization to old field
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)

	// Move FailureDomains
	if in.FailureDomains != nil {
		out.FailureDomains = FailureDomains{}
		for _, fd := range in.FailureDomains {
			out.FailureDomains[fd.Name] = clusterv1beta1.FailureDomainSpec{
				ControlPlane: ptr.Deref(fd.ControlPlane, false),
				Attributes:   fd.Attributes,
			}
		}
	}

	// Move new conditions (v1beta2) to the v1beta2 field.
	if in.Conditions == nil {
		return nil
	}
	out.V1Beta2 = &Metal3ClusterV1Beta2Status{}
	out.V1Beta2.Conditions = in.Conditions
	return nil

}

// Convert_v1beta1_Metal3ClusterStatus_To_v1beta2_Metal3ClusterStatus is an autogenerated conversion function.
func Convert_v1beta1_Metal3ClusterStatus_To_v1beta2_Metal3ClusterStatus(in *Metal3ClusterStatus, out *infrav1.Metal3ClusterStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_Metal3ClusterStatus_To_v1beta2_Metal3ClusterStatus(in, out, s); err != nil {
		return err
	}

	// Reset conditions from autogenerated conversions
	// NOTE: v1beta1 conditions should not be automatically be converted into v1beta2 conditions.
	out.Conditions = nil

	// Retrieve new conditions (v1beta2) from the v1beta2 field.
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}

	// Move FailureDomains
	if in.FailureDomains != nil {
		out.FailureDomains = []clusterv1.FailureDomain{}
		domainNames := slices.Collect(maps.Keys(in.FailureDomains))
		sort.Strings(domainNames)
		for _, name := range domainNames {
			fd := in.FailureDomains[name]
			failureDomain := clusterv1.FailureDomain{
				Name:       name,
				Attributes: fd.Attributes,
			}
			// Only set ControlPlane pointer if true (omitempty semantic)
			if fd.ControlPlane {
				failureDomain.ControlPlane = ptr.To(true)
			}
			out.FailureDomains = append(out.FailureDomains, failureDomain)
		}
	}

	// Move legacy conditions (v1beta1) to the deprecated field.
	if in.Conditions != nil || in.FailureReason != nil || in.FailureMessage != nil {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1.Metal3ClusterDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1.Metal3ClusterV1Beta1DeprecatedStatus{}
		}
		if in.Conditions != nil {
			clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
		}
		// Move FailureReason and FailureMessage to deprecated field
		out.Deprecated.V1Beta1.FailureReason = in.FailureReason
		out.Deprecated.V1Beta1.FailureMessage = in.FailureMessage
	}

	return nil
}

func Convert_v1beta2_Metal3MachineStatus_To_v1beta1_Metal3MachineStatus(in *infrav1.Metal3MachineStatus, out *Metal3MachineStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_Metal3MachineStatus_To_v1beta1_Metal3MachineStatus(in, out, s); err != nil {
		return err
	}

	// Reset top-level failure fields from autogenerated conversions
	// NOTE: v1beta1 failure fields should only go to Deprecated, not top-level
	out.FailureReason = nil
	out.FailureMessage = nil

	// Reset conditions from autogenerated conversions
	// NOTE: v1beta2 conditions should not automatically be converted into legacy conditions (v1beta1).
	out.Conditions = nil

	// Retrieve legacy conditions (v1beta1), failureReason and failureMessage from the deprecated field.
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		if in.Deprecated.V1Beta1.Conditions != nil {
			clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta1.Conditions, &out.Conditions)
		}
		out.FailureReason = in.Deprecated.V1Beta1.FailureReason
		out.FailureMessage = in.Deprecated.V1Beta1.FailureMessage
	}

	out.Phase = "" // Phase is deprecated and it was never used in v1beta1, so we don't want to populate it during conversion.

	// Move initialization to old field
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)

	// Move new conditions (v1beta2) to the v1beta2 field.
	if in.Conditions == nil {
		return nil
	}
	out.V1Beta2 = &Metal3MachineV1Beta2Status{}
	out.V1Beta2.Conditions = in.Conditions

	if in.RenderedData != nil {
		out.RenderedData = &corev1.ObjectReference{
			Name:      in.RenderedData.Name,
			Namespace: in.RenderedData.Namespace,
		}
	}

	return nil
}

func Convert_v1beta1_Metal3MachineStatus_To_v1beta2_Metal3MachineStatus(in *Metal3MachineStatus, out *infrav1.Metal3MachineStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_Metal3MachineStatus_To_v1beta2_Metal3MachineStatus(in, out, s); err != nil {
		return err
	}

	// Reset conditions from autogenerated conversions
	// NOTE: v1beta1 conditions should not be automatically be converted into v1beta2 conditions.
	out.Conditions = nil

	// Retrieve new conditions (v1beta2) from the v1beta2 field.
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}

	// Move legacy conditions (v1beta1), failureReason and failureMessage to the deprecated field.
	if in.FailureReason == nil && in.FailureMessage == nil && in.Conditions == nil && in.Phase == "" {
		return nil
	}

	if out.Deprecated == nil {
		out.Deprecated = &infrav1.Metal3MachineDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &infrav1.Metal3MachineV1Beta1DeprecatedStatus{}
	}
	if in.Conditions != nil {
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
	}
	out.Deprecated.V1Beta1.FailureReason = in.FailureReason
	out.Deprecated.V1Beta1.FailureMessage = in.FailureMessage

	if in.RenderedData != nil && !reflect.DeepEqual(in.RenderedData, &corev1.ObjectReference{}) {
		out.RenderedData = &infrav1.Metal3ObjectRef{}
		out.RenderedData.Name = in.RenderedData.Name
		out.RenderedData.Namespace = in.RenderedData.Namespace
	}
	return nil
}

func Convert_v1beta2_Metal3ClusterTemplateResource_To_v1beta1_Metal3ClusterTemplateResource(in *infrav1.Metal3ClusterTemplateResource, out *Metal3ClusterTemplateResource, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_Metal3ClusterTemplateResource_To_v1beta1_Metal3ClusterTemplateResource(in, out, s); err != nil {
		return err
	}

	return nil
}

func Convert_v1beta2_Metal3MachineTemplateResource_To_v1beta1_Metal3MachineTemplateResource(in *infrav1.Metal3MachineTemplateResource, out *Metal3MachineTemplateResource, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_Metal3MachineTemplateResource_To_v1beta1_Metal3MachineTemplateResource(in, out, s); err != nil {
		return err
	}

	return nil
}

func Convert_v1_Condition_To_v1beta1_Condition(_ *metav1.Condition, _ *clusterv1beta1.Condition, _ apimachineryconversion.Scope) error {
	// NOTE: v1beta2 conditions should not be automatically converted into legacy (v1beta1) conditions.
	return nil
}

func Convert_v1beta1_Condition_To_v1_Condition(_ *clusterv1beta1.Condition, _ *metav1.Condition, _ apimachineryconversion.Scope) error {
	// NOTE: legacy (v1beta1) conditions should not be automatically converted into v1beta2 conditions.
	return nil
}

func Convert_v1beta1_ObjectMeta_To_v1beta2_ObjectMeta(in *clusterv1beta1.ObjectMeta, out *clusterv1.ObjectMeta, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1beta1_ObjectMeta_To_v1beta2_ObjectMeta(in, out, s)
}

func Convert_v1beta2_ObjectMeta_To_v1beta1_ObjectMeta(in *clusterv1.ObjectMeta, out *clusterv1beta1.ObjectMeta, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1beta2_ObjectMeta_To_v1beta1_ObjectMeta(in, out, s)
}

func Convert_v1beta1_MachineAddress_To_v1beta2_MachineAddress(in *clusterv1beta1.MachineAddress, out *clusterv1.MachineAddress, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1beta1_MachineAddress_To_v1beta2_MachineAddress(in, out, s)
}

func Convert_v1beta2_MachineAddress_To_v1beta1_MachineAddress(in *clusterv1.MachineAddress, out *clusterv1beta1.MachineAddress, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1beta2_MachineAddress_To_v1beta1_MachineAddress(in, out, s)
}

func Convert_v1beta2_APIEndpoint_To_v1beta1_APIEndpoint(in *infrav1.APIEndpoint, out *APIEndpoint, s apimachineryconversion.Scope) error {
	out.Host = in.Host
	out.Port = int(in.Port)
	return nil
}

func Convert_v1beta1_APIEndpoint_To_v1beta2_APIEndpoint(in *APIEndpoint, out *infrav1.APIEndpoint, s apimachineryconversion.Scope) error {
	out.Host = in.Host
	out.Port = int32(in.Port)
	return nil
}

// Convert_v1beta1_Image_To_v1beta2_Image handles conversion of Image from v1beta1 to v1beta2.
// In v1beta1: Checksum is string, ChecksumType and DiskFormat are *string.
// In v1beta2: Checksum is *string, ChecksumType and DiskFormat are string.
func Convert_v1beta1_Image_To_v1beta2_Image(in *Image, out *infrav1.Image, s apimachineryconversion.Scope) error {
	out.URL = in.URL
	out.Checksum = ptr.To(in.Checksum)
	out.ChecksumType = ptr.Deref(in.ChecksumType, "")
	out.DiskFormat = ptr.Deref(in.DiskFormat, "")
	return nil
}

// Convert_v1beta2_Image_To_v1beta1_Image handles conversion of Image from v1beta2 to v1beta1.
// In v1beta2: Checksum is *string, ChecksumType and DiskFormat are string.
// In v1beta1: Checksum is string, ChecksumType and DiskFormat are *string.
func Convert_v1beta2_Image_To_v1beta1_Image(in *infrav1.Image, out *Image, s apimachineryconversion.Scope) error {
	out.URL = in.URL
	out.Checksum = ptr.Deref(in.Checksum, "")
	// Always set pointers (use &"" for empty) to enable proper round-trip via hub restoration
	out.ChecksumType = ptr.To(in.ChecksumType)
	out.DiskFormat = ptr.To(in.DiskFormat)
	return nil
}

// Convert_v1beta1_Metal3MachineTemplateSpec_To_v1beta2_Metal3MachineTemplateSpec handles the manual conversion
// of Metal3MachineTemplateSpec from v1beta1 to v1beta2. The NodeReuse field changes from bool to *bool.
func Convert_v1beta1_Metal3MachineTemplateSpec_To_v1beta2_Metal3MachineTemplateSpec(in *Metal3MachineTemplateSpec, out *infrav1.Metal3MachineTemplateSpec, s apimachineryconversion.Scope) error {
	if err := Convert_v1beta1_Metal3MachineTemplateResource_To_v1beta2_Metal3MachineTemplateResource(&in.Template, &out.Template, s); err != nil {
		return err
	}
	// Convert NodeReuse from bool to *bool, only set if true
	if in.NodeReuse {
		out.NodeReuse = ptr.To(in.NodeReuse)
	}
	return nil
}

// Convert_v1beta2_Metal3MachineTemplateSpec_To_v1beta1_Metal3MachineTemplateSpec handles the manual conversion
// of Metal3MachineTemplateSpec from v1beta2 to v1beta1. The NodeReuse field changes from *bool to bool.
func Convert_v1beta2_Metal3MachineTemplateSpec_To_v1beta1_Metal3MachineTemplateSpec(in *infrav1.Metal3MachineTemplateSpec, out *Metal3MachineTemplateSpec, s apimachineryconversion.Scope) error {
	if err := Convert_v1beta2_Metal3MachineTemplateResource_To_v1beta1_Metal3MachineTemplateResource(&in.Template, &out.Template, s); err != nil {
		return err
	}
	// Convert NodeReuse from *bool to bool
	out.NodeReuse = ptr.Deref(in.NodeReuse, false)
	return nil
}

// Convert_v1beta1_Metal3DataTemplateSpec_To_v1beta2_Metal3DataTemplateSpec handles the manual conversion
// of Metal3DataTemplateSpec from v1beta1 to v1beta2. The TemplateReference field was removed in v1beta2.
func Convert_v1beta1_Metal3DataTemplateSpec_To_v1beta2_Metal3DataTemplateSpec(in *Metal3DataTemplateSpec, out *infrav1.Metal3DataTemplateSpec, s apimachineryconversion.Scope) error {
	// TemplateReference is dropped as it was removed in v1beta2
	return autoConvert_v1beta1_Metal3DataTemplateSpec_To_v1beta2_Metal3DataTemplateSpec(in, out, s)
}

// Convert_v1beta2_Metal3DataTemplateSpec_To_v1beta1_Metal3DataTemplateSpec handles the manual conversion
// of Metal3DataTemplateSpec from v1beta2 to v1beta1. The TemplateReference field was removed in v1beta2,
// so it will not be restored during conversion.
func Convert_v1beta2_Metal3DataTemplateSpec_To_v1beta1_Metal3DataTemplateSpec(in *infrav1.Metal3DataTemplateSpec, out *Metal3DataTemplateSpec, s apimachineryconversion.Scope) error {
	// TemplateReference is dropped as it was removed in v1beta2 and cannot be restored
	return autoConvert_v1beta2_Metal3DataTemplateSpec_To_v1beta1_Metal3DataTemplateSpec(in, out, s)
}

// Convert_v1beta1_NetworkDataRoutev4_To_v1beta2_NetworkDataRoutev4 handles the manual conversion
// of NetworkDataRoutev4 from v1beta1 to v1beta2. The Prefix field changed from int to int32.
func Convert_v1beta1_NetworkDataRoutev4_To_v1beta2_NetworkDataRoutev4(in *NetworkDataRoutev4, out *infrav1.NetworkDataRoutev4, s apimachineryconversion.Scope) error {
	out.Network = ipamv1.IPAddressv4Str(in.Network)
	if in.Prefix != 0 {
		out.Prefix = ptr.To(int32(in.Prefix))
	}
	if err := Convert_v1beta1_NetworkGatewayv4_To_v1beta2_NetworkGatewayv4(&in.Gateway, &out.Gateway, s); err != nil {
		return err
	}
	if !reflect.DeepEqual(in.Services, NetworkDataServicev4{}) {
		out.Services = &infrav1.NetworkDataServicev4{}
		if err := Convert_v1beta1_NetworkDataServicev4_To_v1beta2_NetworkDataServicev4(&in.Services, out.Services, s); err != nil {
			return err
		}
	}
	return nil
}

// Convert_v1beta1_NetworkDataRoutev6_To_v1beta2_NetworkDataRoutev6 handles the manual conversion
// of NetworkDataRoutev6 from v1beta1 to v1beta2. The Prefix field changed from int to int32.
func Convert_v1beta1_NetworkDataRoutev6_To_v1beta2_NetworkDataRoutev6(in *NetworkDataRoutev6, out *infrav1.NetworkDataRoutev6, s apimachineryconversion.Scope) error {
	out.Network = ipamv1.IPAddressv6Str(in.Network)
	if in.Prefix != 0 {
		out.Prefix = ptr.To(int32(in.Prefix))
	}
	if err := Convert_v1beta1_NetworkGatewayv6_To_v1beta2_NetworkGatewayv6(&in.Gateway, &out.Gateway, s); err != nil {
		return err
	}
	if !reflect.DeepEqual(in.Services, NetworkDataServicev6{}) {
		out.Services = &infrav1.NetworkDataServicev6{}
		if err := Convert_v1beta1_NetworkDataServicev6_To_v1beta2_NetworkDataServicev6(&in.Services, out.Services, s); err != nil {
			return err
		}
	}
	return nil
}

// Convert_v1beta2_NetworkDataRoutev4_To_v1beta1_NetworkDataRoutev4 handles the manual conversion
// of NetworkDataRoutev4 from v1beta2 to v1beta1. The Prefix field changed from int32 to int.
func Convert_v1beta2_NetworkDataRoutev4_To_v1beta1_NetworkDataRoutev4(in *infrav1.NetworkDataRoutev4, out *NetworkDataRoutev4, s apimachineryconversion.Scope) error {
	out.Network = ipamv1.IPAddressv4Str(in.Network)
	out.Prefix = int(ptr.Deref(in.Prefix, 0))
	if err := Convert_v1beta2_NetworkGatewayv4_To_v1beta1_NetworkGatewayv4(&in.Gateway, &out.Gateway, s); err != nil {
		return err
	}
	if in.Services != nil {
		if err := Convert_v1beta2_NetworkDataServicev4_To_v1beta1_NetworkDataServicev4(in.Services, &out.Services, s); err != nil {
			return err
		}
	}
	return nil
}

// Convert_v1beta2_NetworkDataRoutev6_To_v1beta1_NetworkDataRoutev6 handles the manual conversion
// of NetworkDataRoutev6 from v1beta2 to v1beta1. The Prefix field changed from int32 to int.
func Convert_v1beta2_NetworkDataRoutev6_To_v1beta1_NetworkDataRoutev6(in *infrav1.NetworkDataRoutev6, out *NetworkDataRoutev6, s apimachineryconversion.Scope) error {
	out.Network = ipamv1.IPAddressv6Str(in.Network)
	out.Prefix = int(ptr.Deref(in.Prefix, 0))
	if err := Convert_v1beta2_NetworkGatewayv6_To_v1beta1_NetworkGatewayv6(&in.Gateway, &out.Gateway, s); err != nil {
		return err
	}
	if in.Services != nil {
		if err := Convert_v1beta2_NetworkDataServicev6_To_v1beta1_NetworkDataServicev6(in.Services, &out.Services, s); err != nil {
			return err
		}
	}
	return nil
}

// Convert_v1beta1_MetaDataIndex_To_v1beta2_MetaDataIndex handles the manual conversion
// of MetaDataIndex from v1beta1 to v1beta2. The Offset field changed from int to *int32, and Step from int to int32.
func Convert_v1beta1_MetaDataIndex_To_v1beta2_MetaDataIndex(in *MetaDataIndex, out *infrav1.MetaDataIndex, s apimachineryconversion.Scope) error {
	out.Key = in.Key
	out.Prefix = in.Prefix
	out.Suffix = in.Suffix
	out.Step = int32(in.Step)
	// Convert Offset from int to *int32, only set if non-zero
	if in.Offset != 0 {
		out.Offset = ptr.To(int32(in.Offset))
	}
	return nil
}

// Convert_v1beta2_MetaDataIndex_To_v1beta1_MetaDataIndex handles the manual conversion
// of MetaDataIndex from v1beta2 to v1beta1. The Offset field changed from *int32 to int, and Step from int32 to int.
func Convert_v1beta2_MetaDataIndex_To_v1beta1_MetaDataIndex(in *infrav1.MetaDataIndex, out *MetaDataIndex, s apimachineryconversion.Scope) error {
	out.Key = in.Key
	out.Prefix = in.Prefix
	out.Suffix = in.Suffix
	out.Step = int(in.Step)
	// Convert Offset from *int32 to int
	if in.Offset != nil {
		out.Offset = int(*in.Offset)
	}
	return nil
}

// Convert_v1beta1_NetworkLinkEthernetMac_To_v1beta2_NetworkLinkEthernetMac handles the manual conversion
// of NetworkLinkEthernetMac from v1beta1 to v1beta2. The FromAnnotation field changed from *NetworkLinkEthernetMacFromAnnotation to NetworkLinkEthernetMacFromAnnotation.
func Convert_v1beta1_NetworkLinkEthernetMac_To_v1beta2_NetworkLinkEthernetMac(in *NetworkLinkEthernetMac, out *infrav1.NetworkLinkEthernetMac, s apimachineryconversion.Scope) error {
	out.String = in.String
	out.FromHostInterface = in.FromHostInterface
	// Convert FromAnnotation from *NetworkLinkEthernetMacFromAnnotation to NetworkLinkEthernetMacFromAnnotation
	if in.FromAnnotation != nil {
		out.FromAnnotation = infrav1.NetworkLinkEthernetMacFromAnnotation{
			Object:     in.FromAnnotation.Object,
			Annotation: in.FromAnnotation.Annotation,
		}
	}
	return nil
}

// Convert_v1beta2_NetworkLinkEthernetMac_To_v1beta1_NetworkLinkEthernetMac handles the manual conversion
// of NetworkLinkEthernetMac from v1beta2 to v1beta1. The FromAnnotation field changed from NetworkLinkEthernetMacFromAnnotation to *NetworkLinkEthernetMacFromAnnotation.
func Convert_v1beta2_NetworkLinkEthernetMac_To_v1beta1_NetworkLinkEthernetMac(in *infrav1.NetworkLinkEthernetMac, out *NetworkLinkEthernetMac, s apimachineryconversion.Scope) error {
	out.String = in.String
	out.FromHostInterface = in.FromHostInterface
	// Convert FromAnnotation from NetworkLinkEthernetMacFromAnnotation to *NetworkLinkEthernetMacFromAnnotation
	// Only set pointer if non-empty to preserve nil semantics
	if in.FromAnnotation.Object != "" || in.FromAnnotation.Annotation != "" {
		out.FromAnnotation = &NetworkLinkEthernetMacFromAnnotation{
			Object:     in.FromAnnotation.Object,
			Annotation: in.FromAnnotation.Annotation,
		}
	}
	return nil
}

// Convert_v1beta1_RemediationStrategy_To_v1beta2_RemediationStrategy handles the manual conversion
// of RemediationStrategy from v1beta1 to v1beta2. The Timeout field changed from *metav1.Duration to TimeoutSeconds *int32.
func Convert_v1beta1_RemediationStrategy_To_v1beta2_RemediationStrategy(in *RemediationStrategy, out *infrav1.RemediationStrategy, _ apimachineryconversion.Scope) error {
	out.Type = infrav1.RemediationType(in.Type)
	out.RetryLimit = int32(in.RetryLimit)
	if in.Timeout != nil {
		out.TimeoutSeconds = *clusterv1.ConvertToSeconds(in.Timeout)
	}
	return nil
}

// Convert_v1beta2_RemediationStrategy_To_v1beta1_RemediationStrategy handles the manual conversion
// of RemediationStrategy from v1beta2 to v1beta1. The TimeoutSeconds *int32 field changed to Timeout *metav1.Duration.
func Convert_v1beta2_RemediationStrategy_To_v1beta1_RemediationStrategy(in *infrav1.RemediationStrategy, out *RemediationStrategy, _ apimachineryconversion.Scope) error {
	out.Type = RemediationType(in.Type)
	out.RetryLimit = int(in.RetryLimit)
	if in.TimeoutSeconds != 0 {
		out.Timeout = clusterv1.ConvertFromSeconds(&in.TimeoutSeconds)
	}

	return nil
}

// Convert_v1beta1_Metal3MachineSpec_To_v1beta2_Metal3MachineSpec handles the manual conversion
// of Metal3MachineSpec from v1beta1 to v1beta2. Several fields changed from pointers to non-pointers:
// ProviderID (*string to string), CustomDeploy (*CustomDeploy to CustomDeploy),
// AutomatedCleaningMode (*string to string), DataTemplate (ObjectReference to Metal3ObjectRef).
func Convert_v1beta1_Metal3MachineSpec_To_v1beta2_Metal3MachineSpec(in *Metal3MachineSpec, out *infrav1.Metal3MachineSpec, s apimachineryconversion.Scope) error {
	// Convert ProviderID from *string to string
	out.ProviderID = ptr.Deref(in.ProviderID, "")

	// Copy other non-pointer fields
	out.UserData = in.UserData
	out.MetaData = in.MetaData
	out.NetworkData = in.NetworkData
	out.FailureDomain = in.FailureDomain

	// Convert DataTemplate from ObjectReference to Metal3ObjectRef
	if in.DataTemplate != nil {
		out.DataTemplate = &infrav1.Metal3ObjectRef{}
		if err := Convert_v1_ObjectReference_To_v1beta2_Metal3ObjectRef(in.DataTemplate, out.DataTemplate, s); err != nil {
			return err
		}
	}

	// Convert HostSelector manually (copy fields and convert MatchExpressions)
	if len(in.HostSelector.MatchLabels) > 0 || len(in.HostSelector.MatchExpressions) > 0 {
		out.HostSelector = &infrav1.HostSelector{
			MatchLabels: in.HostSelector.MatchLabels,
		}
		if len(in.HostSelector.MatchExpressions) > 0 {
			out.HostSelector.MatchExpressions = make([]infrav1.HostSelectorRequirement, len(in.HostSelector.MatchExpressions))
			for i, expr := range in.HostSelector.MatchExpressions {
				out.HostSelector.MatchExpressions[i] = infrav1.HostSelectorRequirement{
					Key:      expr.Key,
					Operator: expr.Operator,
					Values:   expr.Values,
				}
			}
		}
	}

	// Convert AutomatedCleaningMode from *string to string
	out.AutomatedCleaningMode = ptr.Deref(in.AutomatedCleaningMode, "")

	// Convert Image
	if err := Convert_v1beta1_Image_To_v1beta2_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}

	// Convert CustomDeploy from *CustomDeploy to CustomDeploy
	// Only set if Method is not empty
	if in.CustomDeploy != nil && in.CustomDeploy.Method != "" {
		out.CustomDeploy = infrav1.CustomDeploy{
			Method: in.CustomDeploy.Method,
		}
	}

	return nil
}

// Convert_v1beta2_Metal3MachineSpec_To_v1beta1_Metal3MachineSpec handles the manual conversion
// of Metal3MachineSpec from v1beta2 to v1beta1. Several fields changed from non-pointers to pointers:
// ProviderID (string to *string), CustomDeploy (CustomDeploy to *CustomDeploy),
// AutomatedCleaningMode (string to *string), DataTemplate (Metal3ObjectRef to ObjectReference).
func Convert_v1beta2_Metal3MachineSpec_To_v1beta1_Metal3MachineSpec(in *infrav1.Metal3MachineSpec, out *Metal3MachineSpec, s apimachineryconversion.Scope) error {
	// Convert ProviderID from string to *string, only set if non-empty
	if in.ProviderID != "" {
		out.ProviderID = ptr.To(in.ProviderID)
	}

	// Copy other fields
	out.UserData = in.UserData
	out.MetaData = in.MetaData
	out.NetworkData = in.NetworkData
	out.FailureDomain = in.FailureDomain

	// Convert DataTemplate from Metal3ObjectRef to ObjectReference
	if in.DataTemplate != nil {
		out.DataTemplate = &corev1.ObjectReference{}
		if err := Convert_v1beta2_Metal3ObjectRef_To_v1_ObjectReference(in.DataTemplate, out.DataTemplate, s); err != nil {
			return err
		}
	}

	// Convert HostSelector manually (copy fields and convert MatchExpressions)
	// Only create pointer if HostSelector has content
	if in.HostSelector != nil && (len(in.HostSelector.MatchLabels) > 0 || len(in.HostSelector.MatchExpressions) > 0) {
		out.HostSelector = HostSelector{
			MatchLabels: in.HostSelector.MatchLabels,
		}
		if len(in.HostSelector.MatchExpressions) > 0 {
			out.HostSelector.MatchExpressions = make([]HostSelectorRequirement, len(in.HostSelector.MatchExpressions))
			for i, expr := range in.HostSelector.MatchExpressions {
				out.HostSelector.MatchExpressions[i] = HostSelectorRequirement{
					Key:      expr.Key,
					Operator: expr.Operator,
					Values:   expr.Values,
				}
			}
		}
	}

	// Convert AutomatedCleaningMode from string to *string, only set if non-empty
	if in.AutomatedCleaningMode != "" {
		out.AutomatedCleaningMode = ptr.To(in.AutomatedCleaningMode)
	}

	// Convert Image
	if err := Convert_v1beta2_Image_To_v1beta1_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}

	// Convert CustomDeploy from CustomDeploy to *CustomDeploy
	if in.CustomDeploy.Method != "" {
		out.CustomDeploy = &CustomDeploy{
			Method: in.CustomDeploy.Method,
		}
	}

	return nil
}

// Convert_v1beta1_Metal3DataTemplateStatus_To_v1beta2_Metal3DataTemplateStatus handles conversion
// of Metal3DataTemplateStatus from v1beta1 to v1beta2. The Indexes field is converted from a map to a list.
func Convert_v1beta1_Metal3DataTemplateStatus_To_v1beta2_Metal3DataTemplateStatus(in *Metal3DataTemplateStatus, out *infrav1.Metal3DataTemplateStatus, s apimachineryconversion.Scope) error {
	out.LastUpdated = in.LastUpdated

	// Convert map to list.
	if len(in.Indexes) > 0 {
		out.Indexes = make([]infrav1.IndexEntry, 0, len(in.Indexes))
		for name, index := range in.Indexes {
			out.Indexes = append(out.Indexes, infrav1.IndexEntry{
				Name:  name,
				Index: ptr.To(int32(index)),
			})
		}
	}

	// Ensure deterministic ordering of indexes by index
	sort.Slice(out.Indexes, func(i, j int) bool {
		iVal := ptr.Deref(out.Indexes[i].Index, 0)
		jVal := ptr.Deref(out.Indexes[j].Index, 0)
		return iVal < jVal
	})

	return nil
}

// Convert_v1beta2_Metal3DataTemplateStatus_To_v1beta1_Metal3DataTemplateStatus handles conversion
// of Metal3DataTemplateStatus from v1beta2 to v1beta1. The Indexes field is converted from a list to a map.
func Convert_v1beta2_Metal3DataTemplateStatus_To_v1beta1_Metal3DataTemplateStatus(in *infrav1.Metal3DataTemplateStatus, out *Metal3DataTemplateStatus, s apimachineryconversion.Scope) error {
	out.LastUpdated = in.LastUpdated

	// Convert list back to map, filtering out entries without a valid Index
	if len(in.Indexes) > 0 {
		out.Indexes = make(map[string]int, len(in.Indexes))
		for _, entry := range in.Indexes {
			out.Indexes[entry.Name] = int(*entry.Index)
		}
	}

	return nil
}

// Convert_v1beta1_NetworkDataLinkBond_To_v1beta2_NetworkDataLinkBond handles conversion
// of NetworkDataLinkBond from v1beta1 to v1beta2. The Parameters field is converted from a map to a list.
func Convert_v1beta1_NetworkDataLinkBond_To_v1beta2_NetworkDataLinkBond(in *NetworkDataLinkBond, out *infrav1.NetworkDataLinkBond, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_NetworkDataLinkBond_To_v1beta2_NetworkDataLinkBond(in, out, s); err != nil {
		return err
	}

	// Convert map to list
	if in.Parameters != nil {
		out.Parameters = make([]infrav1.NetworkDataLinkBondParam, 0, len(in.Parameters))
		for name, value := range in.Parameters {
			out.Parameters = append(out.Parameters, infrav1.NetworkDataLinkBondParam{
				Name:  name,
				Value: value,
			})
		}
	}

	// Ensure deterministic ordering of parameters by name
	sort.Slice(out.Parameters, func(i, j int) bool {
		return out.Parameters[i].Name < out.Parameters[j].Name
	})

	return nil
}

// Convert_v1beta2_NetworkDataLinkBond_To_v1beta1_NetworkDataLinkBond handles conversion
// of NetworkDataLinkBond from v1beta2 to v1beta1. The Parameters field is converted from a list to a map.
func Convert_v1beta2_NetworkDataLinkBond_To_v1beta1_NetworkDataLinkBond(in *infrav1.NetworkDataLinkBond, out *NetworkDataLinkBond, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_NetworkDataLinkBond_To_v1beta1_NetworkDataLinkBond(in, out, s); err != nil {
		return err
	}

	// Convert list back to map
	if in.Parameters != nil {
		out.Parameters = make(NetworkDataLinkBondParams, len(in.Parameters))
		for _, param := range in.Parameters {
			out.Parameters[param.Name] = param.Value
		}
	}

	return nil
}

func Convert_v1_TypedLocalObjectReference_To_v1beta2_IPPoolReference(in *corev1.TypedLocalObjectReference, out *infrav1.IPPoolReference, _ apimachineryconversion.Scope) error {
	out.Kind = in.Kind
	out.Name = in.Name
	out.APIGroup = ptr.Deref(in.APIGroup, "")
	return nil
}

func Convert_v1beta2_IPPoolReference_To_v1_TypedLocalObjectReference(in *infrav1.IPPoolReference, out *corev1.TypedLocalObjectReference, _ apimachineryconversion.Scope) error {
	out.Kind = in.Kind
	out.Name = in.Name
	if in.APIGroup != "" {
		out.APIGroup = ptr.To(in.APIGroup)
	}
	return nil
}

// Convert_v1beta2_Metal3DataSpec_To_v1beta1_Metal3DataSpec handles the manual conversion of Metal3DataSpec from v1beta2 to v1beta1.
func Convert_v1beta2_Metal3DataSpec_To_v1beta1_Metal3DataSpec(in *infrav1.Metal3DataSpec, out *Metal3DataSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_Metal3DataSpec_To_v1beta1_Metal3DataSpec(in, out, s); err != nil {
		return err
	}

	// Convert Index from *int32 to int
	out.Index = int(ptr.Deref(in.Index, 0))

	// Convert Claim: v1beta1 requires non-nil ObjectReference
	if in.Claim != nil {
		if err := Convert_v1beta2_Metal3ObjectRef_To_v1_ObjectReference(in.Claim, &out.Claim, s); err != nil {
			return err
		}
	} else {
		out.Claim = corev1.ObjectReference{}
	}

	// Convert Template: v1beta1 requires non-nil ObjectReference
	if in.Template != nil {
		if err := Convert_v1beta2_Metal3ObjectRef_To_v1_ObjectReference(in.Template, &out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = corev1.ObjectReference{}
	}

	return nil
}

// Convert_v1beta1_Metal3DataSpec_To_v1beta2_Metal3DataSpec handles the manual conversion of Metal3DataSpec from v1beta1 to v1beta2.
func Convert_v1beta1_Metal3DataSpec_To_v1beta2_Metal3DataSpec(in *Metal3DataSpec, out *infrav1.Metal3DataSpec, s apimachineryconversion.Scope) error {
	// TemplateReference is dropped as it was removed in v1beta2
	if err := autoConvert_v1beta1_Metal3DataSpec_To_v1beta2_Metal3DataSpec(in, out, s); err != nil {
		return err
	}

	// Convert Index from int to *int32
	// Always set, even for 0, since 0 is a valid index (the first data entry).
	out.Index = ptr.To(int32(in.Index))

	// Convert Claim: create Metal3ObjectRef only if it has actual data
	if in.Claim.Name != "" || in.Claim.Namespace != "" {
		out.Claim = &infrav1.Metal3ObjectRef{}
		if err := Convert_v1_ObjectReference_To_v1beta2_Metal3ObjectRef(&in.Claim, out.Claim, s); err != nil {
			return err
		}
	}

	// Convert Template: create Metal3ObjectRef only if it has actual data
	if in.Template.Name != "" || in.Template.Namespace != "" {
		out.Template = &infrav1.Metal3ObjectRef{}
		if err := Convert_v1_ObjectReference_To_v1beta2_Metal3ObjectRef(&in.Template, out.Template, s); err != nil {
			return err
		}
	}

	return nil
}

// Convert_v1beta1_Metal3DataClaimSpec_To_v1beta2_Metal3DataClaimSpec handles the manual conversion of Metal3DataClaimSpec from v1beta1 to v1beta2.
func Convert_v1beta1_Metal3DataClaimSpec_To_v1beta2_Metal3DataClaimSpec(in *Metal3DataClaimSpec, out *infrav1.Metal3DataClaimSpec, s apimachineryconversion.Scope) error {
	// Convert Template: create Metal3ObjectRef only if it has actual data
	if in.Template.Name != "" || in.Template.Namespace != "" {
		out.Template = &infrav1.Metal3ObjectRef{}
		if err := Convert_v1_ObjectReference_To_v1beta2_Metal3ObjectRef(&in.Template, out.Template, s); err != nil {
			return err
		}
	}

	return nil
}

// Convert_v1beta2_Metal3DataClaimSpec_To_v1beta1_Metal3DataClaimSpec handles the manual conversion of Metal3DataClaimSpec from v1beta2 to v1beta1.
func Convert_v1beta2_Metal3DataClaimSpec_To_v1beta1_Metal3DataClaimSpec(in *infrav1.Metal3DataClaimSpec, out *Metal3DataClaimSpec, s apimachineryconversion.Scope) error {
	// Convert Template: create ObjectReference from Metal3ObjectRef
	if in.Template != nil {
		if err := Convert_v1beta2_Metal3ObjectRef_To_v1_ObjectReference(in.Template, &out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = corev1.ObjectReference{}
	}

	return nil
}

func Convert_v1_ObjectReference_To_v1beta2_Metal3ObjectRef(in *corev1.ObjectReference, out *infrav1.Metal3ObjectRef, _ apimachineryconversion.Scope) error {
	out.Name = in.Name
	out.Namespace = in.Namespace
	return nil
}

func Convert_v1beta2_Metal3ObjectRef_To_v1_ObjectReference(in *infrav1.Metal3ObjectRef, out *corev1.ObjectReference, _ apimachineryconversion.Scope) error {
	out.Name = in.Name
	out.Namespace = in.Namespace
	return nil
}

// convertPoolRefPointerToValue converts FromPoolRef from pointer (v1beta1) to value (v1beta2).
func convertPoolRefPointerToValue(in *corev1.TypedLocalObjectReference, out *infrav1.IPPoolReference, s apimachineryconversion.Scope) error {
	if in != nil {
		return Convert_v1_TypedLocalObjectReference_To_v1beta2_IPPoolReference(in, out, s)
	}
	return nil
}

// convertPoolRefValueToPointer converts FromPoolRef from value (v1beta2) to pointer (v1beta1).
// Always returns a non-nil pointer to ensure round-trip conversion preserves the struct.
func convertPoolRefValueToPointer(in *infrav1.IPPoolReference, s apimachineryconversion.Scope) (*corev1.TypedLocalObjectReference, error) {
	out := new(corev1.TypedLocalObjectReference)
	if err := Convert_v1beta2_IPPoolReference_To_v1_TypedLocalObjectReference(in, out, s); err != nil {
		return nil, err
	}
	return out, nil
}

// convertFromPoolAnnotationPointerToValue converts FromPoolAnnotation from *FromPoolAnnotation (v1beta1) to FromPoolAnnotation (v1beta2).
func convertFromPoolAnnotationPointerToValue(in *FromPoolAnnotation, out *infrav1.FromPoolAnnotation) {
	if in != nil {
		out.Object = in.Object
		out.Annotation = in.Annotation
	}
}

// convertFromPoolAnnotationValueToPointer converts FromPoolAnnotation from FromPoolAnnotation (v1beta2) to *FromPoolAnnotation (v1beta1).
// Only returns a non-nil pointer if the annotation has content, to preserve nil semantics.
func convertFromPoolAnnotationValueToPointer(in *infrav1.FromPoolAnnotation) *FromPoolAnnotation {
	if in.Object != "" || in.Annotation != "" {
		return &FromPoolAnnotation{
			Object:     in.Object,
			Annotation: in.Annotation,
		}
	}
	return nil
}

// Convert_v1beta1_NetworkDataLinkVlan_To_v1beta2_NetworkDataLinkVlan handles conversion
// of NetworkDataLinkVlan from v1beta1 to v1beta2. The VlanID field changed from int to *int32.
func Convert_v1beta1_NetworkDataLinkVlan_To_v1beta2_NetworkDataLinkVlan(in *NetworkDataLinkVlan, out *infrav1.NetworkDataLinkVlan, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_NetworkDataLinkVlan_To_v1beta2_NetworkDataLinkVlan(in, out, s); err != nil {
		return err
	}

	// Convert VlanID from int to *int32, always preserving the value (including 0).
	out.VlanID = ptr.To(int32(in.VlanID))

	return nil
}

// Convert_v1beta2_NetworkDataLinkVlan_To_v1beta1_NetworkDataLinkVlan handles conversion
// of NetworkDataLinkVlan from v1beta2 to v1beta1. The VlanID field changed from *int32 to int.
func Convert_v1beta2_NetworkDataLinkVlan_To_v1beta1_NetworkDataLinkVlan(in *infrav1.NetworkDataLinkVlan, out *NetworkDataLinkVlan, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_NetworkDataLinkVlan_To_v1beta1_NetworkDataLinkVlan(in, out, s); err != nil {
		return err
	}

	// Convert VlanID from *int32 to int.
	// VlanID is required in v1beta2, so it should not be nil; default to 0 if missing.
	if in.VlanID != nil {
		out.VlanID = int(*in.VlanID)
	} else {
		out.VlanID = 0
	}

	return nil
}

// Convert_v1beta1_NetworkGatewayv4_To_v1beta2_NetworkGatewayv4 handles conversion
// of NetworkGatewayv4 from v1beta1 to v1beta2. The FromPoolRef field changed from pointer to value type.
func Convert_v1beta1_NetworkGatewayv4_To_v1beta2_NetworkGatewayv4(in *NetworkGatewayv4, out *infrav1.NetworkGatewayv4, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_NetworkGatewayv4_To_v1beta2_NetworkGatewayv4(in, out, s); err != nil {
		return err
	}
	if err := convertPoolRefPointerToValue(in.FromPoolRef, &out.FromPoolRef, s); err != nil {
		return err
	}
	convertFromPoolAnnotationPointerToValue(in.FromPoolAnnotation, &out.FromPoolAnnotation)
	return nil
}

// Convert_v1beta2_NetworkGatewayv4_To_v1beta1_NetworkGatewayv4 handles conversion
// of NetworkGatewayv4 from v1beta2 to v1beta1. The FromPoolRef field changed from value to pointer type.
func Convert_v1beta2_NetworkGatewayv4_To_v1beta1_NetworkGatewayv4(in *infrav1.NetworkGatewayv4, out *NetworkGatewayv4, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_NetworkGatewayv4_To_v1beta1_NetworkGatewayv4(in, out, s); err != nil {
		return err
	}
	var err error
	out.FromPoolRef, err = convertPoolRefValueToPointer(&in.FromPoolRef, s)
	if err != nil {
		return err
	}
	out.FromPoolAnnotation = convertFromPoolAnnotationValueToPointer(&in.FromPoolAnnotation)
	return nil
}

// Convert_v1beta1_NetworkGatewayv6_To_v1beta2_NetworkGatewayv6 handles conversion
// of NetworkGatewayv6 from v1beta1 to v1beta2. The FromPoolRef field changed from pointer to value type.
func Convert_v1beta1_NetworkGatewayv6_To_v1beta2_NetworkGatewayv6(in *NetworkGatewayv6, out *infrav1.NetworkGatewayv6, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_NetworkGatewayv6_To_v1beta2_NetworkGatewayv6(in, out, s); err != nil {
		return err
	}
	if err := convertPoolRefPointerToValue(in.FromPoolRef, &out.FromPoolRef, s); err != nil {
		return err
	}
	convertFromPoolAnnotationPointerToValue(in.FromPoolAnnotation, &out.FromPoolAnnotation)
	return nil
}

// Convert_v1beta2_NetworkGatewayv6_To_v1beta1_NetworkGatewayv6 handles conversion
// of NetworkGatewayv6 from v1beta2 to v1beta1. The FromPoolRef field changed from value to pointer type.
func Convert_v1beta2_NetworkGatewayv6_To_v1beta1_NetworkGatewayv6(in *infrav1.NetworkGatewayv6, out *NetworkGatewayv6, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_NetworkGatewayv6_To_v1beta1_NetworkGatewayv6(in, out, s); err != nil {
		return err
	}
	var err error
	out.FromPoolRef, err = convertPoolRefValueToPointer(&in.FromPoolRef, s)
	if err != nil {
		return err
	}
	out.FromPoolAnnotation = convertFromPoolAnnotationValueToPointer(&in.FromPoolAnnotation)
	return nil
}

// Convert_v1beta1_NetworkDataIPv4_To_v1beta2_NetworkDataIPv4 handles conversion
// of NetworkDataIPv4 from v1beta1 to v1beta2. The FromPoolRef field changed from pointer to value type.
func Convert_v1beta1_NetworkDataIPv4_To_v1beta2_NetworkDataIPv4(in *NetworkDataIPv4, out *infrav1.NetworkDataIPv4, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_NetworkDataIPv4_To_v1beta2_NetworkDataIPv4(in, out, s); err != nil {
		return err
	}
	if err := convertPoolRefPointerToValue(in.FromPoolRef, &out.FromPoolRef, s); err != nil {
		return err
	}
	convertFromPoolAnnotationPointerToValue(in.FromPoolAnnotation, &out.FromPoolAnnotation)
	return nil
}

// Convert_v1beta2_NetworkDataIPv4_To_v1beta1_NetworkDataIPv4 handles conversion
// of NetworkDataIPv4 from v1beta2 to v1beta1. The FromPoolRef field changed from value to pointer type.
func Convert_v1beta2_NetworkDataIPv4_To_v1beta1_NetworkDataIPv4(in *infrav1.NetworkDataIPv4, out *NetworkDataIPv4, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_NetworkDataIPv4_To_v1beta1_NetworkDataIPv4(in, out, s); err != nil {
		return err
	}
	var err error
	out.FromPoolRef, err = convertPoolRefValueToPointer(&in.FromPoolRef, s)
	if err != nil {
		return err
	}
	out.FromPoolAnnotation = convertFromPoolAnnotationValueToPointer(&in.FromPoolAnnotation)
	return nil
}

// Convert_v1beta1_NetworkDataIPv6_To_v1beta2_NetworkDataIPv6 handles conversion
// of NetworkDataIPv6 from v1beta1 to v1beta2. The FromPoolRef field changed from pointer to value type.
func Convert_v1beta1_NetworkDataIPv6_To_v1beta2_NetworkDataIPv6(in *NetworkDataIPv6, out *infrav1.NetworkDataIPv6, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_NetworkDataIPv6_To_v1beta2_NetworkDataIPv6(in, out, s); err != nil {
		return err
	}
	if err := convertPoolRefPointerToValue(in.FromPoolRef, &out.FromPoolRef, s); err != nil {
		return err
	}
	convertFromPoolAnnotationPointerToValue(in.FromPoolAnnotation, &out.FromPoolAnnotation)
	return nil
}

// Convert_v1beta2_NetworkDataIPv6_To_v1beta1_NetworkDataIPv6 handles conversion
// of NetworkDataIPv6 from v1beta2 to v1beta1. The FromPoolRef field changed from value to pointer type.
func Convert_v1beta2_NetworkDataIPv6_To_v1beta1_NetworkDataIPv6(in *infrav1.NetworkDataIPv6, out *NetworkDataIPv6, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_NetworkDataIPv6_To_v1beta1_NetworkDataIPv6(in, out, s); err != nil {
		return err
	}
	var err error
	out.FromPoolRef, err = convertPoolRefValueToPointer(&in.FromPoolRef, s)
	if err != nil {
		return err
	}
	out.FromPoolAnnotation = convertFromPoolAnnotationValueToPointer(&in.FromPoolAnnotation)
	return nil
}

// Convert_v1beta1_NetworkData_To_v1beta2_NetworkData handles conversion
// of NetworkData from v1beta1 to v1beta2. The Links, Networks, and Services fields changed from value to pointer types.
func Convert_v1beta1_NetworkData_To_v1beta2_NetworkData(in *NetworkData, out *infrav1.NetworkData, s apimachineryconversion.Scope) error {
	// Links: value to pointer, only allocate if non-empty
	if !reflect.DeepEqual(in.Links, NetworkDataLink{}) {
		out.Links = &infrav1.NetworkDataLink{}
		if err := Convert_v1beta1_NetworkDataLink_To_v1beta2_NetworkDataLink(&in.Links, out.Links, s); err != nil {
			return err
		}
	}
	// Networks: value to pointer, only allocate if non-empty
	if !reflect.DeepEqual(in.Networks, NetworkDataNetwork{}) {
		out.Networks = &infrav1.NetworkDataNetwork{}
		if err := Convert_v1beta1_NetworkDataNetwork_To_v1beta2_NetworkDataNetwork(&in.Networks, out.Networks, s); err != nil {
			return err
		}
	}
	// Services: value to pointer, only allocate if non-empty
	if !reflect.DeepEqual(in.Services, NetworkDataService{}) {
		out.Services = &infrav1.NetworkDataService{}
		if err := Convert_v1beta1_NetworkDataService_To_v1beta2_NetworkDataService(&in.Services, out.Services, s); err != nil {
			return err
		}
	}
	return nil
}

// Convert_v1beta2_NetworkData_To_v1beta1_NetworkData handles conversion
// of NetworkData from v1beta2 to v1beta1. The Links, Networks, and Services fields changed from pointer to value types.
func Convert_v1beta2_NetworkData_To_v1beta1_NetworkData(in *infrav1.NetworkData, out *NetworkData, s apimachineryconversion.Scope) error {
	// Links: pointer to value
	if in.Links != nil {
		if err := Convert_v1beta2_NetworkDataLink_To_v1beta1_NetworkDataLink(in.Links, &out.Links, s); err != nil {
			return err
		}
	}
	// Networks: pointer to value
	if in.Networks != nil {
		if err := Convert_v1beta2_NetworkDataNetwork_To_v1beta1_NetworkDataNetwork(in.Networks, &out.Networks, s); err != nil {
			return err
		}
	}
	// Services: pointer to value
	if in.Services != nil {
		if err := Convert_v1beta2_NetworkDataService_To_v1beta1_NetworkDataService(in.Services, &out.Services, s); err != nil {
			return err
		}
	}
	return nil
}

// Convert_v1beta1_NetworkDataService_To_v1beta2_NetworkDataService is an autogenerated conversion function.
func Convert_v1beta1_NetworkDataService_To_v1beta2_NetworkDataService(in *NetworkDataService, out *infrav1.NetworkDataService, s apimachineryconversion.Scope) error {
	return autoConvert_v1beta1_NetworkDataService_To_v1beta2_NetworkDataService(in, out, s)
}

// Convert_v1beta2_NetworkDataService_To_v1beta1_NetworkDataService converts v1beta2 NetworkDataService (string DNSFromIPPool)
// to v1beta1 NetworkDataService (*string DNSFromIPPool), ensuring empty strings map to nil pointers.
func Convert_v1beta2_NetworkDataService_To_v1beta1_NetworkDataService(in *infrav1.NetworkDataService, out *NetworkDataService, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_NetworkDataService_To_v1beta1_NetworkDataService(in, out, s); err != nil {
		return err
	}
	// Empty strings should convert to nil pointers, not &"", since v1beta1 uses nil to represent "not set"
	if in.DNSFromIPPool == "" && out.DNSFromIPPool != nil && *out.DNSFromIPPool == "" {
		out.DNSFromIPPool = nil
	}
	return nil
}

// Convert_v1beta1_NetworkDataServicev4_To_v1beta2_NetworkDataServicev4 is an autogenerated conversion function.
func Convert_v1beta1_NetworkDataServicev4_To_v1beta2_NetworkDataServicev4(in *NetworkDataServicev4, out *infrav1.NetworkDataServicev4, s apimachineryconversion.Scope) error {
	return autoConvert_v1beta1_NetworkDataServicev4_To_v1beta2_NetworkDataServicev4(in, out, s)
}

// Convert_v1beta2_NetworkDataServicev4_To_v1beta1_NetworkDataServicev4 converts v1beta2 NetworkDataServicev4 (string DNSFromIPPool)
// to v1beta1 NetworkDataServicev4 (*string DNSFromIPPool), ensuring empty strings map to nil pointers.
func Convert_v1beta2_NetworkDataServicev4_To_v1beta1_NetworkDataServicev4(in *infrav1.NetworkDataServicev4, out *NetworkDataServicev4, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_NetworkDataServicev4_To_v1beta1_NetworkDataServicev4(in, out, s); err != nil {
		return err
	}
	// Empty strings should convert to nil pointers, not &"", since v1beta1 uses nil to represent "not set"
	if in.DNSFromIPPool == "" && out.DNSFromIPPool != nil && *out.DNSFromIPPool == "" {
		out.DNSFromIPPool = nil
	}
	return nil
}

// Convert_v1beta1_NetworkDataServicev6_To_v1beta2_NetworkDataServicev6 is an autogenerated conversion function.
func Convert_v1beta1_NetworkDataServicev6_To_v1beta2_NetworkDataServicev6(in *NetworkDataServicev6, out *infrav1.NetworkDataServicev6, s apimachineryconversion.Scope) error {
	return autoConvert_v1beta1_NetworkDataServicev6_To_v1beta2_NetworkDataServicev6(in, out, s)
}

// Convert_v1beta2_NetworkDataServicev6_To_v1beta1_NetworkDataServicev6 converts v1beta2 NetworkDataServicev6 (string DNSFromIPPool)
// to v1beta1 NetworkDataServicev6 (*string DNSFromIPPool), ensuring empty strings map to nil pointers.
func Convert_v1beta2_NetworkDataServicev6_To_v1beta1_NetworkDataServicev6(in *infrav1.NetworkDataServicev6, out *NetworkDataServicev6, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_NetworkDataServicev6_To_v1beta1_NetworkDataServicev6(in, out, s); err != nil {
		return err
	}
	// Empty strings should convert to nil pointers, not &"", since v1beta1 uses nil to represent "not set"
	if in.DNSFromIPPool == "" && out.DNSFromIPPool != nil && *out.DNSFromIPPool == "" {
		out.DNSFromIPPool = nil
	}
	return nil
}
