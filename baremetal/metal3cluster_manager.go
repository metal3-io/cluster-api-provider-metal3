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

package baremetal

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	// TODO Why blank import ?
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterManagerInterface is an interface for a ClusterManager.
type ClusterManagerInterface interface {
	Create(context.Context) error
	Delete() error
	UpdateClusterStatus() error
	SetFinalizer()
	UnsetFinalizer()
	CountDescendants(context.Context) (int, error)
}

// ClusterManager is responsible for performing metal3 cluster reconciliation.
type ClusterManager struct {
	client client.Client

	Cluster       *clusterv1.Cluster
	Metal3Cluster *capm3.Metal3Cluster
	Log           logr.Logger
	// name string
}

// NewClusterManager returns a new helper for managing a cluster with a given name.
func NewClusterManager(client client.Client, cluster *clusterv1.Cluster,
	metal3Cluster *capm3.Metal3Cluster,
	clusterLog logr.Logger) (ClusterManagerInterface, error) {
	if metal3Cluster == nil {
		return nil, errors.New("Metal3Cluster is required when creating a ClusterManager")
	}
	if cluster == nil {
		return nil, errors.New("Cluster is required when creating a ClusterManager")
	}

	return &ClusterManager{
		client:        client,
		Metal3Cluster: metal3Cluster,
		Cluster:       cluster,
		Log:           clusterLog,
	}, nil
}

// SetFinalizer sets finalizer.
func (s *ClusterManager) SetFinalizer() {
	// If the Metal3Cluster doesn't have finalizer, add it.
	if !Contains(s.Metal3Cluster.ObjectMeta.Finalizers, capm3.ClusterFinalizer) {
		s.Metal3Cluster.ObjectMeta.Finalizers = append(
			s.Metal3Cluster.ObjectMeta.Finalizers, capm3.ClusterFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer.
func (s *ClusterManager) UnsetFinalizer() {
	// Cluster is deleted so remove the finalizer.
	s.Metal3Cluster.ObjectMeta.Finalizers = Filter(
		s.Metal3Cluster.ObjectMeta.Finalizers, capm3.ClusterFinalizer,
	)
}

// Create creates a cluster manager for the cluster.
func (s *ClusterManager) Create(ctx context.Context) error {
	config := s.Metal3Cluster.Spec
	err := config.IsValid()
	if err != nil {
		// Should have been picked earlier. Do not requeue.
		s.setError("Invalid Metal3Cluster provided", capierrors.InvalidConfigurationClusterError)
		return err
	}

	// clear an error if one was previously set.
	s.clearError()

	return nil
}

// ControlPlaneEndpoint returns cluster controlplane endpoint.
func (s *ClusterManager) ControlPlaneEndpoint() ([]capm3.APIEndpoint, error) {
	// Get IP address from spec, which gets it from posted cr yaml.
	endPoint := s.Metal3Cluster.Spec.ControlPlaneEndpoint
	var err error

	if endPoint.Host == "" || endPoint.Port == 0 {
		s.Log.Error(err, "Host IP or PORT not set")
		return nil, err
	}

	return []capm3.APIEndpoint{
		{
			Host: endPoint.Host,
			Port: endPoint.Port,
		},
	}, nil
}

// Delete function, no-op for now.
func (s *ClusterManager) Delete() error {
	return nil
}

// UpdateClusterStatus updates a metal3Cluster object's status.
func (s *ClusterManager) UpdateClusterStatus() error {
	// Get APIEndpoints from  metal3Cluster Spec
	_, err := s.ControlPlaneEndpoint()

	if err != nil {
		s.Metal3Cluster.Status.Ready = false
		s.setError("Invalid ControlPlaneEndpoint values", capierrors.InvalidConfigurationClusterError)
		conditions.MarkFalse(s.Metal3Cluster, capm3.BaremetalInfrastructureReadyCondition, capm3.ControlPlaneEndpointFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return err
	}

	// Mark the metal3Cluster ready.
	s.Metal3Cluster.Status.Ready = true
	conditions.MarkTrue(s.Metal3Cluster, capm3.BaremetalInfrastructureReadyCondition)
	now := metav1.Now()
	s.Metal3Cluster.Status.LastUpdated = &now
	return nil
}

// setError sets the FailureMessage and FailureReason fields on the metal3Cluster and logs
// the message. It assumes the reason is invalid configuration, since that is
// currently the only relevant Metal3ClusterStatusError choice.
func (s *ClusterManager) setError(message string, reason capierrors.ClusterStatusError) {
	s.Metal3Cluster.Status.FailureMessage = &message
	s.Metal3Cluster.Status.FailureReason = &reason
}

// clearError removes the ErrorMessage from the metal3Cluster Status if set. Returns
// nil if ErrorMessage was already nil.
func (s *ClusterManager) clearError() {
	if s.Metal3Cluster.Status.FailureMessage != nil || s.Metal3Cluster.Status.FailureReason != nil {
		s.Metal3Cluster.Status.FailureMessage = nil
		s.Metal3Cluster.Status.FailureReason = nil
	}
}

// CountDescendants will return the number of descendants objects of the
// metal3Cluster.
func (s *ClusterManager) CountDescendants(ctx context.Context) (int, error) {
	// Verify that no metal3machine depend on the metal3cluster
	descendants, err := s.listDescendants(ctx)
	if err != nil {
		s.Log.Error(err, "Failed to list descendants")

		return 0, err
	}

	nbDescendants := len(descendants.Items)

	if nbDescendants > 0 {
		s.Log.Info(
			"metal3Cluster still has descendants - need to requeue", "descendants",
			nbDescendants,
		)
	}
	return nbDescendants, nil
}

// listDescendants returns a list of all Machines, for the cluster owning the
// metal3Cluster.
func (s *ClusterManager) listDescendants(ctx context.Context) (clusterv1.MachineList, error) {
	machines := clusterv1.MachineList{}
	cluster, err := util.GetOwnerCluster(ctx, s.client,
		s.Metal3Cluster.ObjectMeta,
	)
	if err != nil {
		return machines, err
	}

	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			clusterv1.ClusterLabelName: cluster.Name,
		}),
	}

	if s.client.List(ctx, &machines, listOptions...) != nil {
		errMsg := fmt.Sprintf("failed to list metal3machines for cluster %s/%s", cluster.Namespace, cluster.Name)
		return machines, errors.Wrapf(err, errMsg)
	}

	return machines, nil
}
