package e2e

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bmo "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
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

const bmoPath = "BMOPATH"

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
			Name: os.Getenv("IRONIC_NAMESPACE"),
		},
	}
	_, err := targetClusterClientSet.CoreV1().Namespaces().Create(ctx, ironicNamespace, metav1.CreateOptions{})
	Expect(err).To(BeNil(), "Unable to create the Ironic namespace")

	By("Initialize Provider component in target cluster")
	clusterctl.Init(ctx, clusterctl.InitInput{
		KubeconfigPath:          targetCluster.GetKubeconfigPath(),
		ClusterctlConfigPath:    os.Getenv("CONFIG_FILE_PATH"),
		CoreProvider:            config.ClusterAPIProviderName + ":" + os.Getenv("CAPIRELEASE"),
		BootstrapProviders:      []string{config.KubeadmBootstrapProviderName + ":" + os.Getenv("CAPIRELEASE")},
		ControlPlaneProviders:   []string{config.KubeadmControlPlaneProviderName + ":" + os.Getenv("CAPIRELEASE")},
		InfrastructureProviders: []string{config.Metal3ProviderName + ":" + os.Getenv("CAPM3RELEASE")},
		LogFolder:               filepath.Join(artifactFolder, "clusters", clusterName+"-pivoting"),
	})

	LogFromFile(filepath.Join(artifactFolder, "clusters", clusterName+"-pivoting", "clusterctl-init.log"))

	By("Configure Ironic Configmap")
	configureIronicConfigmap(true)

	By("Add labels to BMO CRDs")
	labelBMOCRDs(nil)

	By("Install BMO")
	installIronicBMO(targetCluster, "false", "true")

	By("Install Ironic in the target cluster")
	installIronicBMO(targetCluster, "true", "false")

	By("Restore original BMO configmap")
	restoreBMOConfigmap()
	By("Reinstate Ironic Configmap")
	configureIronicConfigmap(false)

	By("Add labels to BMO CRDs in the target cluster")
	labelBMOCRDs(targetCluster)

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
	Eventually(func() error {
		bmhList := &bmo.BareMetalHostList{}
		if err := targetCluster.GetClient().List(ctx, bmhList, client.InNamespace(namespace)); err != nil {
			return err
		}
		for _, bmh := range bmhList.Items {
			if !bmh.WasProvisioned() {
				return errors.New("BMHs cannot be provisioned")
			}
		}
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-object-provisioned")...).Should(BeNil())

	By("Check if metal3machines become ready.")
	Eventually(func() error {
		m3Machines := &capm3.Metal3MachineList{}
		if err := targetCluster.GetClient().List(ctx, m3Machines, client.InNamespace(namespace)); err != nil {
			return err
		}
		for _, m3Machine := range m3Machines.Items {
			if !m3Machine.Status.Ready {
				return errors.New("Metal3Machines cannot be ready")
			}
		}
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-object-provisioned")...).Should(BeNil())

	By("Check if machines become running.")
	Eventually(func() error {
		machines := &clusterv1.MachineList{}
		if err := targetCluster.GetClient().List(ctx, machines, client.InNamespace(namespace)); err != nil {
			return err
		}
		for _, machine := range machines.Items {
			if !strings.EqualFold(machine.Status.Phase, "running") { // Case insensitive comparison
				return errors.New("Machines cannot be in the Running state")
			}
		}
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-machine-running")...).Should(BeNil())

	By("PIVOTING TESTS PASSED!")
}

func configureIronicConfigmap(isIronicDeployed bool) {
	ironicDataDir := "IRONIC_DATA_DIR"
	ironicConfigmap := fmt.Sprintf("%s/ironic-deployment/keepalived/ironic_bmo_configmap.env", os.Getenv(bmoPath))
	newIronicConfigmap := fmt.Sprintf("%s/ironic_bmo_configmap.env", os.Getenv(ironicDataDir))
	backupIronicConfigmap := fmt.Sprintf("%s/ironic-deployment/keepalived/ironic_bmo_configmap.env.orig", os.Getenv(bmoPath))
	if isIronicDeployed {
		cmd := exec.Command("cp", ironicConfigmap, backupIronicConfigmap)
		err := cmd.Run()
		Expect(err).To(BeNil(), "Cannot run cp command")
		cmd = exec.Command("cp", newIronicConfigmap, ironicConfigmap)
		err = cmd.Run()
		Expect(err).To(BeNil(), "Cannot run cp command")
	} else {
		cmd := exec.Command("mv", backupIronicConfigmap, ironicConfigmap)
		err := cmd.Run()
		Expect(err).To(BeNil(), "Cannot run mv command")
	}
}

func restoreBMOConfigmap() {
	bmoConfigmap := fmt.Sprintf("%s/config/default/ironic.env", os.Getenv(bmoPath))
	backupBmoConfigmap := fmt.Sprintf("%s/config/default/ironic.env.orig", os.Getenv(bmoPath))
	cmd := exec.Command("mv", backupBmoConfigmap, bmoConfigmap)
	_ = cmd.Run()
	// Accept the error of this cmd because there might be no backup file to restore
}

func installIronicBMO(targetCluster framework.ClusterProxy, isIronic, isBMO string) {
	ironicTLSSetup := "IRONIC_TLS_SETUP"
	ironicBasicAuth := "IRONIC_BASIC_AUTH"
	ironicHost := os.Getenv("CLUSTER_PROVISIONING_IP")
	path := fmt.Sprintf("%s/tools/", os.Getenv(bmoPath))
	args := []string{
		isBMO,
		isIronic,
		os.Getenv(ironicTLSSetup),
		os.Getenv(ironicBasicAuth),
		"true",
	}
	env := []string{
		fmt.Sprintf("IRONIC_HOST=%s", ironicHost),
		fmt.Sprintf("IRONIC_HOST_IP=%s", ironicHost),
		fmt.Sprintf("KUBECTL_ARGS=--kubeconfig=%s", targetCluster.GetKubeconfigPath()),
		"USER=ubuntu",
	}
	cmd := exec.Command("./deploy.sh", args...)
	cmd.Dir = path
	cmd.Env = append(env, os.Environ()...)

	outputPipe, er := cmd.StdoutPipe()
	Expect(er).To(BeNil(), "Cannot get the stdout from the command")
	errorPipe, er := cmd.StderrPipe()
	Expect(er).To(BeNil(), "Cannot get the stderr from the command")
	err := cmd.Start()
	Expect(err).To(BeNil(), "Failed to deploy Ironic")
	data, er := io.ReadAll(outputPipe)
	Expect(er).To(BeNil(), "Cannot get the stdout from the command")
	if len(data) > 0 {
		Logf("Output of the shell: %s\n", string(data))
	}
	errorData, er := io.ReadAll(errorPipe)
	Expect(er).To(BeNil(), "Cannot get the stderr from the command")
	err = cmd.Wait()
	if len(errorData) > 0 {
		Logf("Error of the shell: %v\n", string(errorData))
	}
	Expect(err).To(BeNil(), "Failed to deploy Ironic")
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
	deploymentName := os.Getenv("NAMEPREFIX") + "-ironic"
	ironicNamespace := os.Getenv("IRONIC_NAMESPACE")
	err := bootstrapClusterProxy.GetClientSet().AppsV1().Deployments(ironicNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	Expect(err).To(BeNil(), "Failed to delete Ironic from the source cluster")
}

func removeIronicDeploymentOnTarget() {
	deploymentName := os.Getenv("NAMEPREFIX") + "-ironic"
	ironicNamespace := os.Getenv("IRONIC_NAMESPACE")
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

func rePivoting() {
	Logf("Start the re-pivoting test")
	By("Remove Ironic deployment from target cluster")
	removeIronicDeploymentOnTarget()

	By("Reinstate Ironic containers and BMH")
	ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
	if ephemeralCluster == KIND {
		bmoPath := os.Getenv("BMOPATH")
		ironicCommand := bmoPath + "/tools/run_local_ironic.sh"
		cmd := exec.Command("sh", "-c", "export CONTAINER_RUNTIME=docker; "+ironicCommand)
		stdoutStderr, err := cmd.CombinedOutput()
		fmt.Printf("%s\n", stdoutStderr)
		Expect(err).To(BeNil(), "Cannot run local ironic")
	} else {
		By("Configure Ironic Configmap")
		configureIronicConfigmap(true)

		By("Install Ironic in the target cluster")
		installIronicBMO(bootstrapClusterProxy, "true", "false")

		By("Reinstate Ironic Configmap")
		configureIronicConfigmap(false)
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
		bmhList := &bmo.BareMetalHostList{}
		g.Expect(bootstrapClusterProxy.GetClient().List(ctx, bmhList, client.InNamespace(namespace))).To(Succeed())
		for _, bmh := range bmhList.Items {
			g.Expect(bmh.WasProvisioned()).To(BeTrue())
			g.Expect(bmh.Spec.Online).To(BeTrue())
		}
	}, e2eConfig.GetIntervals(specName, "wait-object-provisioned")...).Should(Succeed())

	By("Check if metal3machines become ready.")
	Eventually(func() error {
		m3Machines := &capm3.Metal3MachineList{}
		if err := bootstrapClusterProxy.GetClient().List(ctx, m3Machines, client.InNamespace(namespace)); err != nil {
			return err
		}
		for _, m3Machine := range m3Machines.Items {
			if !m3Machine.Status.Ready {
				return errors.New("Metal3Machines cannot be ready")
			}
		}
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-object-provisioned")...).Should(Succeed())

	By("Check if machines become running.")
	Eventually(func() error {
		machines := &clusterv1.MachineList{}
		if err := bootstrapClusterProxy.GetClient().List(ctx, machines, client.InNamespace(namespace)); err != nil {
			return err
		}
		for _, machine := range machines.Items {
			if !strings.EqualFold(machine.Status.Phase, "running") { // Case insensitive comparison
				return errors.New("Machines cannot be in the Running state")
			}
		}
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-machine-running")...).Should(Succeed())

	By("RE-PIVOTING TEST PASSED!")
}
