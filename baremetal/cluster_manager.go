/*
Copyright 2018 The Kubernetes Authors.

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
	_ "github.com/go-logr/logr"
	"github.com/pkg/errors"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterManager is responsible for performing machine reconciliation
type ClusterManager struct {
	client       client.Client
	capiCluster  *capi.Cluster
	capbmCluster *capbm.BareMetalCluster
	// log          logr.Logger
	// name string
}

// newClusterManager returns a new helper for managing a cluster with a given name.
func newClusterManager(client client.Client,
	capiCluster *capi.Cluster, capbmCluster *capbm.BareMetalCluster) (*ClusterManager, error) {
	if capiCluster == nil {
		return nil, errors.New("capiCluster is required when creating a ClusterManager")
	}
	if capbmCluster == nil {
		return nil, errors.New("capbmCluster is required when creating a ClusterManager")
	}

	return &ClusterManager{
		client:       client,
		capiCluster:  capiCluster,
		capbmCluster: capbmCluster,
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
func (s *ClusterManager) IP() (string, error) {
	return "", nil
}

// Delete the docker containers hosting a loadbalancer for the cluster.
func (s *ClusterManager) Delete() error {
	return nil
}
