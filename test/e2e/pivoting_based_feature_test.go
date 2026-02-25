package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ctx                 = context.TODO()
	specName            = "metal3"
	namespace           = "metal3"
	clusterName         = "test1"
	clusterctlLogFolder string
	targetCluster       framework.ClusterProxy
)

/*
 * Pivoting-based feature tests
 * This test evaluates capm3 feature in a pivoted cluster, which means migrating resources and control components from
 * bootsrap to target cluster
 *
 * Tested Features:
 * - Create a workload cluster
 * - Pivot to self-hosted
 * - Test Certificate Rotation
 * - Test Node Reuse
 *
 * Pivot to self-hosted:
 * - the Ironic containers removed from the source cluster, and a new Ironic namespace is created in the target cluster.
 * - The provider components are initialized in the target cluster using `clusterctl.Init`.
 * - Ironic is installed in the target cluster, followed by the installation of BMO.
 * - The stability of the API servers is checked before proceeding with the move to self-hosted.
 * - The cluster is moved to self-hosted using `clusterctl.Move`.
 * - After the move, various checks are performed to ensure that the cluster resources are in the expected state.
 * - If all checks pass, the test is considered successful.
 *
 * Certificate Rotation:
 * This test ensures that certificate rotation in the Ironic pod is functioning correctly.
 * by forcing certificate regeneration and verifying container restarts
 * - It starts by checking if the Ironic pod is running. It retrieves the Ironic deployment and waits for the pod to be in the "Running" state.
 * - The test forces cert-manager to regenerate the certificates by deleting the relevant secrets.
 * - It then waits for the containers in the Ironic pod to be restarted. It checks if each container exists and compares the restart count with the previously recorded values.
 * - If all containers are restarted successfully, the test passes.
 *
 * Node Reuse:
 * This test verifies the feature of reusing the same node after upgrading Kubernetes version in KubeadmControlPlane (KCP) and MachineDeployment (MD) nodes.
 * Note that while other controlplane providers are expected to work only KubeadmControlPlane is currently tested.
 * - The test starts with a cluster containing 3 KCP (Kubernetes control plane) nodes and 1 MD (MachineDeployment) node.
 * - The control plane nodes are untainted to allow scheduling new pods on them.
 * - The MachineDeployment is scaled down to 0 replicas, ensuring that all worker nodes will be deprovisioned. This provides 1 BMH (BareMetalHost) available for reuse during the upgrade.
 * - The code waits for one BareMetalHost (BMH) to become available, indicating that one worker node is deprovisioned and available for reuse.
 * - The names and UUIDs of the provisioned BMHs before the upgrade are obtained.
 * - An image is downloaded, and the nodeReuse field is set to True for the existing KCP Metal3MachineTemplate to reuse the node.
 * - A new Metal3MachineTemplate with the upgraded image is created for the KCP.
 * - The KCP is updated to upgrade the Kubernetes version and binaries. The rolling update strategy is set to update one machine at a time (MaxSurge: 0).
 * - The code waits for one machine to enter the deleting state and ensures that no new machines are in the provisioning state.
 * - The code waits for the deprovisioning BMH to become available again.
 * - It checks if the deprovisioned BMH is reused for the next provisioning.
 * - The code waits for the second machine to become running and updated with the new Kubernetes version.
 * - The upgraded control plane nodes are untainted to allow scheduling worker pods on them.
 * - Check all control plane nodes become running and update with the new Kubernetes version.
 * - The names and UUIDs of the provisioned BMHs after the upgrade are obtained.
 * - The difference between the mappings before and after the upgrade is checked to ensure that the same BMHs were reused.
 * - Similar steps are performed to test machine deployment node reuse.
 *
 * Finally, the cluster is re-pivoted and cleaned up.
 */

var _ = Describe("Testing features in target cluster", Label("pivoting", "features"),
	func() {

		BeforeEach(func() {
			osType = strings.ToLower(os.Getenv("OS"))
			Expect(osType).ToNot(Equal(""))
			validateGlobals(specName)

			// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
			clusterctlLogFolder = filepath.Join(os.TempDir(), "target_cluster_logs", bootstrapClusterProxy.GetName())
		})

		It("Should get a management cluster then test cert rotation and node reuse", func() {
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
			managementCluster := targetCluster
			Pivoting(ctx, func() PivotingInput {
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

			CertRotation(ctx, func() CertRotationInput {
				return CertRotationInput{
					E2EConfig:    e2eConfig,
					ClusterProxy: managementCluster,
					SpecName:     specName,
				}
			})

			NodeReuse(ctx, func() NodeReuseInput {
				return NodeReuseInput{
					E2EConfig:      e2eConfig,
					ClusterProxy:   managementCluster,
					SpecName:       specName,
					ClusterName:    clusterName,
					Namespace:      namespace,
					ArtifactFolder: artifactFolder,
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
			ListBareMetalHosts(ctx, targetCluster.GetClient(), client.InNamespace(namespace))
			ListMetal3Machines(ctx, targetCluster.GetClient(), client.InNamespace(namespace))
			ListMachines(ctx, targetCluster.GetClient(), client.InNamespace(namespace))
			ListNodes(ctx, targetCluster.GetClient())
			// Dump the target cluster resources before re-pivoting.
			Logf("Dump the target cluster resources before re-pivoting")
			framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
				Lister:               targetCluster.GetClient(),
				Namespace:            namespace,
				LogPath:              filepath.Join(artifactFolder, "clusters", clusterName, "resources"),
				KubeConfigPath:       targetCluster.GetKubeconfigPath(),
				ClusterctlConfigPath: clusterctlConfigPath,
			})

			RePivoting(ctx, func() RePivotingInput {
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
			DumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, targetCluster, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup, clusterctlConfigPath)
		})

	})
