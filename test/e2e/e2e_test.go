package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

var (
	ctx                      = context.TODO()
	specName                 = "metal3"
	namespace                = "metal3"
	cluster                  *clusterv1.Cluster
	clusterName              = "test1"
	clusterctlLogFolder      string
	controlplaneListOptions  metav1.ListOptions
	targetCluster            framework.ClusterProxy
	controlPlaneMachineCount int64
	workerMachineCount       int64
)

const KIND = "kind"

var _ = Describe("Workload cluster creation", func() {

	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})

	Context("Creating a highly available control-plane cluster", func() {
		It("Should create a cluster with 3 control-plane and 1 worker nodes", func() {
			By("Creating a high available cluster")
			if upgradeTest {
				upgradeManagementCluster()
			} else {
				controlPlaneMachineCount = int64(*e2eConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
				workerMachineCount = int64(*e2eConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
				result := &clusterctl.ApplyClusterTemplateAndWaitResult{}
				clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
					ClusterProxy: bootstrapClusterProxy,
					ConfigCluster: clusterctl.ConfigClusterInput{
						LogFolder:                clusterctlLogFolder,
						ClusterctlConfigPath:     clusterctlConfigPath,
						KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
						InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
						Flavor:                   osType,
						Namespace:                namespace,
						ClusterName:              clusterName,
						KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
						ControlPlaneMachineCount: &controlPlaneMachineCount,
						WorkerMachineCount:       &workerMachineCount,
					},
					CNIManifestPath:              e2eConfig.GetVariable(capi_e2e.CNIPath),
					WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
					WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
					WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				}, result)
				cluster = result.Cluster
				targetCluster = bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
				remediation()
				pivoting()
				upgradeBMO()
				upgradeIronic()
				certRotation()
				nodeReuse()
				rePivoting()
			}
		})
	})
})
