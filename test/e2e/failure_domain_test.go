/*
Copyright 2024 The Kubernetes Authors.

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

package e2e

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// failureDomainClusterName differs from the shared "test1" name so this spec's
// non-garbage-collected template resources (ClusterResourceSet, CNI ConfigMap,
// Metal3DataTemplates) don't collide with the other features specs that share
// the namespace.
const failureDomainClusterName = "test-fd"

// This spec runs in the shared "features" bucket. The fallback assertion needs
// exactly three selectable hosts; LabelBmhsForFailureDomain guarantees that by
// marking every spare host unhealthy, so the spec stays deterministic even under
// the larger features node profile. The "failure-domain" label lets it be
// singled out within a features run via GINKGO_FOCUS/GINKGO_SKIP.
var _ = Describe("When testing failure domains", Label("failure-domain", "features"), func() {
	BeforeEach(func() {
		osType = strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "target_cluster_logs", bootstrapClusterProxy.GetName())
	})

	It("Should place control plane machines according to failure domain labels", func() {
		numberOfControlplane = int(*e2eConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
		k8sVersion := e2eConfig.MustGetVariable("KUBERNETES_VERSION")

		By("Apply BMH for workload cluster")
		ApplyBmh(ctx, e2eConfig, bootstrapClusterProxy, namespace, specName)

		By("Label available BMHs with failure domains")
		LabelBmhsForFailureDomain(ctx, bootstrapClusterProxy.GetClient(), namespace)
		// Restore the shared BMHs (remove failure-domain labels and unhealthy
		// annotations) once this spec finishes, so the next spec in the
		// features bucket starts from a clean, fully selectable host pool.
		DeferCleanup(func() {
			CleanupFailureDomainBmhs(ctx, bootstrapClusterProxy.GetClient(), namespace)
		})

		By("Provision Workload cluster with failure domains")
		targetCluster, _ = CreateTargetCluster(ctx, func() CreateTargetClusterInput {
			return CreateTargetClusterInput{
				E2EConfig:             e2eConfig,
				BootstrapClusterProxy: bootstrapClusterProxy,
				SpecName:              specName,
				ClusterName:           failureDomainClusterName,
				K8sVersion:            k8sVersion,
				KCPMachineCount:       int64(numberOfControlplane),
				WorkerMachineCount:    0,
				ClusterctlLogFolder:   clusterctlLogFolder,
				ClusterctlConfigPath:  clusterctlConfigPath,
				OSType:                osType + "-failure-domain" + flavorSuffix(),
				Namespace:             namespace,
			}
		})

		FailureDomain(ctx, func() FailureDomainInput {
			return FailureDomainInput{
				E2EConfig:             e2eConfig,
				BootstrapClusterProxy: bootstrapClusterProxy,
				SpecName:              specName,
				ClusterName:           failureDomainClusterName,
				Namespace:             namespace,
			}
		})
	})

	AfterEach(func() {
		ListBareMetalHosts(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		ListMetal3Machines(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		ListMachines(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		if targetCluster != nil {
			ListNodes(ctx, targetCluster.GetClient())
		}
		DumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, targetCluster, artifactFolder, namespace, e2eConfig.GetIntervals, failureDomainClusterName, clusterctlLogFolder, skipCleanup, clusterctlConfigPath)
	})
})
