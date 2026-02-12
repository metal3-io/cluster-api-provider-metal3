package e2e

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("When testing ip reuse", Label("ip-reuse", "features"), func() {

	BeforeEach(func() {
		osType = strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "target_cluster_logs", bootstrapClusterProxy.GetName())
	})
	It("Should create a workload cluster then verify ip allocation reuse while upgrading k8s", func() {
		By("Apply BMH for workload cluster")
		ApplyBmh(ctx, e2eConfig, bootstrapClusterProxy, namespace, specName)
		By("Provision Workload cluster")
		targetCluster, _ = CreateTargetCluster(ctx, func() CreateTargetClusterInput {
			return CreateTargetClusterInput{
				E2EConfig:             e2eConfig,
				BootstrapClusterProxy: bootstrapClusterProxy,
				SpecName:              specName,
				ClusterName:           clusterName,
				K8sVersion:            e2eConfig.MustGetVariable("KUBERNETES_VERSION_FROM"),
				KCPMachineCount:       int64(numberOfControlplane),
				WorkerMachineCount:    int64(numberOfWorkers),
				ClusterctlLogFolder:   clusterctlLogFolder,
				ClusterctlConfigPath:  clusterctlConfigPath,
				OSType:                osType,
				Namespace:             namespace,
			}
		})
		IPReuse(ctx, func() IPReuseInput {
			return IPReuseInput{
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
