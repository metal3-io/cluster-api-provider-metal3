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
	"github.com/go-logr/logr"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	capm3remote "github.com/metal3-io/cluster-api-provider-metal3/baremetal/remote"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagerFactoryInterface is a collection of new managers.
type ManagerFactoryInterface interface {
	NewClusterManager(cluster *clusterv1beta1.Cluster,
		metal3Cluster *infrav1.Metal3Cluster,
		clusterLog logr.Logger,
	) (ClusterManagerInterface, error)
	NewMachineManager(*clusterv1beta1.Cluster, *infrav1.Metal3Cluster, *clusterv1beta1.Machine,
		*infrav1.Metal3Machine, logr.Logger,
	) (MachineManagerInterface, error)
	NewDataTemplateManager(*infrav1.Metal3DataTemplate, logr.Logger) (
		DataTemplateManagerInterface, error,
	)
	NewDataManager(*infrav1.Metal3Data, logr.Logger) (
		DataManagerInterface, error,
	)
	NewMachineTemplateManager(capm3Template *infrav1.Metal3MachineTemplate,
		capm3MachineList *infrav1.Metal3MachineList,
		metadataLog logr.Logger,
	) (TemplateManagerInterface, error)
	NewRemediationManager(*infrav1.Metal3Remediation, *infrav1.Metal3Machine, *clusterv1beta1.Machine, logr.Logger) (
		RemediationManagerInterface, error,
	)
}

// ManagerFactory only contains a client.
type ManagerFactory struct {
	client client.Client
}

// NewManagerFactory returns a new factory.
func NewManagerFactory(client client.Client) ManagerFactory {
	return ManagerFactory{client: client}
}

// NewClusterManager creates a new ClusterManager.
func (f ManagerFactory) NewClusterManager(cluster *clusterv1beta1.Cluster, capm3Cluster *infrav1.Metal3Cluster, clusterLog logr.Logger) (ClusterManagerInterface, error) {
	return NewClusterManager(f.client, cluster, capm3Cluster, clusterLog)
}

// NewMachineManager creates a new MachineManager.
func (f ManagerFactory) NewMachineManager(capiCluster *clusterv1beta1.Cluster,
	capm3Cluster *infrav1.Metal3Cluster,
	capiMachine *clusterv1beta1.Machine, capm3Machine *infrav1.Metal3Machine,
	machineLog logr.Logger) (MachineManagerInterface, error) {
	return NewMachineManager(f.client, capiCluster, capm3Cluster, capiMachine,
		capm3Machine, machineLog)
}

// NewDataTemplateManager creates a new DataTemplateManager.
func (f ManagerFactory) NewDataTemplateManager(metadata *infrav1.Metal3DataTemplate, metadataLog logr.Logger) (DataTemplateManagerInterface, error) {
	return NewDataTemplateManager(f.client, metadata, metadataLog)
}

// NewDataManager creates a new DataManager.
func (f ManagerFactory) NewDataManager(metadata *infrav1.Metal3Data, metadataLog logr.Logger) (DataManagerInterface, error) {
	return NewDataManager(f.client, metadata, metadataLog)
}

// NewMachineTemplateManager creates a new Metal3MachineTemplateManager.
func (f ManagerFactory) NewMachineTemplateManager(capm3Template *infrav1.Metal3MachineTemplate,
	capm3MachineList *infrav1.Metal3MachineList,
	metadataLog logr.Logger) (TemplateManagerInterface, error) {
	return NewMachineTemplateManager(f.client, capm3Template, capm3MachineList, metadataLog)
}

// NewRemediationManager creates a new RemediationManager.
func (f ManagerFactory) NewRemediationManager(remediation *infrav1.Metal3Remediation,
	metal3machine *infrav1.Metal3Machine, machine *clusterv1beta1.Machine,
	remediationLog logr.Logger) (RemediationManagerInterface, error) {
	return NewRemediationManager(f.client, capm3remote.NewClusterClient, remediation, metal3machine, machine, remediationLog)
}
