package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Kubernetes version upgrade in target nodes [k8s-upgrade]", Label("k8s-upgrade"), func() {

	var (
		ctx                 = context.TODO()
		specName            = "metal3"
		namespace           = "metal3"
		clusterName         = "test1"
		clusterctlLogFolder string
	)

	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "target_cluster_logs", bootstrapClusterProxy.GetName())
	})

	It("Should create a cluster and run k8s_upgrade tests", func() {
		By("Apply BMH for workload cluster")
		ApplyBmh(ctx, e2eConfig, bootstrapClusterProxy, namespace, specName)
		By("Creating target cluster")
		targetCluster, _ = CreateTargetCluster(ctx, func() CreateTargetClusterInput {
			return CreateTargetClusterInput{
				E2EConfig:             e2eConfig,
				BootstrapClusterProxy: bootstrapClusterProxy,
				SpecName:              specName,
				ClusterName:           clusterName,
				K8sVersion:            e2eConfig.MustGetVariable("FROM_K8S_VERSION"),
				KCPMachineCount:       int64(numberOfControlplane),
				WorkerMachineCount:    int64(numberOfWorkers),
				ClusterctlLogFolder:   clusterctlLogFolder,
				ClusterctlConfigPath:  clusterctlConfigPath,
				OSType:                osType,
				Namespace:             namespace,
			}
		})

		By("Running Kubernetes Upgrade tests")
		upgradeKubernetes(ctx, func() upgradeKubernetesInput {
			return upgradeKubernetesInput{
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

type upgradeKubernetesInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
}

// upgradeKubernetes implements a test upgrading the cluster nodes from an old k8s version to a newer version.
func upgradeKubernetes(ctx context.Context, inputGetter func() upgradeKubernetesInput) {
	Logf("Starting Kubernetes upgrade tests")
	input := inputGetter()
	clusterClient := input.BootstrapClusterProxy.GetClient()
	targetClusterClient := input.TargetCluster.GetClient()
	clientSet := input.TargetCluster.GetClientSet()
	kubernetesVersion := input.E2EConfig.MustGetVariable("FROM_K8S_VERSION")
	upgradedK8sVersion := input.E2EConfig.MustGetVariable("KUBERNETES_VERSION")
	numberOfWorkers := int(*input.E2EConfig.MustGetInt32PtrVariable("WORKER_MACHINE_COUNT"))
	numberOfControlplane := int(*input.E2EConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))

	var (
		controlplaneTaints = []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule}}
	)

	Logf("KUBERNETES VERSION: %v", kubernetesVersion)
	Logf("UPGRADED K8S VERSION: %v", upgradedK8sVersion)
	Logf("NUMBER OF CONTROLPLANE BMH: %v", numberOfControlplane)
	Logf("NUMBER OF WORKER BMH: %v", numberOfWorkers)

	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, clusterClient, client.InNamespace(namespace))
	ListMachines(ctx, clusterClient, client.InNamespace(namespace))
	ListNodes(ctx, targetClusterClient)

	// Download node image
	By("Download image")
	imageURL, imageChecksum := EnsureImage(upgradedK8sVersion)

	By("Create new KCP Metal3MachineTemplate with upgraded image to boot")
	m3MachineTemplateName := clusterName + "-controlplane"
	newM3MachineTemplateName := clusterName + "-new-controlplane"
	CreateNewM3MachineTemplate(ctx, namespace, newM3MachineTemplateName, m3MachineTemplateName, clusterClient, imageURL, imageChecksum)

	Byf("Update KCP to upgrade k8s version and binaries from %s to %s", kubernetesVersion, upgradedK8sVersion)
	kcpObj := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      clusterClient,
		ClusterName: clusterName,
		Namespace:   namespace,
	})
	helper, err := patch.NewHelper(kcpObj, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.MachineTemplate.InfrastructureRef.Name = newM3MachineTemplateName
	kcpObj.Spec.Version = upgradedK8sVersion
	kcpObj.Spec.RolloutStrategy.RollingUpdate.MaxSurge.IntVal = 0
	Expect(helper.Patch(ctx, kcpObj)).To(Succeed())

	Byf("Wait until %d BMH(s) in deprovisioning state", 1)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateDeprovisioning, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  1,
		Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning"),
	})

	Byf("Wait until three Control Plane machines become running and updated with the new %s k8s version", upgradedK8sVersion)
	runningAndUpgraded := func(machine clusterv1.Machine) bool {
		running := machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning
		upgraded := *machine.Spec.Version == upgradedK8sVersion
		return (running && upgraded)
	}
	WaitForNumMachines(ctx, runningAndUpgraded, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  numberOfControlplane,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-running"),
	})

	By("Untaint CP nodes")
	controlplaneNodes := getControlplaneNodes(ctx, clientSet)
	untaintNodes(ctx, targetClusterClient, controlplaneNodes, controlplaneTaints)

	By("Put maxSurge field in KubeadmControlPlane back to default value(1)")
	kcpObj = framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      clusterClient,
		ClusterName: clusterName,
		Namespace:   namespace,
	})
	helper, err = patch.NewHelper(kcpObj, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.RolloutStrategy.RollingUpdate.MaxSurge.IntVal = 1
	for range 3 {
		err = helper.Patch(ctx, kcpObj)
		if err == nil {
			break
		}
		time.Sleep(30 * time.Second)
	}

	By("Get MachineDeployment")
	machineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      clusterClient,
		ClusterName: clusterName,
		Namespace:   namespace,
	})
	Expect(machineDeployments).To(HaveLen(1), "Expected exactly 1 MachineDeployment")
	machineDeploy := machineDeployments[0]

	By("Create new Metal3MachineTemplate for MD with upgraded image to boot")
	m3MachineTemplateName = clusterName + "-workers"
	newM3MachineTemplateName = clusterName + "-new-workers"
	CreateNewM3MachineTemplate(ctx, namespace, newM3MachineTemplateName, m3MachineTemplateName, clusterClient, imageURL, imageChecksum)

	Byf("Update MD to upgrade k8s version and binaries from %s to %s", kubernetesVersion, upgradedK8sVersion)
	helper, err = patch.NewHelper(machineDeploy, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	machineDeploy.Spec.Strategy.RollingUpdate.MaxSurge.IntVal = 0
	machineDeploy.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal = 1
	machineDeploy.Spec.Template.Spec.InfrastructureRef.Name = newM3MachineTemplateName
	machineDeploy.Spec.Template.Spec.Version = &upgradedK8sVersion
	Expect(helper.Patch(ctx, machineDeploy)).To(Succeed())

	Byf("Wait until %d BMH(s) in deprovisioning state", 1)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateDeprovisioning, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  1,
		Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning"),
	})

	Byf("Wait until %d BMH(s) in provisioning state", 1)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioning, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  1,
		Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning"),
	})

	Byf("Wait until the worker BMH becomes provisioned")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-provisioned"),
	})

	Byf("Wait until the worker machine becomes running")
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-running"),
	})

	// Verify that all nodes is using the k8s version
	Byf("Verify all machines become running and updated with new %s k8s version", upgradedK8sVersion)
	WaitForNumMachines(ctx, runningAndUpgraded, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-running"),
	})

	By("KUBERNETES UPGRADE TESTS PASSED!")
}
