package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	dockerTypes "github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	framework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const tempKustomizations = "TEMP_KUSTOMIZATIONS"

func pivoting() {
	Logf("Starting pivoting tests")
	By("Remove Ironic containers from the source cluster")
	ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
	if ephemeralCluster == KIND {
		removeIronicContainers()
	} else {
		removeIronicDeployment()
	}

	By("Create Ironic namespace")
	targetClusterClientSet := targetCluster.GetClientSet()
	ironicNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: e2eConfig.GetVariable("IRONIC_NAMESPACE"),
		},
	}
	_, err := targetClusterClientSet.CoreV1().Namespaces().Create(ctx, ironicNamespace, metav1.CreateOptions{})
	Expect(err).To(BeNil(), "Unable to create the Ironic namespace")

	By("Initialize Provider component in target cluster")
	clusterctl.Init(ctx, clusterctl.InitInput{
		KubeconfigPath:          targetCluster.GetKubeconfigPath(),
		ClusterctlConfigPath:    e2eConfig.GetVariable("CONFIG_FILE_PATH"),
		CoreProvider:            config.ClusterAPIProviderName + ":" + os.Getenv("CAPIRELEASE"),
		BootstrapProviders:      []string{config.KubeadmBootstrapProviderName + ":" + os.Getenv("CAPIRELEASE")},
		ControlPlaneProviders:   []string{config.KubeadmControlPlaneProviderName + ":" + os.Getenv("CAPIRELEASE")},
		InfrastructureProviders: []string{config.Metal3ProviderName + ":" + os.Getenv("CAPM3RELEASE")},
		LogFolder:               filepath.Join(artifactFolder, "clusters", clusterName+"-pivoting"),
	})

	LogFromFile(filepath.Join(artifactFolder, "clusters", clusterName+"-pivoting", "clusterctl-init.log"))

	By("Add labels to BMO CRDs")
	labelBMOCRDs(nil)
	By("Add Labels to hardwareData CRDs")
	labelHDCRDs(nil)

	By("Install BMO")
	installBareMetalOperator(targetCluster)

	By("Install Ironic in the target cluster")
	installIronic(targetCluster)

	By("Add labels to BMO CRDs in the target cluster")
	labelBMOCRDs(targetCluster)
	By("Add Labels to hardwareData CRDs in the target cluster")
	labelHDCRDs(targetCluster)

	By("Ensure API servers are stable before doing move")
	// Nb. This check was introduced to prevent doing move to self-hosted in an aggressive way and thus avoid flakes.
	// More specifically, we were observing the test failing to get objects from the API server during move, so we
	// are now testing the API servers are stable before starting move.
	Consistently(func() error {
		kubeSystem := &corev1.Namespace{}
		return bootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
	}, "5s", "100ms").Should(BeNil(), "Failed to assert bootstrap API server stability")
	Consistently(func() error {
		kubeSystem := &corev1.Namespace{}
		return targetCluster.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
	}, "5s", "100ms").Should(BeNil(), "Failed to assert target API server stability")

	By("Moving the cluster to self hosted")
	clusterctl.Move(ctx, clusterctl.MoveInput{
		LogFolder:            filepath.Join(artifactFolder, "clusters", clusterName+"-bootstrap"),
		ClusterctlConfigPath: clusterctlConfigPath,
		FromKubeconfigPath:   bootstrapClusterProxy.GetKubeconfigPath(),
		ToKubeconfigPath:     targetCluster.GetKubeconfigPath(),
		Namespace:            namespace,
	})
	LogFromFile(filepath.Join(artifactFolder, "clusters", clusterName+"-bootstrap", "clusterctl-move.log"))

	pivotingCluster := framework.DiscoveryAndWaitForCluster(ctx, framework.DiscoveryAndWaitForClusterInput{
		Getter:    targetCluster.GetClient(),
		Namespace: namespace,
		Name:      clusterName,
	}, e2eConfig.GetIntervals(specName, "wait-cluster")...)

	controlPlane := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      targetCluster.GetClient(),
		ClusterName: pivotingCluster.Name,
		Namespace:   pivotingCluster.Namespace,
	})
	Expect(controlPlane).ToNot(BeNil())

	By("Check if BMH is in provisioned state")
	Eventually(func(g Gomega) {
		bmhList := &bmov1alpha1.BareMetalHostList{}
		g.Expect(targetCluster.GetClient().List(ctx, bmhList, client.InNamespace(namespace))).To(Succeed())
		for _, bmh := range bmhList.Items {
			g.Expect(bmh.WasProvisioned()).To(BeTrue())
		}
	}, e2eConfig.GetIntervals(specName, "wait-object-provisioned")...).Should(Succeed())

	By("Check if metal3machines become ready.")
	Eventually(func(g Gomega) {
		m3Machines := &infrav1.Metal3MachineList{}
		g.Expect(targetCluster.GetClient().List(ctx, m3Machines, client.InNamespace(namespace)))
		for _, m3Machine := range m3Machines.Items {
			g.Expect(m3Machine.Status.Ready).To(BeTrue())
		}
	}, e2eConfig.GetIntervals(specName, "wait-object-provisioned")...).Should(Succeed())

	By("Check if machines become running.")
	Eventually(func(g Gomega) {
		machines := &clusterv1.MachineList{}
		g.Expect(targetCluster.GetClient().List(ctx, machines, client.InNamespace(namespace))).To(Succeed())
		for _, machine := range machines.Items {
			g.Expect(strings.EqualFold(machine.Status.Phase, "running")).To(BeTrue())
		}
	}, e2eConfig.GetIntervals(specName, "wait-machine-running")...).Should(Succeed())

	By("PIVOTING TESTS PASSED!")
}

func installIronic(clusterProxy framework.ClusterProxy) {
	path := fmt.Sprintf("%s/ironic.yaml", e2eConfig.GetVariable(tempKustomizations))
	yaml, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	Expect(clusterProxy.Apply(ctx, yaml)).To(Succeed())
}

func installBareMetalOperator(clusterProxy framework.ClusterProxy) {
	path := fmt.Sprintf("%s/bmo.yaml", e2eConfig.GetVariable(tempKustomizations))
	yaml, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	Expect(clusterProxy.Apply(ctx, yaml)).To(Succeed())
}

func removeIronicContainers() {
	ironicContainerList := []string{
		"ironic",
		"ironic-inspector",
		"dnsmasq",
		"ironic-endpoint-keepalived",
		"ironic-log-watch",
	}
	dockerClient, err := docker.NewClientWithOpts()
	Expect(err).To(BeNil(), "Unable to get docker client")
	removeOptions := dockerTypes.ContainerRemoveOptions{}
	for _, container := range ironicContainerList {
		d := 1 * time.Minute
		err = dockerClient.ContainerStop(ctx, container, &d)
		Expect(err).To(BeNil(), "Unable to stop the container %s: %v", container, err)
		err = dockerClient.ContainerRemove(ctx, container, removeOptions)
		Expect(err).To(BeNil(), "Unable to delete the container %s: %v", container, err)
	}
}

func removeIronicDeployment() {
	deploymentName := "ironic"
	ironicNamespace := e2eConfig.GetVariable("IRONIC_NAMESPACE")
	err := bootstrapClusterProxy.GetClientSet().AppsV1().Deployments(ironicNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	Expect(err).To(BeNil(), "Failed to delete Ironic from the source cluster")
}

func removeIronicDeploymentOnTarget() {
	deploymentName := "ironic"
	ironicNamespace := e2eConfig.GetVariable("IRONIC_NAMESPACE")
	err := targetCluster.GetClientSet().AppsV1().Deployments(ironicNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	Expect(err).To(BeNil(), "Failed to delete Ironic from the target cluster")
}

func labelBMOCRDs(targetCluster framework.ClusterProxy) {
	labels := []string{
		"clusterctl.cluster.x-k8s.io=",
		"cluster.x-k8s.io/provider=metal3",
	}
	kubectlArgs := ""
	if targetCluster != nil {
		kubectlArgs = fmt.Sprintf("--kubeconfig=%s", targetCluster.GetKubeconfigPath())
	}

	crdName := "baremetalhosts.metal3.io"
	for _, label := range labels {
		var cmd *exec.Cmd
		if kubectlArgs == "" {
			cmd = exec.Command("kubectl", "label", "--overwrite", "crds", crdName, label)
		} else {
			cmd = exec.Command("kubectl", kubectlArgs, "label", "--overwrite", "crds", crdName, label)
		}
		err := cmd.Run()
		Expect(err).To(BeNil(), "Cannot label BMO CRDs")
	}
}

func labelHDCRDs(targetCluster framework.ClusterProxy) {
	labels := []string{
		"clusterctl.cluster.x-k8s.io=",
		"clusterctl.cluster.x-k8s.io/move=",
	}
	kubectlArgs := ""
	if targetCluster != nil {
		kubectlArgs = fmt.Sprintf("--kubeconfig=%s", targetCluster.GetKubeconfigPath())
	}

	crdName := "hardwaredata.metal3.io"
	for _, label := range labels {
		var cmd *exec.Cmd
		if kubectlArgs == "" {
			cmd = exec.Command("kubectl", "label", "--overwrite", "crds", crdName, label)
		} else {
			cmd = exec.Command("kubectl", kubectlArgs, "label", "--overwrite", "crds", crdName, label)
		}
		err := cmd.Run()
		Expect(err).To(BeNil(), "Cannot label HD CRDs")
	}
}

func rePivoting() {
	Logf("Start the re-pivoting test")
	By("Remove Ironic deployment from target cluster")
	removeIronicDeploymentOnTarget()

	By("Reinstate Ironic containers and BMH")
	ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
	if ephemeralCluster == KIND {
		bmoPath := e2eConfig.GetVariable("BMOPATH")
		ironicCommand := bmoPath + "/tools/run_local_ironic.sh"
		cmd := exec.Command("sh", "-c", "export CONTAINER_RUNTIME=docker; "+ironicCommand)
		stdoutStderr, err := cmd.CombinedOutput()
		fmt.Printf("%s\n", stdoutStderr)
		Expect(err).To(BeNil(), "Cannot run local ironic")
	} else {
		By("Install Ironic in the target cluster")
		installIronic(bootstrapClusterProxy)
	}

	By("Ensure API servers are stable before doing move")
	// Nb. This check was introduced to prevent doing move to self-hosted in an aggressive way and thus avoid flakes.
	// More specifically, it was observed that the test was failing to get objects from the API server during move, so now
	// it is tested whether the API servers are stable before starting move.
	Consistently(func() error {
		kubeSystem := &corev1.Namespace{}
		return bootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
	}, "5s", "100ms").Should(BeNil(), "Failed to assert bootstrap API server stability")

	By("Move back to bootstrap cluster")
	clusterctl.Move(ctx, clusterctl.MoveInput{
		LogFolder:            filepath.Join(artifactFolder, "clusters", clusterName+"-pivot"),
		ClusterctlConfigPath: clusterctlConfigPath,
		FromKubeconfigPath:   targetCluster.GetKubeconfigPath(),
		ToKubeconfigPath:     bootstrapClusterProxy.GetKubeconfigPath(),
		Namespace:            namespace,
	})

	LogFromFile(filepath.Join(artifactFolder, "clusters", clusterName+"-pivot", "clusterctl-move.log"))

	By("Check that the re-pivoted cluster is up and running")
	pivotingCluster := framework.DiscoveryAndWaitForCluster(ctx, framework.DiscoveryAndWaitForClusterInput{
		Getter:    bootstrapClusterProxy.GetClient(),
		Namespace: namespace,
		Name:      clusterName,
	}, e2eConfig.GetIntervals(specName, "wait-cluster")...)

	By("Check that the control plane of the re-pivoted cluster is up and running")
	controlPlane := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      bootstrapClusterProxy.GetClient(),
		ClusterName: pivotingCluster.Name,
		Namespace:   pivotingCluster.Namespace,
	})
	Expect(controlPlane).ToNot(BeNil())

	By("Check if BMH is in provisioned state")
	Eventually(func(g Gomega) {
		bmhList := &bmov1alpha1.BareMetalHostList{}
		g.Expect(bootstrapClusterProxy.GetClient().List(ctx, bmhList, client.InNamespace(namespace))).To(Succeed())
		for _, bmh := range bmhList.Items {
			g.Expect(bmh.WasProvisioned()).To(BeTrue())
			g.Expect(bmh.Spec.Online).To(BeTrue())
		}
	}, e2eConfig.GetIntervals(specName, "wait-object-provisioned")...).Should(Succeed())

	By("Check if metal3machines become ready.")
	Eventually(func(g Gomega) {
		m3Machines := &infrav1.Metal3MachineList{}
		g.Expect(bootstrapClusterProxy.GetClient().List(ctx, m3Machines, client.InNamespace(namespace))).Should(Succeed())
		for _, m3Machine := range m3Machines.Items {
			g.Expect(m3Machine.Status.Ready).To(BeTrue())
		}
	}, e2eConfig.GetIntervals(specName, "wait-object-provisioned")...).Should(Succeed())

	By("Check if machines become running.")
	Eventually(func(g Gomega) {
		machines := &clusterv1.MachineList{}
		g.Expect(bootstrapClusterProxy.GetClient().List(ctx, machines, client.InNamespace(namespace))).Should(Succeed())
		for _, machine := range machines.Items {
			g.Expect(strings.EqualFold(machine.Status.Phase, "running")).To(BeTrue())
		}
	}, e2eConfig.GetIntervals(specName, "wait-machine-running")...).Should(Succeed())

	By("RE-PIVOTING TEST PASSED!")
}
