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
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"sort"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func (src *Metal3Cluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3Cluster)
	if err := Convert_v1beta1_Metal3Cluster_To_v1beta2_Metal3Cluster(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3Cluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3Cluster)
	if err := Convert_v1beta2_Metal3Cluster_To_v1beta1_Metal3Cluster(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3ClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3ClusterTemplate)
	if err := Convert_v1beta1_Metal3ClusterTemplate_To_v1beta2_Metal3ClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3ClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3ClusterTemplate)
	if err := Convert_v1beta2_Metal3ClusterTemplate_To_v1beta1_Metal3ClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3Machine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3Machine)
	if err := Convert_v1beta1_Metal3Machine_To_v1beta2_Metal3Machine(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3Machine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3Machine)
	if err := Convert_v1beta2_Metal3Machine_To_v1beta1_Metal3Machine(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3MachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3MachineTemplate)
	if err := Convert_v1beta1_Metal3MachineTemplate_To_v1beta2_Metal3MachineTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3MachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3MachineTemplate)
	if err := Convert_v1beta2_Metal3MachineTemplate_To_v1beta1_Metal3MachineTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3DataTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3DataTemplate)
	if err := Convert_v1beta1_Metal3DataTemplate_To_v1beta2_Metal3DataTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3DataTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3DataTemplate)
	if err := Convert_v1beta2_Metal3DataTemplate_To_v1beta1_Metal3DataTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3Data) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3Data)
	if err := Convert_v1beta1_Metal3Data_To_v1beta2_Metal3Data(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3Data) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3Data)
	if err := Convert_v1beta2_Metal3Data_To_v1beta1_Metal3Data(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3DataClaim) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.Metal3DataClaim)
	if err := Convert_v1beta1_Metal3DataClaim_To_v1beta2_Metal3DataClaim(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3DataClaim) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.Metal3DataClaim)
	if err := Convert_v1beta2_Metal3DataClaim_To_v1beta1_Metal3DataClaim(src, dst, nil); err != nil {
		return err
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

	out.Ready = in.Ready

	// Move new conditions (v1beta2) to the v1beta2 field.
	if in.Conditions == nil {
		return nil
	}
	out.V1Beta2 = &Metal3MachineV1Beta2Status{}
	out.V1Beta2.Conditions = in.Conditions
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
	if in.FailureReason == nil && in.FailureMessage == nil && in.Conditions == nil {
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

func Convert_v1beta2_Metal3ClusterTemplateResource_To_v1beta1_Metal3ClusterTemplateResource(in *infrav1.Metal3ClusterTemplateResource, out *Metal3ClusterTemplateResource, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_Metal3ClusterTemplateResource_To_v1beta1_Metal3ClusterTemplateResource(in, out, s); err != nil {
		return err
	}

	return nil
}
