package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/utils/pointer"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

var _ = Describe("Workload cluster creation", func() {
	var (
		ctx                 = context.TODO()
		specName            = "metal3"
		namespace           = "metal3"
		flavorSuffix        string
		cluster             *clusterv1.Cluster
		clusterName         = "test1"
		clusterctlLogFolder string
	)

	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		if osType == "centos" {
			flavorSuffix = "-centos"
		} else {
			flavorSuffix = ""
		}

		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)

		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, cluster, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})

	Context("Creating a highly available control-plane cluster", func() {
		It("Should create a cluster with 3 control-plane and 1 worker nodes", func() {
			By("Creating a high available cluster")
			result := &clusterctl.ApplyClusterTemplateAndWaitResult{}
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ha" + flavorSuffix,
					Namespace:                namespace,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(3),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				CNIManifestPath:              e2eTestsPath + "/data/cni/calico/calico.yaml",
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
			cluster = result.Cluster
		})
	})
})
