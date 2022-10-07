package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1alpha4 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	framework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("When testing cluster upgrade [upgrade]", func() {
	upgradeManagementCluster()
})

func upgradeManagementCluster() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})
	var (
		ctx                 = context.TODO()
		specName            = "metal3"
		namespace           = "metal3"
		clusterName         = "test1"
		workloadClusterName = "test2"
		clusterctlLogFolder string
		kubernetesVersion   = e2eConfig.GetVariable(KubernetesVersion)
	)
	It("Should create a management cluster and then upgrade capi and capm3", func() {
		/*---------------------------------------------*
		| Create a target cluster with v1a4 clusterctl |
		*----------------------------------------------*/
		Logf("Starting v1a5 to v1b1 upgrade tests")

		clusterctlBinaryURLTemplate := e2eConfig.GetVariable("INIT_WITH_BINARY")
		clusterctlBinaryURLReplacer := strings.NewReplacer("{OS}", runtime.GOOS, "{ARCH}", runtime.GOARCH)
		clusterctlBinaryURL := clusterctlBinaryURLReplacer.Replace(clusterctlBinaryURLTemplate)
		clusterctlBinaryPath := "/tmp/clusterctltmp"

		Logf("Downloading clusterctl binary from %s", clusterctlBinaryURL)
		err := downloadFile(clusterctlBinaryPath, clusterctlBinaryURL)
		Expect(err).ToNot(HaveOccurred(), "failed to download temporary file")
		defer os.Remove(clusterctlBinaryPath) // clean up

		err = os.Chmod(clusterctlBinaryPath, 0744)
		Expect(err).ToNot(HaveOccurred(), "failed to chmod temporary file")

		By("Creating a cluster")
		Logf("Getting the cluster template yaml")
		upgradeClusterTemplate := clusterctl.ConfigClusterWithBinary(ctx, clusterctlBinaryPath, clusterctl.ConfigClusterInput{
			KubeconfigPath:       bootstrapClusterProxy.GetKubeconfigPath(),
			ClusterctlConfigPath: clusterctlConfigPath,
			// define template variables
			Namespace:                namespace,
			ClusterName:              clusterName,
			KubernetesVersion:        kubernetesVersion,
			Flavor:                   "ubuntu",
			ControlPlaneMachineCount: pointer.Int64Ptr(1),
			WorkerMachineCount:       pointer.Int64Ptr(1),
			InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
			// setup clusterctl logs folder
			LogFolder: clusterctlLogFolder,
		})

		Expect(upgradeClusterTemplate).ToNot(BeNil(), "Failed to get the cluster template")

		Logf("Applying the cluster template yaml to the cluster")

		Expect(bootstrapClusterProxy.Apply(ctx, upgradeClusterTemplate)).To(Succeed())

		By("Waiting for the machines to be running")
		// Note: We cannot use waitForNumMachinesInState because these are v1alpha4 Machines!
		Eventually(func(g Gomega) {
			n := 0
			machineList := &clusterv1alpha4.MachineList{}
			opts := []client.ListOption{
				client.InNamespace(namespace),
				client.MatchingLabels{clusterv1.ClusterLabelName: clusterName},
			}
			g.Expect(bootstrapClusterProxy.GetClient().List(ctx, machineList, opts...)).To(Succeed())
			for _, machine := range machineList.Items {
				if machine.Status.GetTypedPhase() == clusterv1alpha4.MachinePhaseRunning {
					n++
				}
			}
			g.Expect(n).To(Equal(2))
		}, e2eConfig.GetIntervals(specName, "wait-worker-nodes")...).Should(Succeed())

		upgradeClusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
		upgradeClusterClient := upgradeClusterProxy.GetClient()

		By("Initializing the workload cluster with older versions of providers")

		contract := e2eConfig.GetVariable("CAPI_VERSION")

		clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
			ClusterctlBinaryPath:    clusterctlBinaryPath, // use older version of clusterctl to init the management cluster
			ClusterProxy:            upgradeClusterProxy,
			ClusterctlConfigPath:    clusterctlConfigPath,
			CoreProvider:            e2eConfig.GetProviderLatestVersionsByContract(contract, config.ClusterAPIProviderName)[0],
			BootstrapProviders:      e2eConfig.GetProviderLatestVersionsByContract(contract, config.KubeadmBootstrapProviderName),
			ControlPlaneProviders:   e2eConfig.GetProviderLatestVersionsByContract(contract, config.KubeadmControlPlaneProviderName),
			InfrastructureProviders: e2eConfig.GetProviderLatestVersionsByContract(contract, e2eConfig.InfrastructureProviders()...),
			LogFolder:               filepath.Join(artifactFolder, "clusters", clusterName),
		}, e2eConfig.GetIntervals(specName, "wait-controllers")...)

		/*-------------------------------------------------*
		| Pivot to run ironic/BMO and resources on target |
		*--------------------------------------------------*/
		By("Start pivoting")
		listBareMetalHosts(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		listNodes(ctx, upgradeClusterClient)

		By("Remove Ironic containers from the source cluster")
		ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
		if ephemeralCluster == KIND {
			removeIronicContainers()
		} else {
			removeIronicDeployment()
		}

		By("Create Ironic namespace")
		upgradeClusterClientSet := upgradeClusterProxy.GetClientSet()
		ironicNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: e2eConfig.GetVariable("IRONIC_NAMESPACE"),
			},
		}
		_, err = upgradeClusterClientSet.CoreV1().Namespaces().Create(ctx, ironicNamespace, metav1.CreateOptions{})
		Expect(err).To(BeNil(), "Unable to create the Ironic namespace")

		By("Add labels to BMO CRDs")
		labelBMOCRDs(nil)

		By("Install BMO")
		installIronicBMO(upgradeClusterProxy, "false", "true")

		By("Install Ironic in the target cluster")
		installIronicBMO(upgradeClusterProxy, "true", "false")

		By("Add labels to BMO CRDs in the target cluster")
		labelBMOCRDs(upgradeClusterProxy)

		By("Ensure API servers are stable before doing move")
		Consistently(func() error {
			kubeSystem := &corev1.Namespace{}
			return bootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
		}, "5s", "100ms").Should(BeNil(), "Failed to assert bootstrap API server stability")
		Consistently(func() error {
			kubeSystem := &corev1.Namespace{}
			return upgradeClusterClient.Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
		}, "5s", "100ms").Should(BeNil(), "Failed to assert target API server stability")
		By("Moving the cluster to self hosted")

		cmd := exec.Command(clusterctlBinaryPath, "move", "--to-kubeconfig", upgradeClusterProxy.GetKubeconfigPath(), "-n", namespace, "-v", "10")
		output, err := cmd.CombinedOutput()
		Logf("move: %v", string(output))
		Expect(err).To(BeNil())

		By("Check that BMHs are in provisioned state")
		waitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, waitForNumInput{
			Client:    upgradeClusterClient,
			Options:   []client.ListOption{client.InNamespace(namespace)},
			Replicas:  2,
			Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-provisioned"),
		})

		listBareMetalHosts(ctx, upgradeClusterClient, client.InNamespace(namespace))
		listNodes(ctx, upgradeClusterClient)

		Logf("Apply the available BMHs CRs")
		cmd = exec.Command("kubectl", "apply", "-f", "/opt/metal3-dev-env/bmhosts_crs.yaml", "-n", namespace, "--kubeconfig", upgradeClusterProxy.GetKubeconfigPath())
		output, err = cmd.CombinedOutput()
		Logf("Apply BMHs CRs: %v", string(output))
		Expect(err).To(BeNil())

		Logf("Waiting for 2 BMHs to be in Available state")
		waitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, waitForNumInput{
			Client:    upgradeClusterClient,
			Options:   []client.ListOption{client.InNamespace(namespace)},
			Replicas:  2,
			Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
		})

		listBareMetalHosts(ctx, upgradeClusterClient, client.InNamespace(namespace))
		listNodes(ctx, upgradeClusterClient)

		/*-------------------------------*
		| Create a test workload cluster |
		*--------------------------------*/
		var (
			controlPlaneMachineCount = pointer.Int64Ptr(1)
			workerMachineCount       = pointer.Int64Ptr(1)
		)
		// Update CLUSTER_APIENDPOINT_HOST
		os.Setenv("CLUSTER_APIENDPOINT_HOST", "192.168.111.250")

		By("Creating a test workload cluster")

		Logf("Creating the workload cluster with name %q using (Kubernetes %s, %d control-plane machines, %d worker machines)",
			workloadClusterName, kubernetesVersion, *controlPlaneMachineCount, *workerMachineCount)

		Logf("Getting the cluster template yaml")
		workloadClusterTemplate := clusterctl.ConfigClusterWithBinary(ctx, clusterctlBinaryPath, clusterctl.ConfigClusterInput{
			// pass reference to the management cluster hosting this test
			KubeconfigPath: upgradeClusterProxy.GetKubeconfigPath(),
			// pass the clusterctl config file that points to the local provider repository created for this test
			ClusterctlConfigPath: clusterctlConfigPath,

			// define template variables
			Namespace:                namespace,
			ClusterName:              workloadClusterName,
			Flavor:                   "upgrade-workload",
			KubernetesVersion:        kubernetesVersion,
			ControlPlaneMachineCount: controlPlaneMachineCount,
			WorkerMachineCount:       workerMachineCount,
			InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
			// setup clusterctl logs folder
			LogFolder: filepath.Join(artifactFolder, "clusters", upgradeClusterProxy.GetName()),
		})
		Expect(workloadClusterTemplate).ToNot(BeNil(), "Failed to get the cluster template")

		Logf("Applying the cluster template yaml to the cluster")
		Expect(upgradeClusterProxy.Apply(ctx, workloadClusterTemplate)).To(Succeed())

		By("Waiting for the machines to be running")
		// Note: We cannot use waitForNumMachinesInState because these are v1alpha4 Machines!
		Eventually(func(g Gomega) {
			n := 0
			machineList := &clusterv1alpha4.MachineList{}
			opts := []client.ListOption{
				client.InNamespace(namespace),
				client.MatchingLabels{clusterv1.ClusterLabelName: workloadClusterName},
			}
			g.Expect(upgradeClusterClient.List(ctx, machineList, opts...)).To(Succeed())
			for _, machine := range machineList.Items {
				if machine.Status.GetTypedPhase() == clusterv1alpha4.MachinePhaseRunning {
					n++
				}
			}
			g.Expect(n).To(Equal(2))
		}, e2eConfig.GetIntervals(specName, "wait-machine-running")...).Should(Succeed())

		listBareMetalHosts(ctx, upgradeClusterClient, client.InNamespace(namespace))

		By("THE MANAGEMENT CLUSTER WITH OLDER VERSION OF PROVIDERS WORKS!")

		/*--------------------------------------*
		| Upgrade Ironic and BareMetalOperator  |
		*---------------------------------------*/

		// TODO: If/when we rework the upgrade test to be able to use CAPI e2e
		// upgrade test, these two should be included as a PreUpgrade step.
		// If we do not go that route, we should instead refactor the whole
		// upgradeManagementCluster function into smaller parts.
		upgradeIronic(upgradeClusterProxy.GetClientSet())
		upgradeBMO(upgradeClusterProxy.GetClientSet())

		/*-------------------------------*
		| Upgrade the management cluster |
		*--------------------------------*/
		By("Upgrading providers to the latest version available")
		By("Get the management cluster images before upgrading")
		printImages(upgradeClusterProxy)

		Logf("Upgrade management cluster to : %v", clusterv1.GroupVersion.Version)
		clusterctl.UpgradeManagementClusterAndWait(ctx, clusterctl.UpgradeManagementClusterAndWaitInput{
			ClusterctlConfigPath: clusterctlConfigPath,
			ClusterProxy:         upgradeClusterProxy,
			Contract:             clusterv1.GroupVersion.Version,
			LogFolder:            filepath.Join(artifactFolder, "clusters", workloadClusterName),
		}, e2eConfig.GetIntervals(specName, "wait-controllers")...)

		Logf("The target cluster upgraded!")

		By("Get the management cluster images after upgrading")
		printImages(upgradeClusterProxy)

		By("Upgrade bootstrap cluster")
		By("Get the management cluster images before upgrading")
		printImages(bootstrapClusterProxy)

		Logf("Upgrade bootstrap cluster to : %v", clusterv1.GroupVersion.Version)
		clusterctl.UpgradeManagementClusterAndWait(ctx, clusterctl.UpgradeManagementClusterAndWaitInput{
			ClusterctlConfigPath: clusterctlConfigPath,
			ClusterProxy:         bootstrapClusterProxy,
			Contract:             clusterv1.GroupVersion.Version,
			LogFolder:            filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
		}, e2eConfig.GetIntervals(specName, "wait-controllers")...)

		Logf("The source cluster upgraded!")

		By("Get the bootstrap cluster images after upgrading")
		printImages(bootstrapClusterProxy)

		By("UPGRADE MANAGEMENT CLUSTER PASSED!")
	})

	AfterEach(func() {
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})
}

func printImages(clusterProxy framework.ClusterProxy) {
	pods, err := clusterProxy.GetClientSet().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	Expect(err).To(BeNil())
	var images []string
	for _, pod := range pods.Items {
		for _, c := range pod.Spec.Containers {
			exist := false
			for _, i := range images {
				if i == c.Image {
					exist = true
					break
				}
			}
			if !exist {
				images = append(images, c.Image)
				Logf("%v", c.Image)
			}
		}
	}
}
