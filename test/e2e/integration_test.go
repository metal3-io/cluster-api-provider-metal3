package e2e

import (
	"path/filepath"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
var _ = Describe("When testing integration [integration]", func() {
	It("CI Test Provision", func() {
		numberOfWorkers = int(*e2eConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
		numberOfControlplane = int(*e2eConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
		k8sVersion := e2eConfig.GetVariable("KUBERNETES_VERSION")
		By("Provision Workload cluster")
		targetCluster, result := createTargetCluster(k8sVersion)
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
		By("Deprovision target cluster")
		bootstrapClient := bootstrapClusterProxy.GetClient()
		intervals := e2eConfig.GetIntervals(specName, "wait-deprovision-cluster")
		// In pivoting step we labeled the BMO CRDs (so that the objects are moved
		// by CAPI pivoting feature), which made CAPI DeleteClusterAndWait()
		// fail as it has a check to make sure all resources managed by CAPI
		// is gone after Cluster deletion. Therefore, we opted not to use
		// DeleteClusterAndWait(), but only delete the cluster and then wait
		// for it to be deleted.
		framework.DeleteCluster(ctx, framework.DeleteClusterInput{
			Deleter: bootstrapClient,
			Cluster: result.Cluster,
		})
		Logf("Waiting for the Cluster object to be deleted")
		framework.WaitForClusterDeleted(ctx, framework.WaitForClusterDeletedInput{
			Getter:  bootstrapClient,
			Cluster: result.Cluster,
		}, intervals...)
		numberOfAvailableBMHs := numberOfWorkers + numberOfControlplane
		intervals = e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning-available")
		WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
			Client:    bootstrapClient,
			Options:   []client.ListOption{client.InNamespace(namespace)},
			Replicas:  numberOfAvailableBMHs,
			Intervals: intervals,
		})
	})
})
