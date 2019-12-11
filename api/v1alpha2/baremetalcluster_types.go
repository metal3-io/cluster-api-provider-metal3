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
	"net/url"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// ClusterFinalizer allows BareMetalClusterReconciler to clean up resources associated with BareMetalCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "baremetalcluster.infrastructure.cluster.x-k8s.io"
)

// BareMetalClusterSpec defines the desired state of BareMetalCluster.
type BareMetalClusterSpec struct {
	APIEndpoint     string `json:"apiEndpoint"`
	NoCloudProvider bool   `json:"noCloudProvider,omitempty"`
}

// APIEndPointError represents error in the APIEndPoint in BareMetalCluster.Spec
type APIEndPointError struct {
	Message string
}

// Error implements the error interface and returns the error message
func (e *APIEndPointError) Error() string {
	return fmt.Sprintf("APIEndPoint is not valid, %s", e.Message)
}

// IsValid returns an error if the object is not valid, otherwise nil. The
// string representation of the error is suitable for human consumption.
func (s *BareMetalClusterSpec) IsValid() error {
	missing := []string{}
	if s.APIEndpoint == "" {
		missing = append(missing, "APIEndpoint")
	}
	if len(missing) > 0 {
		return &APIEndPointError{fmt.Sprintf("Missing fields from Spec: %s", missing)}
	}
	u, err := url.Parse(s.APIEndpoint)

	if err != nil || u.Hostname() == "" {
		return &APIEndPointError{"Incorrect API endpoint, expecting [scheme:]//host[:port]"}
	}

	if u.Port() != "" {
		_, err = strconv.Atoi(u.Port())
		if err != nil {
			return &APIEndPointError{"Invalid Port"}
		}
	}
	return nil
}

// BareMetalClusterStatus defines the observed state of BareMetalCluster.
type BareMetalClusterStatus struct {
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
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the BaremetalMachine object and/or logged in the
	// controller's output.
	// +optional
	ErrorReason *capierrors.ClusterStatusError `json:"errorReason,omitempty"`

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
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the BaremetalMachine object and/or logged in the
	// controller's output.
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// Ready denotes that the baremetal cluster (infrastructure) is ready. In
	// Baremetal case, it does not mean anything for now as no infrastructure
	// steps need to be performed. Required by Cluster API. Set to True by the
	// BaremetalCluster controller after creation.
	Ready bool `json:"ready"`

	// APIEndpoints represents the endpoints to communicate with the control plane.
	// +optional
	APIEndpoints []APIEndpoint `json:"apiEndpoints,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=baremetalclusters,scope=Namespaced,categories=cluster-api,shortName=bmc;bmcluster
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="BaremetalCluster is Ready"
// +kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.errorReason",description="Most recent error"
// +kubebuilder:printcolumn:name="APIEndpoints",type="string",JSONPath=".status.apiEndpoints",description="API endpoints"

// BareMetalCluster is the Schema for the baremetalclusters API
type BareMetalCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BareMetalClusterSpec   `json:"spec,omitempty"`
	Status BareMetalClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalClusterList contains a list of BareMetalCluster
type BareMetalClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalCluster{}, &BareMetalClusterList{})
}
