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
	// DataClaimFinalizer allows Metal3DataReconciler to clean up resources
	// associated with Metal3DataClaim before removing it from the apiserver.
	DataClaimFinalizer = "metal3dataclaim.infrastructure.cluster.x-k8s.io"
)

// Metal3DataClaimSpec defines the desired state of Metal3DataClaim.
type Metal3DataClaimSpec struct {
	// Template is the Metal3DataTemplate this was generated for.
	Template corev1.ObjectReference `json:"template"`
}

// Metal3DataClaimStatus defines the observed state of Metal3DataClaim.
type Metal3DataClaimStatus struct {
	// RenderedData references the Metal3Data when ready
	RenderedData *corev1.ObjectReference `json:"renderedData,omitempty"`

	// ErrorMessage contains the error message
	ErrorMessage *string `json:"errorMessage,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3dataclaims,scope=Namespaced,categories=cluster-api,shortName=m3dc;m3dataclaim;m3dataclaims;metal3dc;metal3dataclaim
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Metal3DataClaim"
// Metal3DataClaim is the Schema for the metal3datas API
type Metal3DataClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Metal3DataClaimSpec   `json:"spec,omitempty"`
	Status Metal3DataClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3DataClaimList contains a list of Metal3DataClaim
type Metal3DataClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3DataClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Metal3DataClaim{}, &Metal3DataClaimList{})
}
