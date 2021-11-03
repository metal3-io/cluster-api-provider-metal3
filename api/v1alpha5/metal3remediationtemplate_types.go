/*
Copyright The Kubernetes Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Metal3RemediationTemplateSpec defines the desired state of Metal3RemediationTemplate
type Metal3RemediationTemplateSpec struct {
	Template Metal3RemediationTemplateResource `json:"template"`
}

// Metal3RemediationTemplateResource describes the data needed to create a Metal3Remediation from a template
type Metal3RemediationTemplateResource struct {
	// Spec is the specification of the desired behavior of the Metal3Remediation.
	Spec Metal3RemediationSpec `json:"spec"`
}

// Metal3RemediationTemplateStatus defines the observed state of Metal3RemediationTemplate
type Metal3RemediationTemplateStatus struct {
	// Metal3RemediationStatus defines the observed state of Metal3Remediation
	Status Metal3RemediationStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3remediationtemplates,scope=Namespaced,categories=cluster-api,shortName=m3rt;m3remediationtemplate;m3remediationtemplates;metal3rt;metal3remediationtemplate
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true

// Metal3RemediationTemplate is the Schema for the metal3remediationtemplates API
type Metal3RemediationTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Metal3RemediationTemplateSpec   `json:"spec,omitempty"`
	Status Metal3RemediationTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3RemediationTemplateList contains a list of Metal3RemediationTemplate
type Metal3RemediationTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3RemediationTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Metal3RemediationTemplate{}, &Metal3RemediationTemplateList{})
}
