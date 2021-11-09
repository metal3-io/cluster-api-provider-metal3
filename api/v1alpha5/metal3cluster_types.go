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

package v1alpha5

import (
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// ClusterFinalizer allows Metal3ClusterReconciler to clean up resources associated with Metal3Cluster before
	// removing it from the apiserver.
	ClusterFinalizer = "metal3cluster.infrastructure.cluster.x-k8s.io"
)

// Metal3ClusterSpec defines the desired state of Metal3Cluster.
type Metal3ClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	ControlPlaneEndpoint APIEndpoint `json:"controlPlaneEndpoint"`
	// Determines if the cluster is not to be deployed with an external cloud provider.
	// If set to true, CAPM3 will use node labels to set providerID on the kubernetes nodes.
	// If set to false, providerID is set on nodes by other entities and CAPM3 uses the value of the providerID on the m3m resource.
	NoCloudProvider bool `json:"noCloudProvider,omitempty"`
}

// IsValid returns an error if the object is not valid, otherwise nil. The
// string representation of the error is suitable for human consumption.
func (s *Metal3ClusterSpec) IsValid() error {
	missing := []string{}
	if s.ControlPlaneEndpoint.Host == "" {
		missing = append(missing, "ControlPlaneEndpoint.Host")
	}

	if s.ControlPlaneEndpoint.Port == 0 {
		missing = append(missing, "ControlPlaneEndpoint.Host")
	}

	if len(missing) > 0 {
		return errors.Errorf("Missing fields from Spec: %v", missing)
	}
	return nil
}

// Metal3ClusterStatus defines the observed state of Metal3Cluster.
type Metal3ClusterStatus struct {
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

	// Ready denotes that the Metal3 cluster (infrastructure) is ready. In
	// Baremetal case, it does not mean anything for now as no infrastructure
	// steps need to be performed. Required by Cluster API. Set to True by the
	// metal3Cluster controller after creation.
	Ready bool `json:"ready"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3clusters,scope=Namespaced,categories=cluster-api,shortName=m3c;m3cluster;m3clusters;metal3c;metal3cluster
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Metal3Cluster"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="metal3Cluster is Ready"
// +kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.failureReason",description="Most recent error"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this BMCluster belongs"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.controlPlaneEndpoint",description="Control plane endpoint"

// Metal3Cluster is the Schema for the metal3clusters API
type Metal3Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Metal3ClusterSpec   `json:"spec,omitempty"`
	Status Metal3ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3ClusterList contains a list of Metal3Cluster
type Metal3ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Metal3Cluster{}, &Metal3ClusterList{})
}
