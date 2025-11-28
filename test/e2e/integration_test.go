package e2e

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/cluster-api/test/framework"
)

// Integration tests in CAPM3 focus on validating the seamless integration between different components of the CAPM3 project,
// including CAPM3, IPAM, CAPI, BMO, and Ironic. These tests ensure that these components work together cohesively to provision a
// workload cluster and perform pivoting operations between the bootstrap cluster and the target cluster.
// The primary goal is to detect any compatibility issues or conflicts that may arise during integration.

// By executing the integration tests, CAPM3 verifies that:

// - The CAPM3 controller effectively interacts with the IPAM, CAPI, BMO, and Ironic components.
// - The provisioning of a workload cluster proceeds smoothly, and that BMHs are created, inspected and provisioned as expected.
// - The pivoting functionality enables the seamless moving of resources and control components from the bootstrap cluster to the target cluster and vice versa.
// - Deprovisioning the cluster and BMHs happens smoothly.
var _ = Describe("When testing integration", Label("integration"), func() {

	It("CI Test Provision", func() {
		numberOfWorkers = int(*e2eConfig.MustGetInt32PtrVariable("WORKER_MACHINE_COUNT"))
		numberOfControlplane = int(*e2eConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
		k8sVersion := e2eConfig.MustGetVariable("K8S_VERSION")
		By("Apply BMH for workload cluster")
		ApplyBmh(ctx, e2eConfig, bootstrapClusterProxy, namespace, specName)

		By("Provision Workload cluster")
		targetCluster, _ = CreateTargetCluster(ctx, func() CreateTargetClusterInput {
			return CreateTargetClusterInput{
				E2EConfig:             e2eConfig,
				BootstrapClusterProxy: bootstrapClusterProxy,
				SpecName:              specName,
				ClusterName:           clusterName,
				K8sVersion:            k8sVersion,
				KCPMachineCount:       int64(numberOfControlplane),
				WorkerMachineCount:    int64(numberOfWorkers),
				ClusterctlLogFolder:   clusterctlLogFolder,
				ClusterctlConfigPath:  clusterctlConfigPath,
				OSType:                osType,
				Namespace:             namespace,
			}
		})
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
			Lister:               targetCluster.GetClient(),
			Namespace:            namespace,
			LogPath:              filepath.Join(artifactFolder, "clusters", clusterName, "resources"),
			KubeConfigPath:       targetCluster.GetKubeconfigPath(),
			ClusterctlConfigPath: clusterctlConfigPath,
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
		DumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, targetCluster, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup, clusterctlConfigPath)
	})
})
