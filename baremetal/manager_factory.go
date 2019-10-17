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
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ManagerFactory struct {
	client client.Client
}

// NewManagerFactory returns a new factory.
func NewManagerFactory(client client.Client) ManagerFactory {
	return ManagerFactory{client: client}
}

// NewClusterManager creates a new ClusterManager
func (f ManagerFactory) NewClusterManager(capiCluster *capi.Cluster, capbmCluster *capbm.BareMetalCluster, clusterLog logr.Logger) (*ClusterManager, error) {
	return newClusterManager(f.client, capiCluster, capbmCluster, clusterLog)
}

// NewMachineManager creates a new MachineManager
func (f ManagerFactory) NewMachineManager(capiCluster *capi.Cluster,
	capbmCluster *capbm.BareMetalCluster,
	capiMachine *capi.Machine, capbmMachine *capbm.BareMetalMachine,
	machineLog logr.Logger) (*MachineManager, error) {
	return newMachineManager(f.client, capiCluster, capbmCluster, capiMachine,
		capbmMachine, machineLog)
}
