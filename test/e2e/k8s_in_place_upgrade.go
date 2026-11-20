package e2e

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// inPlaceUpgradeFocus is the GINKGO_FOCUS (and test label) of the in-place
	// upgrade scenario.
	inPlaceUpgradeFocus = "in-place-upgrade"
	// testExtensionProviderName is the RuntimeExtensionProvider name in
	// test/e2e/config/e2e_conf.yaml.
	testExtensionProviderName = "test-extension"
	// testExtensionNamespace is the namespace the runtime extension is deployed
	// into.
	testExtensionNamespace = "test-extension-system"
	// testExtensionSSHSecretName is the secret holding the private key the
	// extension uses to reach the nodes. It is mounted at
	// /home/nonroot/.ssh/id_rsa by test/extension/config/default/manager_ssh_patch.yaml.
	testExtensionSSHSecretName = "ssh-key"
)

// CreateTestExtensionSSHKeySecret creates the namespace of the in-place update
// runtime extension and the ssh-key secret its pod mounts. The extension
// upgrades Kubernetes over SSH, so it needs the private key matching
// SSH_PUB_KEY_CONTENT (the key provisioned onto the nodes).
func CreateTestExtensionSSHKeySecret(ctx context.Context, clusterProxy framework.ClusterProxy) {
	c := clusterProxy.GetClient()

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
}

// cpMachineHealthy returns true if the Machine is a control plane Machine that
// passes the KCP preflight checks: it has a Node and all control plane
// component and etcd conditions are true.
func cpMachineHealthy(machine clusterv1.Machine) bool {
	if _, isControlPlane := machine.GetLabels()[clusterv1.MachineControlPlaneLabel]; !isControlPlane {
		return false
	}
	if !machine.Status.NodeRef.IsDefined() {
		return false
	}
	for _, conditionType := range []string{
		controlplanev1.KubeadmControlPlaneMachineAPIServerPodHealthyCondition,
		controlplanev1.KubeadmControlPlaneMachineControllerManagerPodHealthyCondition,
		controlplanev1.KubeadmControlPlaneMachineSchedulerPodHealthyCondition,
		controlplanev1.KubeadmControlPlaneMachineEtcdPodHealthyCondition,
		controlplanev1.KubeadmControlPlaneMachineEtcdMemberHealthyCondition,
	} {
		if !conditions.IsTrue(&machine, conditionType) {
			return false
		}
	}
	return true
}

// cpStableDuration is how long all CP Machines must stay healthy before the
// test starts the in-place upgrade.
const cpStableDuration = 2 * time.Minute

// WaitForStableControlPlane waits until the number of healthy CP Machines is
// equal to replicas, and stays equal to replicas for cpStableDuration. Each
// failed check starts the stable period again.
func WaitForStableControlPlane(ctx context.Context, c client.Client, namespace string, replicas int, intervals ...interface{}) {
	var healthySince time.Time
	Eventually(func(g Gomega) {
		machineList := clusterv1.MachineList{}
		g.Expect(c.List(ctx, &machineList, client.InNamespace(namespace))).To(Succeed())
		healthy := len(FilterMachines(machineList.Items, cpMachineHealthy))
		if healthy != replicas {
			healthySince = time.Time{}
			g.Expect(healthy).To(Equal(replicas), "Not all CP machines are healthy")
		}
		if healthySince.IsZero() {
			healthySince = time.Now()
		}
		g.Expect(time.Since(healthySince)).To(BeNumerically(">=", cpStableDuration),
			"CP machines are healthy, but not for %s yet", cpStableDuration)
	}, intervals...).Should(Succeed())
	ListMachines(ctx, c, client.InNamespace(namespace))
}

type InPlaceUpgradeInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
	ArtifactFolder        string
}

// fetchInPlaceUpgradeArtifacts collects the pod logs and the manifests of the
// bootstrap and the target cluster into <ArtifactFolder>/<stage>. A failure
// to collect is logged and does not stop the test.
func fetchInPlaceUpgradeArtifacts(input InPlaceUpgradeInput, stage string) {
	outputPath := filepath.Join(input.ArtifactFolder, stage)
	for _, proxy := range []framework.ClusterProxy{input.BootstrapClusterProxy, input.TargetCluster} {
		Byf("Fetch logs from %s cluster (%s)", proxy.GetName(), stage)
		if err := FetchClusterLogs(proxy, outputPath); err != nil {
			Logf("Error fetching logs for %s cluster (%s): %v", proxy.GetName(), stage, err)
		}

		Byf("Fetch manifests for %s cluster (%s)", proxy.GetName(), stage)
		if err := FetchManifests(proxy, outputPath); err != nil {
			Logf("Error fetching manifests for %s cluster (%s): %v", proxy.GetName(), stage, err)
		}
	}
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
	fetchInPlaceUpgradeArtifacts(input, "beforeStableWait")

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

	Byf("Wait for %d CP machine(s) to stay healthy for %s", numberOfControlplane, cpStableDuration)
	WaitForStableControlPlane(ctx, managementClusterClient, input.Namespace, numberOfControlplane,
		input.E2EConfig.GetIntervals(input.SpecName, "wait-control-plane")...)

	fetchInPlaceUpgradeArtifacts(input, "afterStableWait")

	// The log collection above takes time. Check the CP health again
	// immediately before the KCP patch.
	Byf("Check that %d CP machine(s) are still healthy", numberOfControlplane)
	WaitForNumMachines(ctx, cpMachineHealthy, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfControlplane,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-control-plane"),
	})

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

	fetchInPlaceUpgradeArtifacts(input, "afterUpgrade")

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
