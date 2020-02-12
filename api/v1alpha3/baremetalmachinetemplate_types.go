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

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BareMetalMachineTemplateSpec defines the desired state of BareMetalMachineTemplate
type BareMetalMachineTemplateSpec struct {
	Template BareMetalMachineTemplateResource `json:"template"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=baremetalmachinetemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// BareMetalMachineTemplate is the Schema for the baremetalmachinetemplates API
type BareMetalMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BareMetalMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalMachineTemplateList contains a list of BareMetalMachineTemplate
type BareMetalMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalMachineTemplate{}, &BareMetalMachineTemplateList{})
}

// BareMetalMachineTemplateResource describes the data needed to create a BareMetalMachine from a template
type BareMetalMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec BareMetalMachineSpec `json:"spec"`
}
