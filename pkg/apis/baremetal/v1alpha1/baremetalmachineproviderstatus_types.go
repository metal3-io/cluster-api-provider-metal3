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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BareMetalMachineProviderStatusSpec defines the desired state of BareMetalMachineProviderStatus
type BareMetalMachineProviderStatusSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// BareMetalMachineProviderStatusStatus defines the observed state of BareMetalMachineProviderStatus
type BareMetalMachineProviderStatusStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BareMetalMachineProviderStatus is the Schema for the baremetalmachineproviderstatuses API
// +k8s:openapi-gen=true
type BareMetalMachineProviderStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BareMetalMachineProviderStatusSpec   `json:"spec,omitempty"`
	Status BareMetalMachineProviderStatusStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BareMetalMachineProviderStatusList contains a list of BareMetalMachineProviderStatus
type BareMetalMachineProviderStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalMachineProviderStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalMachineProviderStatus{}, &BareMetalMachineProviderStatusList{})
}
