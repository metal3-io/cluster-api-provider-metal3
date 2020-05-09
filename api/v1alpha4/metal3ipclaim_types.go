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
	// IPClaimFinalizer allows Metal3IPClaimReconciler to clean up resources
	// associated with Metal3IPClaim before removing it from the apiserver.
	IPClaimFinalizer = "metal3ipclaim.infrastructure.cluster.x-k8s.io"
)

// Metal3IPClaimSpec defines the desired state of Metal3IPClaim.
type Metal3IPClaimSpec struct {

	// Pool is the Metal3IPPool this was generated from.
	Pool corev1.ObjectReference `json:"pool"`
}

// Metal3IPClaimStatus defines the observed state of Metal3IPClaim.
type Metal3IPClaimStatus struct {

	// Address is the Metal3IPAddress that was generated for this claim.
	Address *corev1.ObjectReference `json:"address,omitempty"`

	// ErrorMessage contains the error message
	ErrorMessage *string `json:"errorMessage,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3ipclaims,scope=Namespaced,categories=cluster-api,shortName=m3ipc;m3ipclaim;metal3ipclaim
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// Metal3IPClaim is the Schema for the metal3ipclaims API
type Metal3IPClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Metal3IPClaimSpec   `json:"spec,omitempty"`
	Status Metal3IPClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3IPClaimList contains a list of Metal3IPClaim
type Metal3IPClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3IPClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Metal3IPClaim{}, &Metal3IPClaimList{})
}
