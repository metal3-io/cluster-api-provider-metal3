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

package v1alpha3

import (
	"github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

//Constant variables
const (
	APIEndpointPort = "6443"
)

func (src *Metal3Cluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.Metal3Cluster)
	return Convert_v1alpha3_Metal3Cluster_To_v1alpha4_Metal3Cluster(src, dst, nil)
}

func (dst *Metal3Cluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3Cluster)
	return Convert_v1alpha4_Metal3Cluster_To_v1alpha3_Metal3Cluster(src, dst, nil)
}

func (src *Metal3ClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.Metal3ClusterList)
	return Convert_v1alpha3_Metal3ClusterList_To_v1alpha4_Metal3ClusterList(src, dst, nil)
}

func (dst *Metal3ClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3ClusterList)
	return Convert_v1alpha4_Metal3ClusterList_To_v1alpha3_Metal3ClusterList(src, dst, nil)
}

func (src *Metal3Machine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.Metal3Machine)
	if err := Convert_v1alpha3_Metal3Machine_To_v1alpha4_Metal3Machine(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &v1alpha4.Metal3Machine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.MetaData = restored.Spec.MetaData
	dst.Spec.NetworkData = restored.Spec.NetworkData
	dst.Spec.DataTemplate = restored.Spec.DataTemplate
	dst.Spec.Image = restored.Spec.Image
	dst.Status.UserData = restored.Status.UserData
	dst.Status.MetaData = restored.Status.MetaData
	dst.Status.NetworkData = restored.Status.NetworkData
	dst.Status.RenderedData = restored.Status.RenderedData

	return nil
}

func (dst *Metal3Machine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3Machine)
	if err := Convert_v1alpha4_Metal3Machine_To_v1alpha3_Metal3Machine(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion except for metadata
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

func (src *Metal3MachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.Metal3MachineList)
	return Convert_v1alpha3_Metal3MachineList_To_v1alpha4_Metal3MachineList(src, dst, nil)
}

func (dst *Metal3MachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3MachineList)
	return Convert_v1alpha4_Metal3MachineList_To_v1alpha3_Metal3MachineList(src, dst, nil)
}

func (src *Metal3MachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.Metal3MachineTemplate)
	if err := Convert_v1alpha3_Metal3MachineTemplate_To_v1alpha4_Metal3MachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &v1alpha4.Metal3MachineTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.Template.Spec.MetaData = restored.Spec.Template.Spec.MetaData
	dst.Spec.Template.Spec.NetworkData = restored.Spec.Template.Spec.NetworkData
	dst.Spec.Template.Spec.DataTemplate = restored.Spec.Template.Spec.DataTemplate
	dst.Spec.Template.Spec.Image = restored.Spec.Template.Spec.Image
	dst.Spec.NodeReuse = restored.Spec.NodeReuse

	return nil
}

func (dst *Metal3MachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3MachineTemplate)
	if err := Convert_v1alpha4_Metal3MachineTemplate_To_v1alpha3_Metal3MachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion except for metadata
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

func (src *Metal3MachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.Metal3MachineTemplateList)
	return Convert_v1alpha3_Metal3MachineTemplateList_To_v1alpha4_Metal3MachineTemplateList(src, dst, nil)
}

func (dst *Metal3MachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3MachineTemplateList)
	return Convert_v1alpha4_Metal3MachineTemplateList_To_v1alpha3_Metal3MachineTemplateList(src, dst, nil)
}

func Convert_v1alpha4_Metal3MachineSpec_To_v1alpha3_Metal3MachineSpec(in *v1alpha4.Metal3MachineSpec, out *Metal3MachineSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_Metal3MachineSpec_To_v1alpha3_Metal3MachineSpec(in, out, s); err != nil {
		return err
	}

	// Discards unused ObjectMeta

	return nil
}

func Convert_v1alpha4_Metal3MachineStatus_To_v1alpha3_Metal3MachineStatus(in *v1alpha4.Metal3MachineStatus, out *Metal3MachineStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_Metal3MachineStatus_To_v1alpha3_Metal3MachineStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

func Convert_v1alpha4_Image_To_v1alpha3_Image(in *v1alpha4.Image, out *Image, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_Image_To_v1alpha3_Image(in, out, s); err != nil {
		return err
	}

	return nil
}

func Convert_v1alpha4_Metal3MachineTemplateSpec_To_v1alpha3_Metal3MachineTemplateSpec(in *v1alpha4.Metal3MachineTemplateSpec, out *Metal3MachineTemplateSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_Metal3MachineTemplateSpec_To_v1alpha3_Metal3MachineTemplateSpec(in, out, s); err != nil {
		return err
	}

	return nil
}
