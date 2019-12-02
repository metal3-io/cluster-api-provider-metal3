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
	"testing"

	"k8s.io/klog/klogr"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewManagerFactory(t *testing.T) {
	c := fakeclient.NewFakeClientWithScheme(setupScheme())
	managerFactory := NewManagerFactory(c)
	if managerFactory.client != c {
		t.Fatal("Failed to create a new manager factory")
	}
}

func TestFactoryNewClusterManager(t *testing.T) {
	c := fakeclient.NewFakeClientWithScheme(setupScheme())
	managerFactory := NewManagerFactory(c)
	cluster := &capi.Cluster{}
	capbmCluster := &capbm.BareMetalCluster{}
	clusterLog := klogr.New()

	_, err := managerFactory.NewClusterManager(cluster, capbmCluster,
		clusterLog,
	)
	if err != nil {
		t.Error("Failed to create a cluster manager")
	}
}

func TestFactoryNewMachineManager(t *testing.T) {
	c := fakeclient.NewFakeClientWithScheme(setupScheme())
	managerFactory := NewManagerFactory(c)
	cluster := &capi.Cluster{}
	capbmCluster := &capbm.BareMetalCluster{}
	machine := &capi.Machine{}
	capbmMachine := &capbm.BareMetalMachine{}
	clusterLog := klogr.New()

	_, err := managerFactory.NewMachineManager(cluster, capbmCluster,
		machine, capbmMachine, clusterLog,
	)
	if err != nil {
		t.Error("Failed to create a machine manager")
	}
}
