package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Kubernetes version upgrade in target nodes", Label("k8s-upgrade-n3"), func() {

	var (
		ctx                 = context.TODO()
		clusterctlLogFolder string
	)

	BeforeEach(func() {
		osType = strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "target_cluster_logs", bootstrapClusterProxy.GetName())
	})

	It("Should create a cluster and run kubernetes N+3 tests", func() {
		By("Apply BMH for workload cluster")
		ApplyBmh(ctx, e2eConfig, bootstrapClusterProxy, namespace, specName)
		By("Creating target cluster")
		targetCluster, _ = CreateTargetCluster(ctx, func() CreateTargetClusterInput {
			return CreateTargetClusterInput{
				E2EConfig:             e2eConfig,
				BootstrapClusterProxy: bootstrapClusterProxy,
				SpecName:              specName,
				ClusterName:           clusterName,
				K8sVersion:            e2eConfig.MustGetVariable("KUBERNETES_N0_VERSION"),
				KCPMachineCount:       int64(numberOfControlplane),
				WorkerMachineCount:    int64(numberOfWorkers),
				ClusterctlLogFolder:   clusterctlLogFolder,
				ClusterctlConfigPath:  clusterctlConfigPath,
				OSType:                osType,
				Namespace:             namespace,
			}
		})

		By("Running Kubernetes Upgrade tests")
		upgradeKubernetesN3(ctx, func() upgradeKubernetesN3Input {
			return upgradeKubernetesN3Input{
				E2EConfig:             e2eConfig,
				BootstrapClusterProxy: bootstrapClusterProxy,
				TargetCluster:         targetCluster,
				SpecName:              specName,
				ClusterName:           clusterName,
				Namespace:             namespace,
			}
		})
	})

	AfterEach(func() {
		ListBareMetalHosts(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		ListMetal3Machines(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		ListMachines(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		ListNodes(ctx, targetCluster.GetClient())
		DumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, targetCluster, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup, clusterctlConfigPath)
	})

})

type upgradeKubernetesN3Input struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
}

// upgradeKubernetesN3 implements a test upgrading the cluster nodes from an old k8s version to a newer version.
func upgradeKubernetesN3(ctx context.Context, inputGetter func() upgradeKubernetesN3Input) {
	Logf("Starting Kubernetes upgrade tests")
	input := inputGetter()
	clusterClient := input.BootstrapClusterProxy.GetClient()
	targetClusterClient := input.TargetCluster.GetClient()
	kubernetesVersion := input.E2EConfig.MustGetVariable("KUBERNETES_N0_VERSION")
	upgradedK8sVersion1 := input.E2EConfig.MustGetVariable("KUBERNETES_N1_VERSION")
	upgradedK8sVersion2 := input.E2EConfig.MustGetVariable("KUBERNETES_N2_VERSION")
	upgradedK8sVersion3 := input.E2EConfig.MustGetVariable("KUBERNETES_N3_VERSION")

	Logf("KUBERNETES VERSION: %v", kubernetesVersion)
	Logf("UPGRADED K8S VERSION: %v", upgradedK8sVersion1)

	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, clusterClient, client.InNamespace(namespace))
	ListMachines(ctx, clusterClient, client.InNamespace(namespace))
	ListNodes(ctx, targetClusterClient)

	By("Running Kubernetes n+1 Upgrade tests")
	UpgradeControlPlane(ctx, func() UpgradeControlPlaneInput {
		return UpgradeControlPlaneInput{
			E2EConfig:             e2eConfig,
			BootstrapClusterProxy: bootstrapClusterProxy,
			TargetCluster:         targetCluster,
			SpecName:              specName,
			ClusterName:           clusterName,
			Namespace:             namespace,
			K8sFromVersion:        kubernetesVersion,
			K8sToVersion:          upgradedK8sVersion1,
		}
	})
	By("KUBERNETES UPGRADE N+1 TESTS PASSED!")
	Logf("KUBERNETES VERSION: %v", upgradedK8sVersion1)
	Logf("UPGRADED K8S VERSION: %v", upgradedK8sVersion2)
	Logf("NUMBER OF CONTROLPLANE BMH: %v", numberOfControlplane)

	By("Running Kubernetes N+2 Upgrade tests")
	UpgradeControlPlane(ctx, func() UpgradeControlPlaneInput {
		return UpgradeControlPlaneInput{
			E2EConfig:             e2eConfig,
			BootstrapClusterProxy: bootstrapClusterProxy,
			TargetCluster:         targetCluster,
			SpecName:              specName,
			ClusterName:           clusterName,
			Namespace:             namespace,
			K8sFromVersion:        upgradedK8sVersion1,
			K8sToVersion:          upgradedK8sVersion2,
		}
	})
	By("KUBERNETES UPGRADE N+2 TESTS PASSED!")
	Logf("KUBERNETES VERSION: %v", upgradedK8sVersion2)
	Logf("UPGRADED K8S VERSION: %v", upgradedK8sVersion3)
	Logf("NUMBER OF CONTROLPLANE BMH: %v", numberOfControlplane)

	By("Running Kubernetes n+3 Upgrade tests")
	UpgradeControlPlane(ctx, func() UpgradeControlPlaneInput {
		return UpgradeControlPlaneInput{
			E2EConfig:             e2eConfig,
			BootstrapClusterProxy: bootstrapClusterProxy,
			TargetCluster:         targetCluster,
			SpecName:              specName,
			ClusterName:           clusterName,
			Namespace:             namespace,
			K8sFromVersion:        upgradedK8sVersion2,
			K8sToVersion:          upgradedK8sVersion3,
		}
	})
	By("KUBERNETES UPGRADE N+3 TESTS PASSED!")
}
