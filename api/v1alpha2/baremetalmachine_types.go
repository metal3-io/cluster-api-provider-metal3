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
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// MachineFinalizer allows ReconcileBareMetalMachine to clean up resources associated with AWSMachine before
	// removing it from the apiserver.
	MachineFinalizer = "baremetalmachine.infrastructure.cluster.x-k8s.io"
)

// BareMetalMachineSpec defines the desired state of BareMetalMachine
type BareMetalMachineSpec struct {
	// ProviderID will be the baremetal machine in ProviderID format
	// (baremetal:////<machinename>)
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Image is the image to be provisioned.
	Image Image `json:"image"`

	// UserData references the Secret that holds user data needed by the bare metal
	// operator. The Namespace is optional; it will default to the BaremetalMachine's
	// namespace if not specified.
	UserData *corev1.SecretReference `json:"userData,omitempty"`

	// HostSelector specifies matching criteria for labels on BareMetalHosts.
	// This is used to limit the set of BareMetalHost objects considered for
	// claiming for a BaremetalMachine.
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

	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// ErrorReason will be set in the event that there is a terminal problem
	// reconciling the BaremetalMachine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the BaremetalMachine's spec or the configuration
	// of the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of
	// BaremetalMachines can be added as events to the BaremetalMachine object
	// and/or logged in the controller's output.
	// +optional
	ErrorReason *capierrors.MachineStatusError `json:"errorReason,omitempty"`

	// ErrorMessage will be set in the event that there is a terminal problem
	// reconciling the BaremetalMachine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the BaremetalMachine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of
	// BaremetalMachines can be added as events to the BaremetalMachine object
	// and/or logged in the controller's output.
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// Addresses is a list of addresses assigned to the machine.
	// This field is copied from the infrastructure provider reference.
	// +optional
	Addresses capi.MachineAddresses `json:"addresses,omitempty"`

	// Phase represents the current phase of machine actuation.
	// E.g. Pending, Running, Terminating, Failed etc.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Ready is the state of the metal3.
	// TODO : Document the variable :
	// mhrivnak: " it would be good to document what this means, how to interpret
	// it, under what circumstances the value changes, etc."
	// +optional
	Ready bool `json:"ready"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=baremetalmachines,scope=Namespaced,categories=cluster-api,shortName=bmm;bmmachine
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Machines current phase"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="BaremetalMachine is Ready"
// +kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.errorReason",description="Most recent error"

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
