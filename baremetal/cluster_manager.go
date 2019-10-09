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

	_ "github.com/go-logr/logr"
	"github.com/pkg/errors"

	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterManager is responsible for performing machine reconciliation
type ClusterManager struct {
	client      client.Client
	patchHelper *patch.Helper

	Cluster          *capi.Cluster
	BareMetalCluster *capbm.BareMetalCluster
	// log          logr.Logger
	// name string
}

// newClusterManager returns a new helper for managing a cluster with a given name.
func newClusterManager(client client.Client,
	cluster *capi.Cluster, bareMetalCluster *capbm.BareMetalCluster) (*ClusterManager, error) {
	if cluster == nil {
		return nil, errors.New("Cluster is required when creating a ClusterManager")
	}
	if bareMetalCluster == nil {
		return nil, errors.New("BareMetalCluster is required when creating a ClusterManager")
	}

	helper, err := patch.NewHelper(bareMetalCluster, client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ClusterManager{
		client:      client,
		patchHelper: helper,

		Cluster:          cluster,
		BareMetalCluster: bareMetalCluster,
	}, nil
}

// Create creates a docker container hosting a cluster manager for the cluster.
func (s *ClusterManager) Create() error {
	// Create if not exists.
	return nil
}

// UpdateConfiguration updates the external cluster manager configuration with new control plane nodes.
func (s *ClusterManager) UpdateConfiguration() error {
	return nil
}

// IP returns the cluster manager IP address
func (s *ClusterManager) APIEndpoints() ([]capbm.APIEndpoint, error) {
	return []capbm.APIEndpoint{
		{
			Host: "",
			Port: 8080,
		},
	}, nil
}

// Delete function, no-op for now
func (s *ClusterManager) Delete() error {
	return nil
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterManager) Close() error {
	return s.patchHelper.Patch(context.TODO(), s.BareMetalCluster)
}
