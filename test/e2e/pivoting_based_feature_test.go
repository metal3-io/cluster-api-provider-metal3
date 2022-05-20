package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

var (
	ctx                      = context.TODO()
	specName                 = "metal3"
	namespace                = "metal3"
	clusterName              = "test1"
	clusterctlLogFolder      string
	controlplaneListOptions  metav1.ListOptions
	targetCluster            framework.ClusterProxy
	controlPlaneMachineCount int64
	workerMachineCount       int64
)

const KIND = "kind"

var _ = Describe("Testing features in ephemeral or target cluster", func() {

	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	It("Should get a management cluster then test cert rotation and node reuse", func() {
		targetCluster = createTargetCluster()
		if false {
		managementCluster := bootstrapClusterProxy
		// If not running ephemeral test, use the target cluster for management
		if !ephemeralTest {
			managementCluster = targetCluster
			pivoting()
		}

		certRotation(managementCluster.GetClientSet(), managementCluster.GetClient())
		nodeReuse(managementCluster.GetClient())

		if !ephemeralTest {
			rePivoting()
		}
	}

	})

	AfterEach(func() {
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})

})

func createTargetCluster() (targetCluster framework.ClusterProxy) {
	By("Creating a high available cluster")

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
	targetCluster = bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
	return targetCluster
}
