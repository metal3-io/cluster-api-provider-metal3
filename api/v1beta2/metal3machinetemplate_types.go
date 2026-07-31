/*
Copyright 2026 The Kubernetes Authors.

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

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// Metal3MachineTemplateSpec defines the desired state of Metal3MachineTemplate.
type Metal3MachineTemplateSpec struct {
	// template describes the data needed to create a Metal3Machine from a template
	// +required
	Template Metal3MachineTemplateResource `json:"template,omitempty,omitzero"`

	// nodeReuse is a flag that can be set to True to enable node reuse during upgrade.
	// When set to True, CAPM3 Machine controller will
	// pick the same pool of BMHs' that were released during the upgrade operation.
	// +kubebuilder:default=false
	// +optional
	NodeReuse *bool `json:"nodeReuse,omitempty"`

	// failureDomainDataTemplates maps failure domain names to Metal3DataTemplate
	// references. The mapping is keyed by the failure-domain label of the
	// BareMetalHost a Metal3Machine cloned from this template is actually
	// placed on. When the label value is present in this list, the machine's
	// dataTemplate is overridden with the referenced Metal3DataTemplate before
	// its metadata is rendered. Machines placed on hosts without the label, or
	// whose label value is not in this list, keep the default
	// template.spec.dataTemplate.
	// +optional
	// +listType=map
	// +listMapKey=failureDomain
	// +kubebuilder:validation:MaxItems=32
	FailureDomainDataTemplates []FailureDomainDataTemplate `json:"failureDomainDataTemplates,omitempty"`
}

// FailureDomainDataTemplate maps a failure domain name to a Metal3DataTemplate
// reference.
type FailureDomainDataTemplate struct {
	// failureDomain is the name of the failure domain, as declared in the
	// Metal3Cluster spec.failureDomains. It is matched against the value of
	// the infrastructure.cluster.x-k8s.io/failure-domain label on
	// BareMetalHosts, so it is bounded by the label value length limit.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	FailureDomain string `json:"failureDomain,omitempty"`

	// dataTemplate is a reference to the Metal3DataTemplate to use for
	// Metal3Machines placed on a BareMetalHost in this failure domain.
	// +required
	// +kubebuilder:validation:XValidation:rule="has(self.name)",message="dataTemplate name is required"
	DataTemplate *Metal3ObjectRef `json:"dataTemplate,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Metal3MachineTemplate"
// +kubebuilder:resource:path=metal3machinetemplates,scope=Namespaced,categories=cluster-api,shortName=m3mt;m3machinetemplate;m3machinetemplates;metal3mt;metal3machinetemplate
// +kubebuilder:storageversion

// Metal3MachineTemplate is the Schema for the metal3machinetemplates API.
type Metal3MachineTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// spec defines the desired state of Metal3MachineTemplate.
	// +required
	Spec Metal3MachineTemplateSpec `json:"spec,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// Metal3MachineTemplateList contains a list of Metal3MachineTemplate.
type Metal3MachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3MachineTemplate `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &Metal3MachineTemplate{}, &Metal3MachineTemplateList{})
}

// Metal3MachineTemplateResource describes the data needed to create a Metal3Machine from a template.
type Metal3MachineTemplateResource struct {
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty,omitzero"`
	// spec is the specification of the desired behavior of the machine.
	// +required
	Spec Metal3MachineSpec `json:"spec,omitempty"`
}
