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
	"fmt"
	"github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"net/url"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"strconv"
)

//Constant variables
const (
	APIEndpointPort = "6443"
)

func (src *Metal3Cluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.Metal3Cluster)
	return Convert_v1alpha2_Metal3Cluster_To_v1alpha4_Metal3Cluster(src, dst, nil)
}

func (dst *Metal3Cluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3Cluster)
	if err := Convert_v1alpha4_Metal3Cluster_To_v1alpha2_Metal3Cluster(src, dst, nil); err != nil {
		return err
	}

	// Fill the APIEndpoint
	dst.Status.APIEndpoints = []APIEndpoint{
		APIEndpoint{
			Host: src.Spec.ControlPlaneEndpoint.Host,
			Port: src.Spec.ControlPlaneEndpoint.Port,
		},
	}
	return nil
}

func (src *Metal3ClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.Metal3ClusterList)

	return Convert_v1alpha2_Metal3ClusterList_To_v1alpha4_Metal3ClusterList(src, dst, nil)
}

func (dst *Metal3ClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3ClusterList)

	return Convert_v1alpha4_Metal3ClusterList_To_v1alpha2_Metal3ClusterList(src, dst, nil)
}

func (src *Metal3Machine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.Metal3Machine)
	if err := Convert_v1alpha2_Metal3Machine_To_v1alpha4_Metal3Machine(src, dst, nil); err != nil {
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
	dst.Status.UserData = restored.Status.UserData
	dst.Status.MetaData = restored.Status.MetaData
	dst.Status.NetworkData = restored.Status.NetworkData
	dst.Status.RenderedData = restored.Status.RenderedData

	return nil
}

func (dst *Metal3Machine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3Machine)
	if err := Convert_v1alpha4_Metal3Machine_To_v1alpha2_Metal3Machine(src, dst, nil); err != nil {
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

	return Convert_v1alpha2_Metal3MachineList_To_v1alpha4_Metal3MachineList(src, dst, nil)
}

func (dst *Metal3MachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3MachineList)

	return Convert_v1alpha4_Metal3MachineList_To_v1alpha2_Metal3MachineList(src, dst, nil)
}

func (src *Metal3MachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.Metal3MachineTemplate)
	if err := Convert_v1alpha2_Metal3MachineTemplate_To_v1alpha4_Metal3MachineTemplate(src, dst, nil); err != nil {
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

	return nil
}

func (dst *Metal3MachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3MachineTemplate)
	if err := Convert_v1alpha4_Metal3MachineTemplate_To_v1alpha2_Metal3MachineTemplate(src, dst, nil); err != nil {
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

	return Convert_v1alpha2_Metal3MachineTemplateList_To_v1alpha4_Metal3MachineTemplateList(src, dst, nil)
}

func (dst *Metal3MachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.Metal3MachineTemplateList)

	return Convert_v1alpha4_Metal3MachineTemplateList_To_v1alpha2_Metal3MachineTemplateList(src, dst, nil)
}

func Convert_v1alpha2_Metal3ClusterStatus_To_v1alpha4_Metal3ClusterStatus(in *Metal3ClusterStatus, out *v1alpha4.Metal3ClusterStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha2_Metal3ClusterStatus_To_v1alpha4_Metal3ClusterStatus(in, out, s); err != nil {
		return err
	}
	// Manually convert the Failure fields to the Error fields
	out.FailureMessage = in.ErrorMessage
	out.FailureReason = in.ErrorReason

	return nil
}

func Convert_v1alpha4_Metal3ClusterStatus_To_v1alpha2_Metal3ClusterStatus(in *v1alpha4.Metal3ClusterStatus, out *Metal3ClusterStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_Metal3ClusterStatus_To_v1alpha2_Metal3ClusterStatus(in, out, s); err != nil {
		return err
	}
	// Manually convert the Failure fields to the Error fields
	out.ErrorMessage = in.FailureMessage
	out.ErrorReason = in.FailureReason

	return nil
}

func Convert_v1alpha2_Metal3ClusterSpec_To_v1alpha4_Metal3ClusterSpec(in *Metal3ClusterSpec, out *v1alpha4.Metal3ClusterSpec, s apiconversion.Scope) error {
	var err error
	if err = autoConvert_v1alpha2_Metal3ClusterSpec_To_v1alpha4_Metal3ClusterSpec(in, out, s); err != nil {
		return err
	}

	endPoint := in.APIEndpoint

	// Parse
	u, err := url.Parse(endPoint)
	if err != nil {
		return err
	}

	ip := u.Hostname()
	p := u.Port()

	if p == "" {
		p = APIEndpointPort
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return err
	}

	out.ControlPlaneEndpoint = v1alpha4.APIEndpoint{
		Host: ip,
		Port: port,
	}
	return nil
}

func Convert_v1alpha4_Metal3ClusterSpec_To_v1alpha2_Metal3ClusterSpec(in *v1alpha4.Metal3ClusterSpec, out *Metal3ClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_Metal3ClusterSpec_To_v1alpha2_Metal3ClusterSpec(in, out, s); err != nil {
		return err
	}

	out.APIEndpoint = fmt.Sprintf("https://%s:%d", in.ControlPlaneEndpoint.Host,
		in.ControlPlaneEndpoint.Port,
	)

	return nil
}

func Convert_v1alpha2_Metal3MachineStatus_To_v1alpha4_Metal3MachineStatus(in *Metal3MachineStatus, out *v1alpha4.Metal3MachineStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha2_Metal3MachineStatus_To_v1alpha4_Metal3MachineStatus(in, out, s); err != nil {
		return err
	}
	// Manually convert the Failure fields to the Error fields
	out.FailureMessage = in.ErrorMessage
	out.FailureReason = in.ErrorReason

	return nil
}

func Convert_v1alpha4_Metal3MachineStatus_To_v1alpha2_Metal3MachineStatus(in *v1alpha4.Metal3MachineStatus, out *Metal3MachineStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_Metal3MachineStatus_To_v1alpha2_Metal3MachineStatus(in, out, s); err != nil {
		return err
	}
	// Manually convert the Failure fields to the Error fields
	out.ErrorMessage = in.FailureMessage
	out.ErrorReason = in.FailureReason

	return nil
}

func Convert_v1alpha4_Metal3MachineSpec_To_v1alpha2_Metal3MachineSpec(in *v1alpha4.Metal3MachineSpec, out *Metal3MachineSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_Metal3MachineSpec_To_v1alpha2_Metal3MachineSpec(in, out, s); err != nil {
		return err
	}

	// Discards unused ObjectMeta

	return nil
}
