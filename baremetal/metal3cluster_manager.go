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
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	Metal3Cluster *infrav1.Metal3Cluster
	Log           logr.Logger
	// name string
}

// NewClusterManager returns a new helper for managing a cluster with a given name.
func NewClusterManager(client client.Client, cluster *clusterv1.Cluster,
	metal3Cluster *infrav1.Metal3Cluster,
	clusterLog logr.Logger) (ClusterManagerInterface, error) {
	if metal3Cluster == nil {
		return nil, errors.New("metal3Cluster is required when creating a ClusterManager")
	}
	if cluster == nil {
		return nil, errors.New("cluster is required when creating a ClusterManager")
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
	if !controllerutil.ContainsFinalizer(s.Metal3Cluster, infrav1.ClusterFinalizer) {
		s.Log.V(VerbosityLevelTrace).Info("Adding finalizer to Metal3Cluster")
		controllerutil.AddFinalizer(s.Metal3Cluster, infrav1.ClusterFinalizer)
	}
}

// UnsetFinalizer unsets finalizer.
func (s *ClusterManager) UnsetFinalizer() {
	// Cluster is deleted so remove the finalizer.
	s.Log.V(VerbosityLevelTrace).Info("Removing finalizer from Metal3Cluster")
	controllerutil.RemoveFinalizer(s.Metal3Cluster, infrav1.ClusterFinalizer)
}

// Create creates a cluster manager for the cluster.
func (s *ClusterManager) Create(_ context.Context) error {
	s.Log.V(VerbosityLevelTrace).Info("Validating Metal3Cluster configuration")
	config := s.Metal3Cluster.Spec
	err := config.IsValid()
	if err != nil {
		// Should have been picked earlier. Do not requeue.
		s.Log.V(VerbosityLevelDebug).Info("Invalid Metal3Cluster configuration",
			LogFieldError, err.Error())
		s.setError("Invalid Metal3Cluster provided", capierrors.InvalidConfigurationClusterError)
		return err
	}
	s.Log.V(VerbosityLevelDebug).Info("Metal3Cluster configuration is valid")
	return nil
}

// ControlPlaneEndpoint returns cluster controlplane endpoint.
func (s *ClusterManager) ControlPlaneEndpoint() ([]infrav1.APIEndpoint, error) {
	s.Log.V(VerbosityLevelTrace).Info("Getting ControlPlaneEndpoint")
	// Get IP address from spec, which gets it from posted cr yaml.
	endPoint := s.Metal3Cluster.Spec.ControlPlaneEndpoint
	var err error

	if endPoint.Host == "" || endPoint.Port == 0 {
		err = errors.New("invalid field ControlPlaneEndpoint")
		s.Log.V(VerbosityLevelDebug).Info("ControlPlaneEndpoint validation failed",
			"host", endPoint.Host,
			"port", endPoint.Port)
		return nil, err
	}

	s.Log.V(VerbosityLevelDebug).Info("ControlPlaneEndpoint is valid",
		"host", endPoint.Host,
		"port", endPoint.Port)
	return []infrav1.APIEndpoint{
		{
			Host: endPoint.Host,
			Port: endPoint.Port,
		},
	}, nil
}

// Delete function, no-op for now.
func (s *ClusterManager) Delete() error {
	s.Log.V(VerbosityLevelTrace).Info("Delete called on Metal3Cluster (no-op)")
	return nil
}

// UpdateClusterStatus updates a metal3Cluster object's status.
func (s *ClusterManager) UpdateClusterStatus() error {
	s.Log.V(VerbosityLevelTrace).Info("Updating Metal3Cluster status")
	// Get APIEndpoints from  metal3Cluster Spec
	_, err := s.ControlPlaneEndpoint()

	if err != nil {
		s.Log.V(VerbosityLevelDebug).Info("ControlPlaneEndpoint validation failed",
			LogFieldError, err.Error())
		s.Metal3Cluster.Status.Ready = false
		s.setError("Invalid ControlPlaneEndpoint values", capierrors.InvalidConfigurationClusterError)
		v1beta1conditions.MarkFalse(s.Metal3Cluster, infrav1.BaremetalInfrastructureReadyCondition, infrav1.ControlPlaneEndpointFailedReason, clusterv1beta1.ConditionSeverityError, "%s", err.Error())
		v1beta2conditions.Set(s.Metal3Cluster, metav1.Condition{
			Type:   infrav1.BaremetalInfrastructureReadyV1Beta2Condition,
			Status: metav1.ConditionFalse,
			Reason: infrav1.ControlPlaneEndpointFailedReason,
		})
		return err
	}

	// Mark the metal3Cluster ready.
	s.Log.V(VerbosityLevelDebug).Info("Metal3Cluster is ready")
	s.Metal3Cluster.Status.Ready = true
	v1beta1conditions.MarkTrue(s.Metal3Cluster, infrav1.BaremetalInfrastructureReadyCondition)
	v1beta2conditions.Set(s.Metal3Cluster, metav1.Condition{
		Type:   infrav1.BaremetalInfrastructureReadyV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.BaremetalInfrastructureReadyV1Beta2Reason,
	})
	now := metav1.Now()
	s.Metal3Cluster.Status.LastUpdated = &now
	return nil
}

// setError sets the FailureMessage and FailureReason fields on the metal3Cluster and logs
// the message. It assumes the reason is invalid configuration, since that is
// currently the only relevant Metal3ClusterStatusError choice.
func (s *ClusterManager) setError(message string, reason capierrors.ClusterStatusError) {
	s.Log.V(VerbosityLevelDebug).Info("Setting error on Metal3Cluster status",
		LogFieldError, message,
		"reason", reason)
	s.Metal3Cluster.Status.FailureMessage = &message
	s.Metal3Cluster.Status.FailureReason = &reason
}

// CountDescendants will return the number of descendants objects of the
// metal3Cluster.
func (s *ClusterManager) CountDescendants(ctx context.Context) (int, error) {
	s.Log.V(VerbosityLevelTrace).Info("Counting Metal3Cluster descendants")
	// Verify that no metal3machine depend on the metal3cluster
	descendants, err := s.listDescendants(ctx)
	if err != nil {
		s.Log.Error(err, "Failed to list descendants")

		return 0, err
	}

	nbDescendants := len(descendants.Items)
	s.Log.V(VerbosityLevelDebug).Info("Found descendants",
		LogFieldCount, nbDescendants)

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
	s.Log.V(VerbosityLevelTrace).Info("Listing cluster descendants")
	machines := clusterv1.MachineList{}
	cluster, err := util.GetOwnerCluster(ctx, s.client,
		s.Metal3Cluster.ObjectMeta,
	)
	if err != nil {
		return machines, err
	}
	s.Log.V(VerbosityLevelDebug).Info("Found owner cluster",
		LogFieldCluster, cluster.Name,
		LogFieldNamespace, cluster.Namespace)

	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			clusterv1.ClusterNameLabel: cluster.Name,
		}),
	}

	if err := s.client.List(ctx, &machines, listOptions...); err != nil {
		errMsg := fmt.Sprintf("failed to list metal3machines for cluster %s/%s", cluster.Namespace, cluster.Name)
		return machines, fmt.Errorf("%s: %w", errMsg, err)
	}

	s.Log.V(VerbosityLevelDebug).Info("Listed machines for cluster",
		LogFieldCluster, cluster.Name,
		LogFieldCount, len(machines.Items))
	return machines, nil
}
