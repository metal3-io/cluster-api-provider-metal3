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

package v1alpha2

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// MachineFinalizer allows ReconcileBareMetalMachine to clean up resources associated with AWSMachine before
	// removing it from the apiserver.
	MachineFinalizer = "baremetalmachine.infrastructure.cluster.x-k8s.io"
)

// BareMetalMachineSpec defines the desired state of BareMetalMachine
type BareMetalMachineSpec struct {
	// ProviderID will be the container name in ProviderID format (baremetal:////<containername>)
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Image is the image to be provisioned.
	Image Image `json:"image"`

	// UserData references the Secret that holds user data needed by the bare metal
	// operator. The Namespace is optional; it will default to the Machine's
	// namespace if not specified.
	UserData *corev1.SecretReference `json:"userData,omitempty"`

	// HostSelector specifies matching criteria for labels on BareMetalHosts.
	// This is used to limit the set of BareMetalHost objects considered for
	// claiming for a Machine.
	HostSelector HostSelector `json:"hostSelector,omitempty"`
}

// IsValid returns an error if the object is not valid, otherwise nil. The
// string representation of the error is suitable for human consumption.
func (s *BareMetalMachineSpec) IsValid() error {
	missing := []string{}
	if s.Image.URL == "" {
		missing = append(missing, "Image.URL")
	}
	if s.Image.Checksum == "" {
		missing = append(missing, "Image.Checksum")
	}
	if len(missing) > 0 {
		return fmt.Errorf("Missing fields from ProviderSpec: %v", missing)
	}
	return nil
}

// BareMetalMachineStatus defines the observed state of BareMetalMachine
type BareMetalMachineStatus struct {
	// Ready denotes that the machine (baremetal container) is ready
	Ready bool `json:"ready"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=baremetalmachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// BareMetalMachine is the Schema for the baremetalmachines API
type BareMetalMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BareMetalMachineSpec   `json:"spec,omitempty"`
	Status BareMetalMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalMachineList contains a list of BareMetalMachine
type BareMetalMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalMachine{}, &BareMetalMachineList{})
}
