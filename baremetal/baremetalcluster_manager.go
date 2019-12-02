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

package baremetal

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	// TODO Why blank import ?
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"net/url"
	"strconv"
)

//Constant variables
const (
	APIEndpointPort = "6443"
)

// ClusterManagerInterface is an interface for a ClusterManager
type ClusterManagerInterface interface {
	Create(context.Context) error
	Delete() error
	UpdateClusterStatus() error
	SetFinalizer()
	UnsetFinalizer()
	CountDescendants(context.Context) (int, error)
}

// ClusterManager is responsible for performing machine reconciliation
type ClusterManager struct {
	client client.Client

	Cluster          *capi.Cluster
	BareMetalCluster *capbm.BareMetalCluster
	Log              logr.Logger
	// name string
}

// NewClusterManager returns a new helper for managing a cluster with a given name.
func NewClusterManager(client client.Client, cluster *capi.Cluster,
	bareMetalCluster *capbm.BareMetalCluster,
	clusterLog logr.Logger) (ClusterManagerInterface, error) {

	if bareMetalCluster == nil {
		return nil, errors.New("BareMetalCluster is required when creating a ClusterManager")
	}
	if cluster == nil {
		return nil, errors.New("Cluster is required when creating a ClusterManager")
	}

	return &ClusterManager{
		client:           client,
		BareMetalCluster: bareMetalCluster,
		Cluster:          cluster,
		Log:              clusterLog,
	}, nil
}

// SetFinalizer sets finalizer
func (s *ClusterManager) SetFinalizer() {
	// If the BareMetalCluster doesn't have finalizer, add it.
	if !util.Contains(s.BareMetalCluster.ObjectMeta.Finalizers, capbm.ClusterFinalizer) {
		s.BareMetalCluster.ObjectMeta.Finalizers = append(
			s.BareMetalCluster.ObjectMeta.Finalizers, capbm.ClusterFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer
func (s *ClusterManager) UnsetFinalizer() {
	// Cluster is deleted so remove the finalizer.
	s.BareMetalCluster.ObjectMeta.Finalizers = util.Filter(
		s.BareMetalCluster.ObjectMeta.Finalizers, capbm.ClusterFinalizer,
	)
}

// Create creates a cluster manager for the cluster.
func (s *ClusterManager) Create(ctx context.Context) error {

	config := s.BareMetalCluster.Spec
	err := config.IsValid()
	if err != nil {
		// Should have been picked earlier. Do not requeue
		s.setError("Invalid BareMetalCluster provided", capierrors.InvalidConfigurationClusterError)
		return err
	}

	// clear an error if one was previously set
	s.clearError()

	return nil
}

// apiEndpoints returns the cluster manager IP address
func (s *ClusterManager) apiEndpoints() ([]capbm.APIEndpoint, error) {
	//Get IP address from spec, which gets it from posted cr yaml
	// Once IP is handled, consider setting the port

	endPoint := s.BareMetalCluster.Spec.APIEndpoint

	// Parse
	u, err := url.Parse(endPoint)
	if err != nil {
		s.Log.Error(err, "Unable to parse IP and PORT from the given url")
	}

	ip := u.Hostname()
	p := u.Port()

	if p == "" {
		p = APIEndpointPort
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		s.Log.Error(err, "Invalid Port")
		return nil, err
	}

	return []capbm.APIEndpoint{
		{
			Host: ip,
			Port: port,
		},
	}, nil
}

// Delete function, no-op for now
func (s *ClusterManager) Delete() error {
	return nil
}

// UpdateClusterStatus updates a machine object's status.
func (s *ClusterManager) UpdateClusterStatus() error {

	// Get APIEndpoints from  BaremetalCluster Spec
	endpoints, err := s.apiEndpoints()
	if err != nil {
		s.setError("Invalid APIEndpoints values", capierrors.InvalidConfigurationClusterError)
		return err
	}

	if equality.Semantic.DeepEqual(s.BareMetalCluster.Status.APIEndpoints, endpoints) {
		// Endpoints did not change
		return nil
	}

	s.BareMetalCluster.Status.APIEndpoints = endpoints
	// Mark the baremetalCluster ready
	s.BareMetalCluster.Status.Ready = true
	now := metav1.Now()
	s.BareMetalCluster.Status.LastUpdated = &now

	return nil
}

// setError sets the ErrorMessage and ErrorReason fields on the machine and logs
// the message. It assumes the reason is invalid configuration, since that is
// currently the only relevant MachineStatusError choice.
func (s *ClusterManager) setError(message string, reason capierrors.ClusterStatusError) {
	s.BareMetalCluster.Status.ErrorMessage = &message
	s.BareMetalCluster.Status.ErrorReason = &reason
}

// clearError removes the ErrorMessage from the machine's Status if set. Returns
// nil if ErrorMessage was already nil. Returns a RequeueAfterError if the
// machine was updated.
func (s *ClusterManager) clearError() {
	if s.BareMetalCluster.Status.ErrorMessage != nil || s.BareMetalCluster.Status.ErrorReason != nil {
		s.BareMetalCluster.Status.ErrorMessage = nil
		s.BareMetalCluster.Status.ErrorReason = nil
	}
}

// CountDescendants will return the number of descendants objects of the
// BaremetalCluster
func (s *ClusterManager) CountDescendants(ctx context.Context) (int, error) {
	// Verify that no baremetalmachine depend on the baremetalcluster
	descendants, err := s.listDescendants(ctx)
	if err != nil {
		s.Log.Error(err, "Failed to list descendants")

		return 0, err
	}

	nbDescendants := len(descendants.Items)

	if nbDescendants > 0 {
		s.Log.Info(
			"BaremetalCluster still has descendants - need to requeue", "descendants",
			nbDescendants,
		)
	}
	return nbDescendants, nil
}

// listDescendants returns a list of all Machines, for the cluster owning the
// BaremetalCluster.
func (s *ClusterManager) listDescendants(ctx context.Context) (capi.MachineList, error) {

	machines := capi.MachineList{}
	cluster, err := util.GetOwnerCluster(ctx, s.client,
		s.BareMetalCluster.ObjectMeta,
	)
	if err != nil {
		return machines, err
	}

	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			capi.MachineClusterLabelName: cluster.Name,
		}),
	}

	if s.client.List(ctx, &machines, listOptions...) != nil {
		errMsg := fmt.Sprintf("failed to list BaremetalMachines for cluster %s/%s", cluster.Namespace, cluster.Name)
		return machines, errors.Wrapf(err, errMsg)
	}

	return machines, nil
}
