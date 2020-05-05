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

package v1alpha4

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DataFinalizer allows Metal3IPAddressReconciler to clean up resources
	// associated with Metal3IPAddress before removing it from the apiserver.
	IPAddressFinalizer = "metal3ipaddress.infrastructure.cluster.x-k8s.io"
)

// Metal3IPAddressSpec defines the desired state of Metal3IPAddress.
type Metal3IPAddressSpec struct {

	// Owner points to the object the Metal3IPAddress was created for.
	Owner *corev1.ObjectReference `json:"owner,omitempty"`

	// IPPool is the Metal3IPPool this was generated from.
	IPPool *corev1.ObjectReference `json:"ipPool,omitempty"`

	// +kubebuilder:validation:Maximum=128
	// Prefix is the mask of the network as integer (max 128)
	Prefix int `json:"prefix,omitempty"`

	// Gateway is the gateway ip address
	Gateway *IPAddress `json:"gateway,omitempty"`

	// Address contains the IP address
	Address IPAddress `json:"address"`
}

// Metal3IPAddressStatus defines the observed state of Metal3IPAddress.
type Metal3IPAddressStatus struct {
	// Ready is a flag set to True if the resource is ready for use
	Ready bool `json:"ready,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3ipaddresses,scope=Namespaced,categories=cluster-api,shortName=m3ipa;m3ipaddress;metal3ipaddress
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// Metal3IPAddress is the Schema for the metal3ipaddresses API
type Metal3IPAddress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Metal3IPAddressSpec   `json:"spec,omitempty"`
	Status Metal3IPAddressStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3IPAddressList contains a list of Metal3IPAddress
type Metal3IPAddressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3IPAddress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Metal3IPAddress{}, &Metal3IPAddressList{})
}
