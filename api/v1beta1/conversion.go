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
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
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
