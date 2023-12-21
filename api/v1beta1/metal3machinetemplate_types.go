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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Metal3MachineTemplateSpec defines the desired state of Metal3MachineTemplate.
type Metal3MachineTemplateSpec struct {
	Template Metal3MachineTemplateResource `json:"template"`

	// When set to True, CAPM3 Machine controller will
	// pick the same pool of BMHs' that were released during the upgrade operation.
	// +kubebuilder:default=false
	// +optional
	NodeReuse bool `json:"nodeReuse"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Metal3MachineTemplate"
// +kubebuilder:resource:path=metal3machinetemplates,scope=Namespaced,categories=cluster-api,shortName=m3mt;m3machinetemplate;m3machinetemplates;metal3mt;metal3machinetemplate
// +kubebuilder:storageversion

// Metal3MachineTemplate is the Schema for the metal3machinetemplates API.
type Metal3MachineTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec Metal3MachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3MachineTemplateList contains a list of Metal3MachineTemplate.
type Metal3MachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3MachineTemplate `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &Metal3MachineTemplate{}, &Metal3MachineTemplateList{})
}

// Metal3MachineTemplateResource describes the data needed to create a Metal3Machine from a template.
type Metal3MachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec Metal3MachineSpec `json:"spec"`
}
