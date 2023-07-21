package e2e

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/cluster-api/test/framework"
)

var _ = Describe("When testing integration [integration]", func() {

	It("CI Test Provision", func() {
		numberOfWorkers = int(*e2eConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
		numberOfControlplane = int(*e2eConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
		k8sVersion := e2eConfig.GetVariable("KUBERNETES_VERSION")
		By("Provision Workload cluster")
		targetCluster, _ = createTargetCluster(k8sVersion)
		By("Pivot objects to target cluster")
		pivoting(ctx, func() PivotingInput {
			return PivotingInput{
				E2EConfig:             e2eConfig,
				BootstrapClusterProxy: bootstrapClusterProxy,
				TargetCluster:         targetCluster,
				SpecName:              specName,
				ClusterName:           clusterName,
				Namespace:             namespace,
				ArtifactFolder:        artifactFolder,
				ClusterctlConfigPath:  clusterctlConfigPath,
			}
		})
		// Fetch the target cluster resources before re-pivoting.
		By("Fetch the target cluster resources before re-pivoting")
		framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
			Lister:    targetCluster.GetClient(),
			Namespace: namespace,
			LogPath:   filepath.Join(artifactFolder, "clusters", clusterName, "resources"),
		})
		By("Repivot objects to the source cluster")
		rePivoting(ctx, func() RePivotingInput {
			return RePivotingInput{
				E2EConfig:             e2eConfig,
				BootstrapClusterProxy: bootstrapClusterProxy,
				TargetCluster:         targetCluster,
				SpecName:              specName,
				ClusterName:           clusterName,
				Namespace:             namespace,
				ArtifactFolder:        artifactFolder,
				ClusterctlConfigPath:  clusterctlConfigPath,
			}
		})
	})

	AfterEach(func() {
		DumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})
})
