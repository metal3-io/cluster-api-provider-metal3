package e2e

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
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

// testExtensionSSHPubKeyVar is the cluster template variable for the public
// key of the test-extension.
const testExtensionSSHPubKeyVar = "IN_PLACE_UPGRADE_SSH_PUB_KEY"

// testExtensionSSHPubKey is the authorized_keys line of the temporary key of
// the test-extension.
var testExtensionSSHPubKey string

// GenerateTestExtensionSSHKey generates a temporary ed25519 key pair for the
// in-place upgrade test. It returns the private key in OpenSSH PEM format and
// the public key as an authorized_keys line.
func GenerateTestExtensionSSHKey() ([]byte, string) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	Expect(err).ToNot(HaveOccurred(), "Failed to generate the ssh key for the test-extension")
	block, err := ssh.MarshalPrivateKey(privateKey, "capm3-e2e-in-place-upgrade")
	Expect(err).ToNot(HaveOccurred(), "Failed to encode the ssh key for the test-extension")
	signer, err := ssh.NewSignerFromKey(privateKey)
	Expect(err).ToNot(HaveOccurred(), "Failed to get the public key for the test-extension")
	publicKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	return pem.EncodeToMemory(block), publicKey
}

// CreateTestExtensionSSHKeySecret creates the namespace of the in-place update
// runtime extension and the ssh-key secret its pod mounts.
func CreateTestExtensionSSHKeySecret(ctx context.Context, clusterProxy framework.ClusterProxy, privateKey []byte) {
	c := clusterProxy.GetClient()

	By("Ensure the test-extension namespace exists")
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testExtensionNamespace}}
	err := c.Create(ctx, ns)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		Expect(err).ToNot(HaveOccurred(), "Failed to create namespace %s", testExtensionNamespace)
	}

	By("Create the ssh-key secret the test-extension uses to reach the nodes")
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
