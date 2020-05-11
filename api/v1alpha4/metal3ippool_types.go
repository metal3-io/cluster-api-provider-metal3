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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// IPPoolFinalizer allows Metal3IPPoolReconciler to clean up resources
	// associated with Metal3IPPool before removing it from the apiserver.
	IPPoolFinalizer = "metal3ippool.infrastructure.cluster.x-k8s.io"
)

// MetaDataIPAddress contains the info to render th ip address. It is IP-version
// agnostic
type IPPool struct {

	// Start is the first ip address that can be rendered
	Start *IPAddress `json:"start,omitempty"`

	// End is the last IP address that can be rendered. It is used as a validation
	// that the rendered IP is in bound.
	End *IPAddress `json:"end,omitempty"`

	// Subnet is used to validate that the rendered IP is in bounds. In case the
	// Start value is not given, it is derived from the subnet ip incremented by 1
	// (`192.168.0.1` for `192.168.0.0/24`)
	Subnet *IPSubnet `json:"subnet,omitempty"`

	// +kubebuilder:validation:Maximum=128
	// Prefix is the mask of the network as integer (max 128)
	Prefix int `json:"prefix,omitempty"`

	// Gateway is the gateway ip address
	Gateway *IPAddress `json:"gateway,omitempty"`
}

// Metal3IPPoolSpec defines the desired state of Metal3IPPool.
type Metal3IPPoolSpec struct {

	// ClusterName is the name of the Cluster this object belongs to.
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName"`

	//Pools contains the list of IP addresses pools
	Pools []IPPool `json:"pools,omitempty"`

	// PreAllocations contains the preallocated IP addresses
	PreAllocations map[string]IPAddress `json:"preAllocations,omitempty"`

	// +kubebuilder:validation:Maximum=128
	// Prefix is the mask of the network as integer (max 128)
	Prefix int `json:"prefix,omitempty"`

	// Gateway is the gateway ip address
	Gateway *IPAddress `json:"gateway,omitempty"`

	// +kubebuilder:validation:MinLength=1
	// namePrefix is the prefix used to generate the Metal3IPAddress object names
	NamePrefix string `json:"namePrefix"`
}

// Metal3IPPoolStatus defines the observed state of Metal3IPPool.
type Metal3IPPoolStatus struct {
	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	//Allocations contains the map of objects and IP addresses they have
	Allocations map[string]IPAddress `json:"indexes,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3ippools,scope=Namespaced,categories=cluster-api,shortName=m3ipp;m3ippool
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this template belongs"

// Metal3IPPool is the Schema for the metal3ippools API
type Metal3IPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Metal3IPPoolSpec   `json:"spec,omitempty"`
	Status Metal3IPPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3IPPoolList contains a list of Metal3IPPool
type Metal3IPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3IPPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Metal3IPPool{}, &Metal3IPPoolList{})
}
