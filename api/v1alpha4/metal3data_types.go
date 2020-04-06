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
	// DataFinalizer allows Metal3DataReconciler to clean up resources
	// associated with Metal3Data before removing it from the apiserver.
	DataFinalizer = "metal3data.infrastructure.cluster.x-k8s.io"
)

// Metal3DataSpec defines the desired state of Metal3Data.
type Metal3DataSpec struct {
	Index         int                     `json:"index,omitempty"`
	MetaData      *corev1.SecretReference `json:"metaData,omitempty"`
	NetworkData   *corev1.SecretReference `json:"networkData,omitempty"`
	Metal3Machine *corev1.ObjectReference `json:"metal3Machine,omitempty"`
	DataTemplate  *corev1.ObjectReference `json:"dataTemplate,omitempty"`
}

// Metal3DataStatus defines the observed state of Metal3Data.
type Metal3DataStatus struct {
	Ready        bool    `json:"ready,omitempty"`
	Error        bool    `json:"error,omitempty"`
	ErrorMessage *string `json:"errorMessage,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3datas,scope=Namespaced,categories=cluster-api,shortName=m3d;m3data
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
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
