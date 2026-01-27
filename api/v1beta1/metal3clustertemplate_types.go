/*
Copyright 2024 The Kubernetes Authors.
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

// Metal3ClusterTemplateSpec defines the desired state of Metal3ClusterTemplate.
type Metal3ClusterTemplateSpec struct {
	Template Metal3ClusterTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=metal3clustertemplates,scope=Namespaced,categories=cluster-api,shortName=m3ct
// +kubebuilder:deprecatedversion

// Metal3ClusterTemplate is the Schema for the metal3clustertemplates API.
type Metal3ClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec Metal3ClusterTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3ClusterTemplateList contains a list of Metal3ClusterTemplate.
type Metal3ClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3ClusterTemplate `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &Metal3ClusterTemplate{}, &Metal3ClusterTemplateList{})
}

// Metal3ClusterTemplateResource describes the data for creating a Metal3Cluster from a template.
type Metal3ClusterTemplateResource struct {
	Spec Metal3ClusterSpec `json:"spec"`
}
