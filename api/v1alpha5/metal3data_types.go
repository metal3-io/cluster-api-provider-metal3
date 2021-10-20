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

package v1alpha5

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DataFinalizer allows Metal3DataReconciler to clean up resources
	// associated with Metal3Data before removing it from the apiserver.
	DataFinalizer = "metal3data.infrastructure.cluster.x-k8s.io"
)

// Metal3DataSpec defines the desired state of Metal3Data.
type Metal3DataSpec struct {
	// Index stores the index value of this instance in the Metal3DataTemplate.
	Index int `json:"index,omitempty"`

	// TemplateReference refers to the Template the Metal3MachineTemplate refers to.
	// It can be matched against the key or it may also point to the name of the template
	// Metal3Data refers to
	TemplateReference string `json:"templateReference,omitempty"`

	// MetaData points to the rendered MetaData secret.
	MetaData *corev1.SecretReference `json:"metaData,omitempty"`

	// NetworkData points to the rendered NetworkData secret.
	NetworkData *corev1.SecretReference `json:"networkData,omitempty"`

	// DataClaim points to the Metal3DataClaim the Metal3Data was created for.
	Claim corev1.ObjectReference `json:"claim"`

	// DataTemplate is the Metal3DataTemplate this was generated from.
	Template corev1.ObjectReference `json:"template"`
}

// Metal3DataStatus defines the observed state of Metal3Data.
type Metal3DataStatus struct {
	// Ready is a flag set to True if the secrets were rendered properly
	Ready bool `json:"ready,omitempty"`

	// ErrorMessage contains the error message
	ErrorMessage *string `json:"errorMessage,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3datas,scope=Namespaced,categories=cluster-api,shortName=m3d;m3data;m3datas;metal3d;metal3data
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Metal3Data"
// Metal3Data is the Schema for the metal3datas API
type Metal3Data struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Metal3DataSpec   `json:"spec,omitempty"`
	Status Metal3DataStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3DataList contains a list of Metal3Data
type Metal3DataList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3Data `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Metal3Data{}, &Metal3DataList{})
}
