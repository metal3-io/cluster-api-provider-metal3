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

package v1alpha4

import (
	"github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha5"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

//Constant variables
const (
	APIEndpointPort = "6443"
)

func (src *Metal3Cluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3Cluster)
	return Convert_v1alpha4_Metal3Cluster_To_v1alpha5_Metal3Cluster(src, dst, nil)
}

func (dst *Metal3Cluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3Cluster)
	return Convert_v1alpha5_Metal3Cluster_To_v1alpha4_Metal3Cluster(src, dst, nil)
}

func (src *Metal3ClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3ClusterList)
	return Convert_v1alpha4_Metal3ClusterList_To_v1alpha5_Metal3ClusterList(src, dst, nil)
}

func (dst *Metal3ClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3ClusterList)
	return Convert_v1alpha5_Metal3ClusterList_To_v1alpha4_Metal3ClusterList(src, dst, nil)
}

func (src *Metal3Machine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3Machine)
	if err := Convert_v1alpha4_Metal3Machine_To_v1alpha5_Metal3Machine(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3Machine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3Machine)
	if err := Convert_v1alpha5_Metal3Machine_To_v1alpha4_Metal3Machine(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3MachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3MachineList)
	return Convert_v1alpha4_Metal3MachineList_To_v1alpha5_Metal3MachineList(src, dst, nil)
}

func (dst *Metal3MachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3MachineList)
	return Convert_v1alpha5_Metal3MachineList_To_v1alpha4_Metal3MachineList(src, dst, nil)
}

func (src *Metal3MachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3MachineTemplate)
	if err := Convert_v1alpha4_Metal3MachineTemplate_To_v1alpha5_Metal3MachineTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3MachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3MachineTemplate)
	if err := Convert_v1alpha5_Metal3MachineTemplate_To_v1alpha4_Metal3MachineTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3MachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3MachineTemplateList)
	return Convert_v1alpha4_Metal3MachineTemplateList_To_v1alpha5_Metal3MachineTemplateList(src, dst, nil)
}

func (dst *Metal3MachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3MachineTemplateList)
	return Convert_v1alpha5_Metal3MachineTemplateList_To_v1alpha4_Metal3MachineTemplateList(src, dst, nil)
}

func (src *Metal3Data) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3Data)
	if err := Convert_v1alpha4_Metal3Data_To_v1alpha5_Metal3Data(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3Data) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3Data)
	if err := Convert_v1alpha5_Metal3Data_To_v1alpha4_Metal3Data(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3DataList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3DataList)
	return Convert_v1alpha4_Metal3DataList_To_v1alpha5_Metal3DataList(src, dst, nil)
}

func (dst *Metal3DataList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3DataList)
	return Convert_v1alpha5_Metal3DataList_To_v1alpha4_Metal3DataList(src, dst, nil)
}

func (src *Metal3DataTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3DataTemplate)
	if err := Convert_v1alpha4_Metal3DataTemplate_To_v1alpha5_Metal3DataTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3DataTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3DataTemplate)
	if err := Convert_v1alpha5_Metal3DataTemplate_To_v1alpha4_Metal3DataTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3DataTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3DataTemplateList)
	return Convert_v1alpha4_Metal3DataTemplateList_To_v1alpha5_Metal3DataTemplateList(src, dst, nil)
}

func (dst *Metal3DataTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3DataTemplateList)
	return Convert_v1alpha5_Metal3DataTemplateList_To_v1alpha4_Metal3DataTemplateList(src, dst, nil)
}

func (src *Metal3DataClaim) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3DataClaim)
	if err := Convert_v1alpha4_Metal3DataClaim_To_v1alpha5_Metal3DataClaim(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *Metal3DataClaim) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3DataClaim)
	if err := Convert_v1alpha5_Metal3DataClaim_To_v1alpha4_Metal3DataClaim(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *Metal3DataClaimList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha5.Metal3DataClaimList)
	return Convert_v1alpha4_Metal3DataClaimList_To_v1alpha5_Metal3DataClaimList(src, dst, nil)
}

func (dst *Metal3DataClaimList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha5.Metal3DataClaimList)
	return Convert_v1alpha5_Metal3DataClaimList_To_v1alpha4_Metal3DataClaimList(src, dst, nil)
}
