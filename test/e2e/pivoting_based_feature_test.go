package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		managementCluster := bootstrapClusterProxy
		// If not running ephemeral test, use the target cluster for management
		if !ephemeralTest {
			managementCluster = targetCluster
			pivoting()
		}

		certRotation(managementCluster.GetClientSet(), managementCluster.GetClient())
		nodeReuse(managementCluster.GetClient())

	})

	AfterEach(func() {
		if !ephemeralTest {
			// Dump the target cluster resources before re-pivoting.
			framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
				Lister:    targetCluster.GetClient(),
				Namespace: namespace,
				LogPath:   filepath.Join(artifactFolder, "clusters", clusterName, "resources"),
			})

			rePivoting()
		}
		Logf("Logging state of bootstrap cluster")
		listBareMetalHosts(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		listMetal3Machines(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		listMachines(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		listNodes(ctx, bootstrapClusterProxy.GetClient())
		Logf("Logging state of target cluster")
		if !ephemeralTest {
			listBareMetalHosts(ctx, targetCluster.GetClient(), client.InNamespace(namespace))
			listMetal3Machines(ctx, targetCluster.GetClient(), client.InNamespace(namespace))
			listMachines(ctx, targetCluster.GetClient(), client.InNamespace(namespace))
		}
		listNodes(ctx, targetCluster.GetClient())
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})

})

func createTargetCluster() (targetCluster framework.ClusterProxy) {
	By("Creating a high available cluster")

	controlPlaneMachineCount = int64(numberOfControlplane)
	workerMachineCount = int64(numberOfWorkers)
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
			KubernetesVersion:        e2eConfig.GetVariable("KUBERNETES_VERSION"),
			ControlPlaneMachineCount: &controlPlaneMachineCount,
			WorkerMachineCount:       &workerMachineCount,
		},
		WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
		WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
		WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
	}, result)
	targetCluster = bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
	return targetCluster
}
