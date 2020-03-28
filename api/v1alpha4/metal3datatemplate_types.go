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
	// DataTemplateFinalizer allows Metal3DataTemplateReconciler to clean up resources
	// associated with Metal3DataTemplate before removing it from the apiserver.
	DataTemplateFinalizer = "metal3datatemplate.infrastructure.cluster.x-k8s.io"
)

// Metal3DataTemplateSpec defines the desired state of Metal3DataTemplate.
type Metal3DataTemplateSpec struct {
	MetaData    *string `json:"metaData,omitempty"`
	NetworkData *string `json:"networkData,omitempty"`
}

// Metal3DataTemplateStatus defines the observed state of Metal3DataTemplate
type Metal3DataTemplateStatus struct {
	Indexes            map[string]string                 `json:"indexes,omitempty"`
	MetaDataSecrets    map[string]corev1.SecretReference `json:"metaDataSecrets,omitempty"`
	NetworkDataSecrets map[string]corev1.SecretReference `json:"networkDataSecrets,omitempty"`
	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3datatemplates,scope=Namespaced,categories=cluster-api,shortName=m3dt;m3datatemplate
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this template belongs"

// Metal3DataTemplate is the Schema for the metal3datatemplates API
type Metal3DataTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Metal3DataTemplateSpec   `json:"spec,omitempty"`
	Status Metal3DataTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3DataTemplateList contains a list of Metal3DataTemplate
type Metal3DataTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3DataTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Metal3DataTemplate{}, &Metal3DataTemplateList{})
}
