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
	"fmt"

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
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	ControlPlaneEndpoint APIEndpoint `json:"controlPlaneEndpoint"`
	NoCloudProvider      bool        `json:"noCloudProvider,omitempty"`

	// ClusterName is the name of the Cluster this object belongs to.
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName"`
}

// IsValid returns an error if the object is not valid, otherwise nil. The
// string representation of the error is suitable for human consumption.
func (s *BareMetalClusterSpec) IsValid() error {
	missing := []string{}
	if s.ControlPlaneEndpoint.Host == "" {
		missing = append(missing, "ControlPlaneEndpoint.Host")
	}

	if s.ControlPlaneEndpoint.Port == 0 {
		missing = append(missing, "ControlPlaneEndpoint.Host")
	}

	if len(missing) > 0 {
		return fmt.Errorf("Missing fields from Spec: %v", missing)
	}
	return nil
}

// BareMetalClusterStatus defines the observed state of BareMetalCluster.
type BareMetalClusterStatus struct {
	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// FailureReason indicates that there is a fatal problem reconciling the
	// state, and will be set to a token value suitable for
	// programmatic interpretation.
	// +optional
	FailureReason *capierrors.ClusterStatusError `json:"failureReason,omitempty"`

	// FailureMessage indicates that there is a fatal problem reconciling the
	// state, and will be set to a descriptive error message.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Ready denotes that the baremetal cluster (infrastructure) is ready. In
	// Baremetal case, it does not mean anything for now as no infrastructure
	// steps need to be performed. Required by Cluster API. Set to True by the
	// BaremetalCluster controller after creation.
	Ready bool `json:"ready"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=baremetalclusters,scope=Namespaced,categories=cluster-api,shortName=bmc;bmcluster
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="BaremetalCluster is Ready"
// +kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.failureReason",description="Most recent error"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this BMCluster belongs"

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
