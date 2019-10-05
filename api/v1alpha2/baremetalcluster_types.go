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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ClusterFinalizer allows BareMetalClusterReconciler to clean up resources associated with BareMetalCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "baremetalcluster.infrastructure.cluster.x-k8s.io"
)

// BareMetalClusterSpec defines the desired state of BareMetalCluster.
type BareMetalClusterSpec struct {
	// The name of the secret containing the openstack credentials
	// +optional
	CloudsSecret *corev1.SecretReference `json:"cloudsSecret"`

	// The name of the cloud to use from the clouds secret
	// +optional
	CloudName string `json:"cloudName"`
}

// BareMetalClusterStatus defines the observed state of BareMetalCluster.
type BareMetalClusterStatus struct {
	// Ready denotes that the baremetal cluster (infrastructure) is ready.
	Ready bool `json:"ready"`

	// APIEndpoints represents the endpoints to communicate with the control plane.
	// +optional
	APIEndpoints []APIEndpoint `json:"apiEndpoints,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=baremetalclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:object:root=true

// BareMetalCluster is the Schema for the baremetalclusters API
type BareMetalCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BareMetalClusterSpec   `json:"spec,omitempty"`
	Status BareMetalClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalClusterList contains a list of BareMetalCluster
type BareMetalClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalCluster{}, &BareMetalClusterList{})
}
