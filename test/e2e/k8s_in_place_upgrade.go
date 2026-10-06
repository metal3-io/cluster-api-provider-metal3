package e2e

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// testExtensionNamespace is the namespace the runtime extension is deployed
	// into. It must match the namespace in test/extension/config/default and the
	// clientConfig.service of the ExtensionConfig.
	testExtensionNamespace = "test-extension-system"
	// testExtensionDeploymentName is the extension deployment, i.e. the
	// "controller-manager" of test/extension/config/default with its namePrefix.
	testExtensionDeploymentName = "test-extension-controller-manager"
	// testExtensionSSHSecretName is the secret holding the private key the
	// extension uses to reach the nodes. It is mounted at
	// /home/nonroot/.ssh/id_rsa by test/extension/config/default/manager_ssh_patch.yaml.
	testExtensionSSHSecretName = "ssh-key"
)

// DeployTestExtensionInput provides input for DeployTestExtension().
type DeployTestExtensionInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	SpecName              string
	LogFolder             string
}

// DeployTestExtension deploys the in-place update runtime extension to the
// bootstrap cluster. The extension upgrades Kubernetes over SSH, so it needs the
// private key matching SSH_PUB_KEY_CONTENT (the key provisioned onto the nodes)
// available as a secret. Namespace and secret are created here rather than in
// scripts/ci-e2e.sh because the bootstrap cluster only exists once the test
// framework has created it.
func DeployTestExtension(ctx context.Context, inputGetter func() DeployTestExtensionInput) {
	input := inputGetter()
	c := input.BootstrapClusterProxy.GetClient()

	By("Ensure the test-extension namespace exists")
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testExtensionNamespace}}
	err := c.Create(ctx, ns)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		Expect(err).ToNot(HaveOccurred(), "Failed to create namespace %s", testExtensionNamespace)
	}

	By("Create the ssh-key secret the test-extension uses to reach the nodes")
	home, err := os.UserHomeDir()
	Expect(err).ToNot(HaveOccurred(), "Failed to resolve the home directory")
	keyPath := filepath.Join(filepath.Clean(home), ".ssh", "id_rsa")
	privateKey, err := os.ReadFile(keyPath) //#nosec G304:gosec
	Expect(err).ToNot(HaveOccurred(), "Failed to read the ssh private key from %s", keyPath)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testExtensionSSHSecretName,
			Namespace: testExtensionNamespace,
		},
		Data: map[string][]byte{"id_rsa": privateKey},
	}
	// Recreate the secret so a regenerated key is always picked up.
	err = c.Delete(ctx, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred(), "Failed to delete the existing %s secret", testExtensionSSHSecretName)
	}
	Expect(c.Create(ctx, secret)).To(Succeed(), "Failed to create the %s secret", testExtensionSSHSecretName)

	By("Deploy the test-extension components")
	Expect(BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       input.E2EConfig.MustGetVariable("TEST_EXTENSION_KUSTOMIZATION"),
		ClusterProxy:        input.BootstrapClusterProxy,
		WaitForDeployment:   true,
		WatchDeploymentLogs: true,
		DeploymentName:      testExtensionDeploymentName,
		DeploymentNamespace: testExtensionNamespace,
		LogPath:             input.LogFolder,
		WaitIntervals:       input.E2EConfig.GetIntervals(input.SpecName, "wait-deployment"),
	})).To(Succeed(), "Failed to deploy the test-extension")
}

type InPlaceUpgradeInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
}

func InPlaceUpgrade(ctx context.Context, inputGetter func() InPlaceUpgradeInput) {
	Logf("Starting in-place Kubernetes upgrade tests")
	input := inputGetter()
	managementClusterClient := input.BootstrapClusterProxy.GetClient()
	targetClusterClient := input.TargetCluster.GetClient()
	upgradedK8sVersion := input.E2EConfig.MustGetVariable("KUBERNETES_VERSION")
	fromK8sVersion := input.E2EConfig.MustGetVariable("KUBERNETES_VERSION_UPGRADE_FROM")
	numberOfControlplane := int(*input.E2EConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))

	Logf("FROM K8S VERSION: %v", fromK8sVersion)
	Logf("UPGRADED K8S VERSION: %v", upgradedK8sVersion)
	Logf("NUMBER OF CONTROLPLANE: %v", numberOfControlplane)

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	// Get the initial machine UIDs before upgrade to verify in-place upgrade (no rollout)
	By("Capture machine UIDs before upgrade to verify no rollout occurs")
	machineList := &clusterv1.MachineList{}
	Expect(managementClusterClient.List(ctx, machineList, client.InNamespace(input.Namespace))).To(Succeed())
	initialMachineUIDs := make(map[string]string) // map[machineName]UID
	for _, machine := range machineList.Items {
		initialMachineUIDs[machine.Name] = string(machine.UID)
		Logf("Tracking machine %s with UID %s", machine.Name, machine.UID)
	}
	Logf("Captured %d machine UIDs before upgrade", len(initialMachineUIDs))

	// Download and ensure node image is available locally
	By("Download and ensure image is available locally")
	imageURL, imageChecksum := EnsureImage(input.E2EConfig, upgradedK8sVersion)

	Logf("Image URL: %s", imageURL)
	Logf("Image Checksum: %s", imageChecksum)

	// Get the cluster object
	By("Get the Cluster object")
	cluster := &clusterv1.Cluster{}
	Expect(managementClusterClient.Get(ctx, client.ObjectKey{
		Namespace: input.Namespace,
		Name:      input.ClusterName,
	}, cluster)).To(Succeed())

	By("Create new KCP Metal3MachineTemplate with upgraded image to boot")
	m3MachineTemplateName := input.ClusterName + "-controlplane"
	newM3MachineTemplateName := input.ClusterName + "-new-controlplane"
	CreateNewM3MachineTemplate(ctx, input.Namespace, newM3MachineTemplateName, m3MachineTemplateName, managementClusterClient, imageURL, imageChecksum)

	Byf("Update KCP to upgrade k8s version and binaries from %s to %s", fromK8sVersion, upgradedK8sVersion)
	kcpObj := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      managementClusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	helper, err := patch.NewHelper(kcpObj, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.MachineTemplate.Spec.InfrastructureRef.Name = newM3MachineTemplateName
	kcpObj.Spec.Version = upgradedK8sVersion
	kcpObj.Spec.Rollout.Strategy.RollingUpdate.MaxSurge.IntVal = 0
	Expect(helper.Patch(ctx, kcpObj)).To(Succeed())

	// Wait for CP nodes to be upgraded
	Byf("Wait for %d CP node(s) to be upgraded and running", numberOfControlplane)
	runningAndUpgraded := func(machine clusterv1.Machine) bool {
		running := machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning
		upgraded := machine.Spec.Version == upgradedK8sVersion
		_, isControlPlane := machine.GetLabels()[clusterv1.MachineControlPlaneLabel]
		return running && upgraded && isControlPlane
	}
	WaitForNumMachines(ctx, runningAndUpgraded, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfControlplane,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	Logf("CP nodes upgraded successfully to %s", upgradedK8sVersion)

	// Verify CP machines were not replaced (in-place upgrade)
	By("Verify CP machines were not replaced (no rollout)")
	cpMachineList := &clusterv1.MachineList{}
	Expect(managementClusterClient.List(ctx, cpMachineList,
		client.InNamespace(input.Namespace),
		client.MatchingLabels{clusterv1.MachineControlPlaneLabel: ""})).To(Succeed())
	for _, machine := range cpMachineList.Items {
		initialUID, exists := initialMachineUIDs[machine.Name]
		Expect(exists).To(BeTrue(), "CP machine %s should exist in initial machine list", machine.Name)
		Expect(string(machine.UID)).To(Equal(initialUID),
			"CP machine %s UID should not change (expected: %s, got: %s) - in-place upgrade should not replace machines",
			machine.Name, initialUID, machine.UID)
		Logf("✓ CP machine %s has same UID - confirmed in-place upgrade", machine.Name)
	}

	// Scale up control plane nodes to 5
	By("Scale up control plane nodes to 5")

	kcpObj = framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      managementClusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	helper, err = patch.NewHelper(kcpObj, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.Replicas = ptr.To[int32](5)
	Expect(helper.Patch(ctx, kcpObj)).To(Succeed())

	Logf("Control plane nodes scaled to 5 replicas")
	// Wait for all 5 CP nodes to be running with new version
	By("Wait for all 5 CP nodes to be running with new K8s version")
	WaitForNumMachines(ctx, runningAndUpgraded, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  5,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})
	Logf("All 5 control plane nodes running successfully with %s", upgradedK8sVersion)

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	By("IN-PLACE K8S UPGRADE TESTS PASSED!")
}
