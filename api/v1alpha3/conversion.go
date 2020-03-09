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
	return Convert_v1alpha3_Metal3Machine_To_v1alpha4_Metal3Machine(src, dst, nil)
}

func (dst *Metal3Machine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3Machine)
	return Convert_v1alpha4_Metal3Machine_To_v1alpha3_Metal3Machine(src, dst, nil)
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
	return Convert_v1alpha3_Metal3MachineTemplate_To_v1alpha4_Metal3MachineTemplate(src, dst, nil)
}

func (dst *Metal3MachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3MachineTemplate)
	return Convert_v1alpha4_Metal3MachineTemplate_To_v1alpha3_Metal3MachineTemplate(src, dst, nil)
}

func (src *Metal3MachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.Metal3MachineTemplateList)
	return Convert_v1alpha3_Metal3MachineTemplateList_To_v1alpha4_Metal3MachineTemplateList(src, dst, nil)
}

func (dst *Metal3MachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3MachineTemplateList)
	return Convert_v1alpha4_Metal3MachineTemplateList_To_v1alpha3_Metal3MachineTemplateList(src, dst, nil)
}
