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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// MachineFinalizer allows ReconcileMetal3Machine to clean up resources associated with Metal3Machine before
	// removing it from the apiserver.
	MachineFinalizer     = "metal3machine.infrastructure.cluster.x-k8s.io"
	CleaningModeDisabled = "disabled"
	CleaningModeMetadata = "metadata"
	ClonedFromGroupKind  = "Metal3MachineTemplate.infrastructure.cluster.x-k8s.io"
	LiveIsoDiskFormat    = "live-iso"
)

// Metal3MachineSpec defines the desired state of Metal3Machine.
type Metal3MachineSpec struct {
	// ProviderID will be the Metal3 machine in ProviderID format
	// (metal3://<bmh-uuid>)
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Image is the image to be provisioned.
	Image Image `json:"image"`

	// UserData references the Secret that holds user data needed by the bare metal
	// operator. The Namespace is optional; it will default to the metal3machine's
	// namespace if not specified.
	// +optional
	UserData *corev1.SecretReference `json:"userData,omitempty"`

	// HostSelector specifies matching criteria for labels on BareMetalHosts.
	// This is used to limit the set of BareMetalHost objects considered for
	// claiming for a metal3machine.
	// +optional
	HostSelector HostSelector `json:"hostSelector,omitempty"`

	// MetadataTemplate is a reference to a Metal3DataTemplate object containing
	// a template of metadata to be rendered. Metadata keys defined in the
	// metadataTemplate take precedence over keys defined in metadata field.
	// +optional
	DataTemplate *corev1.ObjectReference `json:"dataTemplate,omitempty"`

	// MetaData is an object storing the reference to the secret containing the
	// Metadata given by the user.
	// +optional
	MetaData *corev1.SecretReference `json:"metaData,omitempty"`

	// NetworkData is an object storing the reference to the secret containing the
	// network data given by the user.
	// +optional
	NetworkData *corev1.SecretReference `json:"networkData,omitempty"`

	// When set to disabled, automated cleaning of host disks will be skipped
	// during provisioning and deprovisioning.
	// +kubebuilder:validation:Enum:=metadata;disabled
	// +optional
	AutomatedCleaningMode *string `json:"automatedCleaningMode,omitempty"`
}

// Metal3MachineStatus defines the observed state of Metal3Machine.
type Metal3MachineStatus struct {

	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the metal3machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the metal3machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of
	// metal3machines can be added as events to the metal3machine object
	// and/or logged in the controller's output.
	// +optional
	FailureReason *capierrors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the metal3machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the metal3machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of
	// metal3machines can be added as events to the metal3machine object
	// and/or logged in the controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Addresses is a list of addresses assigned to the machine.
	// This field is copied from the infrastructure provider reference.
	// +optional
	Addresses clusterv1.MachineAddresses `json:"addresses,omitempty"`

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

	// UserData references the Secret that holds user data needed by the bare metal
	// operator. The Namespace is optional; it will default to the metal3machine's
	// namespace if not specified.
	// +optional
	UserData *corev1.SecretReference `json:"userData,omitempty"`

	// RenderedData is a reference to a rendered Metal3Data object containing
	// the references to metaData and networkData secrets.
	// +optional
	RenderedData *corev1.ObjectReference `json:"renderedData,omitempty"`

	// MetaData is an object storing the reference to the secret containing the
	// Metadata used to deploy the BareMetalHost.
	// +optional
	MetaData *corev1.SecretReference `json:"metaData,omitempty"`

	// NetworkData is an object storing the reference to the secret containing the
	// network data used to deploy the BareMetalHost.
	// +optional
	NetworkData *corev1.SecretReference `json:"networkData,omitempty"`
	// Conditions defines current service state of the Metal3Machine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3machines,scope=Namespaced,categories=cluster-api,shortName=m3m;m3machine;m3machines;metal3m;metal3machine
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Metal3Machine"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="metal3machine is Ready"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this M3Machine belongs"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="metal3machine current phase"

// Metal3Machine is the Schema for the metal3machines API.
type Metal3Machine struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec Metal3MachineSpec `json:"spec,omitempty"`
	// +optional
	Status Metal3MachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3MachineList contains a list of Metal3Machine.
type Metal3MachineList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3Machine `json:"items"`
}

// GetConditions returns the list of conditions for an Metal3Machine API object.
func (c *Metal3Machine) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an Metal3Machine object.
func (c *Metal3Machine) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

func init() {
	objectTypes = append(objectTypes, &Metal3Machine{}, &Metal3MachineList{})
}
