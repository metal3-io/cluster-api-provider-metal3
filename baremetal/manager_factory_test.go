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
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Manager factory testing", func() {
	var fakeClient client.Client
	var managerFactory ManagerFactory
	clusterLog := logr.Discard()

	BeforeEach(func() {
		fakeClient = fake.NewClientBuilder().WithScheme(setupScheme()).Build()
		managerFactory = NewManagerFactory(fakeClient)
	})

	It("returns a manager factory", func() {
		Expect(managerFactory.client).To(Equal(fakeClient))
	})

	It("returns a cluster manager", func() {
		_, err := managerFactory.NewClusterManager(&clusterv1.Cluster{},
			&infrav1.Metal3Cluster{}, clusterLog,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	It("fails to return a cluster manager with nil cluster", func() {
		_, err := managerFactory.NewClusterManager(nil, &infrav1.Metal3Cluster{},
			clusterLog,
		)
		Expect(err).To(HaveOccurred())
	})

	It("fails to return a cluster manager with nil m3cluster", func() {
		_, err := managerFactory.NewClusterManager(&clusterv1.Cluster{}, nil,
			clusterLog,
		)
		Expect(err).To(HaveOccurred())
	})

	It("returns a metal3 machine manager", func() {
		_, err := managerFactory.NewMachineManager(&clusterv1.Cluster{},
			&infrav1.Metal3Cluster{}, &clusterv1.Machine{}, &infrav1.Metal3Machine{},
			clusterLog,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns a DataTemplate manager", func() {
		_, err := managerFactory.NewDataTemplateManager(&infrav1.Metal3DataTemplate{}, clusterLog)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns a Data manager", func() {
		_, err := managerFactory.NewDataManager(&infrav1.Metal3Data{}, clusterLog)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns a MachineTemplate manager", func() {
		_, err := managerFactory.NewMachineTemplateManager(&infrav1.Metal3MachineTemplate{}, &infrav1.Metal3MachineList{}, clusterLog)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns a Remediation manager", func() {
		_, err := managerFactory.NewRemediationManager(&infrav1.Metal3Remediation{}, &infrav1.Metal3Machine{}, &clusterv1.Machine{}, clusterLog)
		Expect(err).NotTo(HaveOccurred())
	})
})
