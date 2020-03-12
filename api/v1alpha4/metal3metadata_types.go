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
	// MetadataFinalizer allows Metal3MetadataReconciler to clean up resources
	// associated with Metal3Metadata before removing it from the apiserver.
	MetadataFinalizer = "metal3metadata.infrastructure.cluster.x-k8s.io"
)

type Metal3MetadataValue map[string]string

// Metal3MetadataSpec defines the desired state of Metal3Metadata.
type Metal3MetadataSpec struct {
	MetaData map[string]string `json:"metaData,omitempty"`
}

// Metal3MetadataStatus defines the observed state of Metal3Metadata
type Metal3MetadataStatus struct {
	Indexes map[string]string                 `json:"indexes,omitempty"`
	Secrets map[string]corev1.SecretReference `json:"secrets,omitempty"`
	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3metadatas,scope=Namespaced,categories=cluster-api,shortName=m3md;m3metadata
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this Metadata belongs"

// Metal3Metadata is the Schema for the metal3metadatas API
type Metal3Metadata struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Metal3MetadataSpec   `json:"spec,omitempty"`
	Status Metal3MetadataStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3MetadataList contains a list of Metal3Metadata
type Metal3MetadataList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3Metadata `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Metal3Metadata{}, &Metal3MetadataList{})
}
