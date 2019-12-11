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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/klog/klogr"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Manager factory testing", func() {
	var managerClient client.Client
	var managerFactory ManagerFactory
	clusterLog := klogr.New()

	BeforeEach(func() {
		managerClient = fakeclient.NewFakeClientWithScheme(setupScheme())
		managerFactory = NewManagerFactory(managerClient)
	})

	It("returns a manager factory", func() {
		Expect(managerFactory.client).To(Equal(managerClient))
	})

	It("returns a cluster manager", func() {
		_, err := managerFactory.NewClusterManager(&capi.Cluster{},
			&capbm.BareMetalCluster{}, clusterLog,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	It("fails to return a cluster manager with nil cluster", func() {
		_, err := managerFactory.NewClusterManager(nil, &capbm.BareMetalCluster{},
			clusterLog,
		)
		Expect(err).To(HaveOccurred())
	})

	It("fails to return a cluster manager with nil bmcluster", func() {
		_, err := managerFactory.NewClusterManager(&capi.Cluster{}, nil,
			clusterLog,
		)
		Expect(err).To(HaveOccurred())
	})

	It("returns a machine manager", func() {
		_, err := managerFactory.NewMachineManager(&capi.Cluster{},
			&capbm.BareMetalCluster{}, &capi.Machine{}, &capbm.BareMetalMachine{},
			clusterLog,
		)
		Expect(err).NotTo(HaveOccurred())
	})
})
