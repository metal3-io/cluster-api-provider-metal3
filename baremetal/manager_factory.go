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
	"github.com/go-logr/logr"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ManagerFactoryInterface interface {
	NewClusterManager(cluster *capi.Cluster,
		bareMetalCluster *capbm.BareMetalCluster,
		clusterLog logr.Logger) (ClusterManagerInterface, error)
	NewMachineManager(*capi.Cluster, *capbm.BareMetalCluster, *capi.Machine,
		*capbm.BareMetalMachine, logr.Logger) (MachineManagerInterface, error)
}

// ManagerFactory only contains a client
type ManagerFactory struct {
	client client.Client
}

// NewManagerFactory returns a new factory.
func NewManagerFactory(client client.Client) ManagerFactory {
	return ManagerFactory{client: client}
}

// NewClusterManager creates a new ClusterManager
func (f ManagerFactory) NewClusterManager(cluster *capi.Cluster, capbmCluster *capbm.BareMetalCluster, clusterLog logr.Logger) (ClusterManagerInterface, error) {
	return NewClusterManager(f.client, cluster, capbmCluster, clusterLog)
}

// NewMachineManager creates a new MachineManager
func (f ManagerFactory) NewMachineManager(capiCluster *capi.Cluster,
	capbmCluster *capbm.BareMetalCluster,
	capiMachine *capi.Machine, capbmMachine *capbm.BareMetalMachine,
	machineLog logr.Logger) (MachineManagerInterface, error) {
	return NewMachineManager(f.client, capiCluster, capbmCluster, capiMachine,
		capbmMachine, machineLog)
}
