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
	"github.com/metal3-io/cluster-api-provider-baremetal/api/v1alpha3"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"strconv"
)

//Constant variables
const (
	APIEndpointPort = "6443"
)

func (src *BareMetalCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha3.BareMetalCluster)
	return Convert_v1alpha2_BareMetalCluster_To_v1alpha3_BareMetalCluster(src, dst, nil)
}

func (dst *BareMetalCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha3.BareMetalCluster)
	if err := Convert_v1alpha3_BareMetalCluster_To_v1alpha2_BareMetalCluster(src, dst, nil); err != nil {
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

func (src *BareMetalClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha3.BareMetalClusterList)

	return Convert_v1alpha2_BareMetalClusterList_To_v1alpha3_BareMetalClusterList(src, dst, nil)
}

func (dst *BareMetalClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha3.BareMetalClusterList)

	return Convert_v1alpha3_BareMetalClusterList_To_v1alpha2_BareMetalClusterList(src, dst, nil)
}

func (src *BareMetalMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha3.BareMetalMachine)
	return Convert_v1alpha2_BareMetalMachine_To_v1alpha3_BareMetalMachine(src, dst, nil)
}

func (dst *BareMetalMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha3.BareMetalMachine)
	return Convert_v1alpha3_BareMetalMachine_To_v1alpha2_BareMetalMachine(src, dst, nil)
}

func (src *BareMetalMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha3.BareMetalMachineList)

	return Convert_v1alpha2_BareMetalMachineList_To_v1alpha3_BareMetalMachineList(src, dst, nil)
}

func (dst *BareMetalMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha3.BareMetalMachineList)

	return Convert_v1alpha3_BareMetalMachineList_To_v1alpha2_BareMetalMachineList(src, dst, nil)
}

func Convert_v1alpha2_BareMetalClusterStatus_To_v1alpha3_BareMetalClusterStatus(in *BareMetalClusterStatus, out *v1alpha3.BareMetalClusterStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha2_BareMetalClusterStatus_To_v1alpha3_BareMetalClusterStatus(in, out, s); err != nil {
		return err
	}
	// Manually convert the Failure fields to the Error fields
	out.FailureMessage = in.ErrorMessage
	out.FailureReason = in.ErrorReason

	return nil
}

func Convert_v1alpha3_BareMetalClusterStatus_To_v1alpha2_BareMetalClusterStatus(in *v1alpha3.BareMetalClusterStatus, out *BareMetalClusterStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha3_BareMetalClusterStatus_To_v1alpha2_BareMetalClusterStatus(in, out, s); err != nil {
		return err
	}
	// Manually convert the Failure fields to the Error fields
	out.ErrorMessage = in.FailureMessage
	out.ErrorReason = in.FailureReason

	return nil
}

func Convert_v1alpha2_BareMetalClusterSpec_To_v1alpha3_BareMetalClusterSpec(in *BareMetalClusterSpec, out *v1alpha3.BareMetalClusterSpec, s apiconversion.Scope) error {
	var err error
	if err = autoConvert_v1alpha2_BareMetalClusterSpec_To_v1alpha3_BareMetalClusterSpec(in, out, s); err != nil {
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

	out.ControlPlaneEndpoint = v1alpha3.APIEndpoint{
		Host: ip,
		Port: port,
	}
	return nil
}

func Convert_v1alpha3_BareMetalClusterSpec_To_v1alpha2_BareMetalClusterSpec(in *v1alpha3.BareMetalClusterSpec, out *BareMetalClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha3_BareMetalClusterSpec_To_v1alpha2_BareMetalClusterSpec(in, out, s); err != nil {
		return err
	}

	out.APIEndpoint = fmt.Sprintf("https://%s:%d", in.ControlPlaneEndpoint.Host,
		in.ControlPlaneEndpoint.Port,
	)

	return nil
}

func Convert_v1alpha2_BareMetalMachineStatus_To_v1alpha3_BareMetalMachineStatus(in *BareMetalMachineStatus, out *v1alpha3.BareMetalMachineStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha2_BareMetalMachineStatus_To_v1alpha3_BareMetalMachineStatus(in, out, s); err != nil {
		return err
	}
	// Manually convert the Failure fields to the Error fields
	out.FailureMessage = in.ErrorMessage
	out.FailureReason = in.ErrorReason

	return nil
}

func Convert_v1alpha3_BareMetalMachineStatus_To_v1alpha2_BareMetalMachineStatus(in *v1alpha3.BareMetalMachineStatus, out *BareMetalMachineStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha3_BareMetalMachineStatus_To_v1alpha2_BareMetalMachineStatus(in, out, s); err != nil {
		return err
	}
	// Manually convert the Failure fields to the Error fields
	out.ErrorMessage = in.FailureMessage
	out.ErrorReason = in.FailureReason

	return nil
}
