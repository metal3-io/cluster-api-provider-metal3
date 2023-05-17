package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	targetCluster            framework.ClusterProxy
	controlPlaneMachineCount int64
	workerMachineCount       int64
)

var _ = Describe("Testing features in ephemeral or target cluster [pivoting] [features]", func() {

	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	It("Should get a management cluster then test cert rotation and node reuse", func() {
		targetCluster, _ = createTargetCluster(e2eConfig.GetVariable("FROM_K8S_VERSION"))
		managementCluster := bootstrapClusterProxy
		// If not running ephemeral test, use the target cluster for management
		if !ephemeralTest {
			managementCluster = targetCluster
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
		}

		certRotation(ctx, func() CertRotationInput {
			return CertRotationInput{
				E2EConfig:         e2eConfig,
				ManagementCluster: managementCluster,
				SpecName:          specName,
			}
		})

		nodeReuse(ctx, func() NodeReuseInput {
			return NodeReuseInput{
				E2EConfig:         e2eConfig,
				ManagementCluster: managementCluster,
				TargetCluster:     targetCluster,
				SpecName:          specName,
				ClusterName:       clusterName,
				Namespace:         namespace,
			}
		})
	})

	AfterEach(func() {
		Logf("Logging state of bootstrap cluster")
		ListBareMetalHosts(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		ListMetal3Machines(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		ListMachines(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		ListNodes(ctx, bootstrapClusterProxy.GetClient())
		Logf("Logging state of target cluster")
		if !ephemeralTest {
			ListBareMetalHosts(ctx, targetCluster.GetClient(), client.InNamespace(namespace))
			ListMetal3Machines(ctx, targetCluster.GetClient(), client.InNamespace(namespace))
			ListMachines(ctx, targetCluster.GetClient(), client.InNamespace(namespace))
		}
		ListNodes(ctx, targetCluster.GetClient())
		// Abort the test in case of failure and keepTestEnv is true during keep VM trigger
		if CurrentSpecReport().Failed() {
			if keepTestEnv {
				AbortSuite("e2e test aborted and skip cleaning the VM", 4)
			}
		}
		if !ephemeralTest {
			// Dump the target cluster resources before re-pivoting.
			Logf("Dump the target cluster resources before re-pivoting")
			framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
				Lister:    targetCluster.GetClient(),
				Namespace: namespace,
				LogPath:   filepath.Join(artifactFolder, "clusters", clusterName, "resources"),
			})

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
		}
		DumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})

})

func createTargetCluster(k8sVersion string) (framework.ClusterProxy, *clusterctl.ApplyClusterTemplateAndWaitResult) {
	By("Creating a high available cluster")
	imageURL, imageChecksum := EnsureImage(k8sVersion)
	os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
	os.Setenv("IMAGE_RAW_URL", imageURL)
	controlPlaneMachineCount = int64(numberOfControlplane)
	workerMachineCount = int64(numberOfWorkers)
	result := clusterctl.ApplyClusterTemplateAndWaitResult{}
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
			KubernetesVersion:        k8sVersion,
			ControlPlaneMachineCount: &controlPlaneMachineCount,
			WorkerMachineCount:       &workerMachineCount,
		},
		WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
		WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
		WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
	}, &result)
	targetCluster := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
	framework.WaitForPodListCondition(ctx, framework.WaitForPodListConditionInput{
		Lister:      targetCluster.GetClient(),
		ListOptions: &client.ListOptions{LabelSelector: labels.Everything(), Namespace: "kube-system"},
		Condition:   framework.PhasePodCondition(corev1.PodRunning),
	}, e2eConfig.GetIntervals(specName, "wait-all-pod-to-be-running-on-target-cluster")...)
	return targetCluster, &result
}
