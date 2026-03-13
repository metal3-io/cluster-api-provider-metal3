package e2e

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	containerTypes "github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	irsov1alpha1 "github.com/metal3-io/ironic-standalone-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	framework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	bmoPath                      = "BMOPATH"
	ironicTLSSetup               = "IRONIC_TLS_SETUP"
	ironicBasicAuth              = "IRONIC_BASIC_AUTH"
	ironicKeepalived             = "IRONIC_KEEPALIVED"
	ironicMariadb                = "IRONIC_USE_MARIADB"
	Kind                         = "kind"
	NamePrefix                   = "NAMEPREFIX"
	restartContainerCertUpdate   = "RESTART_CONTAINER_CERTIFICATE_UPDATED"
	ironicNamespace              = "IRONIC_NAMESPACE"
	clusterLogCollectionBasePath = "/tmp/target_cluster_logs"
	Metal3ipamProviderName       = "metal3"
)

type PivotingInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
	ArtifactFolder        string
	ClusterctlConfigPath  string
}

// Pivoting implements a test that verifies successful moving of management resources (CRs, BMO, Ironic) to a target cluster after initializing it with Provider components.
func pivoting(ctx context.Context, inputGetter func() PivotingInput) {
	Logf("Starting pivoting tests")
	input := inputGetter()
	numberOfWorkers := int(*input.E2EConfig.MustGetInt32PtrVariable("WORKER_MACHINE_COUNT"))
	numberOfControlplane := int(*input.E2EConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
	numberOfAllBmh := numberOfWorkers + numberOfControlplane

	ListBareMetalHosts(ctx, input.BootstrapClusterProxy.GetClient(), client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, input.BootstrapClusterProxy.GetClient(), client.InNamespace(input.Namespace))
	ListMachines(ctx, input.BootstrapClusterProxy.GetClient(), client.InNamespace(input.Namespace))
	ListNodes(ctx, input.TargetCluster.GetClient())

	By("Fetch logs from target cluster before pivot")
	err := FetchClusterLogs(input.TargetCluster, filepath.Join(clusterLogCollectionBasePath, "beforePivot"))
	if err != nil {
		Logf("Error: %v", err)
	}

	ironicContainers := []string{
		"ironic",
		"ironic-endpoint-keepalived",
		"ironic-log-watch",
		"dnsmasq",
	}
	generalContainers := []string{
		"httpd-infra",
		"registry",
		"sushy-tools",
		"vbmc",
	}

	By("Fetch container logs")
	bootstrapCluster := os.Getenv("BOOTSTRAP_CLUSTER")
	fetchContainerLogs(&generalContainers, input.ArtifactFolder, input.E2EConfig.MustGetVariable("CONTAINER_RUNTIME"))
	if bootstrapCluster == Kind {
		fetchContainerLogs(&ironicContainers, input.ArtifactFolder, input.E2EConfig.MustGetVariable("CONTAINER_RUNTIME"))
	}

	By("Fetch manifest for bootstrap cluster before pivot")
	err = FetchManifests(input.BootstrapClusterProxy, "/tmp/manifests/")
	if err != nil {
		Logf("Error fetching manifests for bootstrap cluster before pivot: %v", err)
	}
	By("Fetch target cluster kubeconfig for target cluster log collection")
	kconfigPathWorkload := input.TargetCluster.GetKubeconfigPath()
	os.Setenv("KUBECONFIG_WORKLOAD", kconfigPathWorkload)
	Logf("Save kubeconfig in temp folder for project-infra target log collection")
	// TODO(smoshiur1237): This is a workaround to copy the target kubeconfig and enable project-infra
	// target log collection. There is possibility to handle the kubeconfig in better way.
	// KubeconfigPathTemp will be used by project-infra target log collection only incase of failed e2e test
	kubeconfigPathTemp := "/tmp/kubeconfig-test1.yaml"
	cmd := exec.CommandContext(ctx, "cp", kconfigPathWorkload, kubeconfigPathTemp) // #nosec G204:gosec
	stdoutStderr, er := cmd.CombinedOutput()
	Logf("%s\n", stdoutStderr)
	Expect(er).ToNot(HaveOccurred(), "Cannot fetch target cluster kubeconfig")

	By("Remove Ironic containers from the source cluster")
	ironicDeploymentType := IronicDeploymentTypeBMO
	if bootstrapCluster == Kind {
		ironicDeploymentType = IronicDeploymentTypeLocal
	} else if GetBoolVariable(input.E2EConfig, "USE_IRSO") {
		ironicDeploymentType = IronicDeploymentTypeIrSO
	}
	removeIronic(ctx, func() RemoveIronicInput {
		return RemoveIronicInput{
			ManagementCluster: input.BootstrapClusterProxy,
			DeploymentType:    ironicDeploymentType,
			Namespace:         input.E2EConfig.MustGetVariable(ironicNamespace),
			NamePrefix:        input.E2EConfig.MustGetVariable(NamePrefix),
		}
	})

	By("Create Ironic namespace")
	targetClusterClientSet := input.TargetCluster.GetClientSet()
	ironicNamespaceObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: input.E2EConfig.MustGetVariable(ironicNamespace),
		},
	}
	_, err = targetClusterClientSet.CoreV1().Namespaces().Create(ctx, ironicNamespaceObj, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred(), "Unable to create the Ironic namespace")

	By("Initialize Provider component in target cluster")
	clusterctl.Init(ctx, clusterctl.InitInput{
		KubeconfigPath:          input.TargetCluster.GetKubeconfigPath(),
		ClusterctlConfigPath:    input.E2EConfig.MustGetVariable("CONFIG_FILE_PATH"),
		CoreProvider:            config.ClusterAPIProviderName + ":" + os.Getenv("CAPIRELEASE"),
		BootstrapProviders:      []string{config.KubeadmBootstrapProviderName + ":" + os.Getenv("CAPIRELEASE")},
		ControlPlaneProviders:   []string{config.KubeadmControlPlaneProviderName + ":" + os.Getenv("CAPIRELEASE")},
		InfrastructureProviders: []string{config.Metal3ProviderName + ":" + os.Getenv("CAPM3RELEASE")},
		IPAMProviders:           []string{Metal3ipamProviderName + ":" + os.Getenv("IPAMRELEASE")},
		LogFolder:               filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName+"-pivoting"),
	})

	LogFromFile(filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName+"-pivoting", "clusterctl-init.log"))

	By("Add labels to BMO CRDs")
	labelBMOCRDs(ctx, input.BootstrapClusterProxy)
	By("Add Labels to hardwareData CRDs")
	labelHDCRDs(ctx, input.BootstrapClusterProxy)

	By("Install Ironic in the target cluster")
	// TODO(dtantsur): support ironic-standalone-operator
	ironicDeployLogFolder := filepath.Join(os.TempDir(), "target_cluster_logs", "ironic-deploy-logs", input.TargetCluster.GetName())
	ironicKustomization := input.E2EConfig.MustGetVariable("IRONIC_RELEASE_PR_TEST")
	By(fmt.Sprintf("Installing Ironic from kustomization %s on the target cluster", ironicKustomization))
	err = BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       ironicKustomization,
		ClusterProxy:        input.TargetCluster,
		WaitForDeployment:   true,
		WatchDeploymentLogs: true,
		LogPath:             ironicDeployLogFolder,
		DeploymentName:      "baremetal-operator-ironic",
		DeploymentNamespace: ironicNamespaceObj.Name,
		WaitIntervals:       input.E2EConfig.GetIntervals("default", "wait-deployment"),
	})
	Expect(err).NotTo(HaveOccurred())

	By("Install BMO in the target cluster")
	bmoDeployLogFolder := filepath.Join(os.TempDir(), "target_cluster_logs", "bmo-deploy-logs", input.TargetCluster.GetName())
	bmoKustomization := input.E2EConfig.MustGetVariable("BMO_RELEASE_PR_TEST")
	By(fmt.Sprintf("Installing BMO from kustomization %s on the target cluster", bmoKustomization))
	err = BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       bmoKustomization,
		ClusterProxy:        input.TargetCluster,
		WaitForDeployment:   true,
		WatchDeploymentLogs: true,
		LogPath:             bmoDeployLogFolder,
		DeploymentName:      "baremetal-operator-controller-manager",
		DeploymentNamespace: ironicNamespaceObj.Name,
		WaitIntervals:       input.E2EConfig.GetIntervals("default", "wait-deployment"),
	})
	Expect(err).NotTo(HaveOccurred())

	By("Add labels to BMO CRDs in the target cluster")
	labelBMOCRDs(ctx, input.TargetCluster)
	By("Add Labels to hardwareData CRDs in the target cluster")
	labelHDCRDs(ctx, input.TargetCluster)
	By("Ensure API servers are stable before doing move")
	// Nb. This check was introduced to prevent doing move to self-hosted in an aggressive way and thus avoid flakes.
	// More specifically, we were observing the test failing to get objects from the API server during move, so we
	// are now testing the API servers are stable before starting move.
	Consistently(func() error {
		kubeSystem := &corev1.Namespace{}
		return input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
	}, "5s", "100ms").Should(Succeed(), "Failed to assert bootstrap API server stability")
	Consistently(func() error {
		kubeSystem := &corev1.Namespace{}
		return input.TargetCluster.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
	}, "5s", "100ms").Should(Succeed(), "Failed to assert target API server stability")

	By("Moving the cluster to self hosted")
	clusterctl.Move(ctx, clusterctl.MoveInput{
		LogFolder:            filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName+"-bootstrap"),
		ClusterctlConfigPath: input.ClusterctlConfigPath,
		FromKubeconfigPath:   input.BootstrapClusterProxy.GetKubeconfigPath(),
		ToKubeconfigPath:     input.TargetCluster.GetKubeconfigPath(),
		Namespace:            input.Namespace,
	})
	LogFromFile(filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName+"-bootstrap", "logs", input.Namespace, "clusterctl-move.log"))

	By("Remove BMO deployment from the source cluster")
	RemoveDeployment(ctx, func() RemoveDeploymentInput {
		return RemoveDeploymentInput{
			ManagementCluster: input.BootstrapClusterProxy,
			Namespace:         input.E2EConfig.MustGetVariable(ironicNamespace),
			Name:              input.E2EConfig.MustGetVariable(NamePrefix) + "-controller-manager",
		}
	})
	pivotingCluster := framework.DiscoveryAndWaitForCluster(ctx, framework.DiscoveryAndWaitForClusterInput{
		Getter:    input.TargetCluster.GetClient(),
		Namespace: input.Namespace,
		Name:      input.ClusterName,
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-cluster")...)

	controlPlane := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      input.TargetCluster.GetClient(),
		ClusterName: pivotingCluster.Name,
		Namespace:   pivotingCluster.Namespace,
	})
	Expect(controlPlane).ToNot(BeNil())

	By("Check that BMHs are in provisioned state")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    input.TargetCluster.GetClient(),
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-object-provisioned"),
	})

	By("Check if metal3machines become ready.")
	WaitForNumMetal3MachinesReady(ctx, WaitForNumInput{
		Client:    input.TargetCluster.GetClient(),
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-object-provisioned"),
	})

	By("Check that all machines become running.")
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    input.TargetCluster.GetClient(),
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	By("PIVOTING TESTS PASSED!")
}

type IronicDeploymentType string

const (
	IronicDeploymentTypeLocal IronicDeploymentType = "local"
	IronicDeploymentTypeBMO   IronicDeploymentType = "deploy.sh"
	IronicDeploymentTypeIrSO  IronicDeploymentType = "irso"
)

type RemoveIronicInput struct {
	ManagementCluster framework.ClusterProxy
	DeploymentType    IronicDeploymentType
	Namespace         string
	NamePrefix        string
}

func removeIronic(ctx context.Context, inputGetter func() RemoveIronicInput) {
	input := inputGetter()
	if input.DeploymentType == IronicDeploymentTypeBMO {
		deploymentName := input.NamePrefix + ironicSuffix
		RemoveDeployment(ctx, func() RemoveDeploymentInput {
			return RemoveDeploymentInput{
				ManagementCluster: input.ManagementCluster,
				Namespace:         input.Namespace,
				Name:              deploymentName,
			}
		})
	} else if input.DeploymentType == IronicDeploymentTypeIrSO {
		// NOTE(dtantsur): metal3-dev-env hardcodes the name "ironic".
		ironicObj := &irsov1alpha1.Ironic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ironic",
				Namespace: input.Namespace,
			},
		}
		err := input.ManagementCluster.GetClient().Delete(ctx, ironicObj)
		Expect(err).ToNot(HaveOccurred(), "Failed to delete Ironic")
	} else {
		ironicContainerList := []string{
			"ironic",
			"dnsmasq",
			"ironic-endpoint-keepalived",
			"ironic-log-watch",
		}
		dockerClient, err := docker.NewClientWithOpts()
		Expect(err).ToNot(HaveOccurred(), "Unable to get docker client")
		removeOptions := containerTypes.RemoveOptions{}
		stopTimeout := 60
		for _, container := range ironicContainerList {
			err = dockerClient.ContainerStop(ctx, container, containerTypes.StopOptions{Timeout: &stopTimeout})
			Expect(err).ToNot(HaveOccurred(), "Unable to stop the container %s: %v", container, err)
			err = dockerClient.ContainerRemove(ctx, container, removeOptions)
			Expect(err).ToNot(HaveOccurred(), "Unable to delete the container %s: %v", container, err)
		}
	}
}

type RemoveDeploymentInput struct {
	ManagementCluster framework.ClusterProxy
	Namespace         string
	Name              string
}

func RemoveDeployment(ctx context.Context, inputGetter func() RemoveDeploymentInput) {
	input := inputGetter()

	deploymentName := input.Name
	ironicNamespace := input.Namespace
	err := input.ManagementCluster.GetClientSet().AppsV1().Deployments(ironicNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	Expect(err).ToNot(HaveOccurred(), "Failed to delete %s Deployment", deploymentName)
}

func labelBMOCRDs(ctx context.Context, targetCluster framework.ClusterProxy) {
	labels := map[string]string{}
	labels[clusterctlv1.ClusterctlLabel] = ""
	labels[clusterv1.ProviderNameLabel] = "metal3"
	crdName := "baremetalhosts.metal3.io"
	err := LabelCRD(ctx, targetCluster.GetClient(), crdName, labels)
	Expect(err).ToNot(HaveOccurred(), "Cannot label BMH CRDs")
}

func labelHDCRDs(ctx context.Context, targetCluster framework.ClusterProxy) {
	labels := map[string]string{}
	labels[clusterctlv1.ClusterctlLabel] = ""
	labels[clusterctlv1.ClusterctlMoveLabel] = ""
	crdName := "hardwaredata.metal3.io"
	err := LabelCRD(ctx, targetCluster.GetClient(), crdName, labels)
	Expect(err).ToNot(HaveOccurred(), "Cannot label HD CRDs")
}

type RePivotingInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
	ArtifactFolder        string
	ClusterctlConfigPath  string
}

func rePivoting(ctx context.Context, inputGetter func() RePivotingInput) {
	Logf("Start the re-pivoting test")
	input := inputGetter()
	numberOfWorkers := int(*input.E2EConfig.MustGetInt32PtrVariable("WORKER_MACHINE_COUNT"))
	numberOfControlplane := int(*input.E2EConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
	numberOfAllBmh := numberOfWorkers + numberOfControlplane

	By("Fetch logs from target cluster after pivot")
	err := FetchClusterLogs(input.TargetCluster, filepath.Join(clusterLogCollectionBasePath, "afterPivot"))
	if err != nil {
		Logf("Error: %v", err)
	}

	By("Fetch manifest for workload cluster after pivot")
	workloadClusterProxy := framework.NewClusterProxy("workload-cluster-after-pivot", os.Getenv("KUBECONFIG"), runtime.NewScheme())
	err = FetchManifests(workloadClusterProxy, "/tmp/manifests/")
	if err != nil {
		Logf("Error fetching manifests for workload cluster after pivot: %v", err)
	}
	os.Unsetenv("KUBECONFIG_WORKLOAD")

	By("Remove Ironic deployment from target cluster")
	ironicDeploymentType := IronicDeploymentTypeBMO
	// TODO(dtantsur): support USE_IRSO in the target cluster
	removeIronic(ctx, func() RemoveIronicInput {
		return RemoveIronicInput{
			ManagementCluster: input.TargetCluster,
			DeploymentType:    ironicDeploymentType,
			Namespace:         input.E2EConfig.MustGetVariable(ironicNamespace),
			NamePrefix:        input.E2EConfig.MustGetVariable(NamePrefix),
		}
	})

	By("Reinstate Ironic containers and BMH")
	bootstrapCluster := os.Getenv("BOOTSTRAP_CLUSTER")
	if bootstrapCluster == Kind {
		bmoPath := input.E2EConfig.MustGetVariable("BMOPATH")
		ironicCommand := bmoPath + "/tools/run_local_ironic.sh"
		//#nosec G204:gosec
		cmd := exec.CommandContext(ctx, "sh", "-c", "export CONTAINER_RUNTIME=docker; "+ironicCommand)
		var stdoutStderr []byte
		stdoutStderr, err = cmd.CombinedOutput()
		Logf("Output: %s", stdoutStderr)
		Expect(err).ToNot(HaveOccurred(), "Cannot run local ironic")
	} else {
		By("Install Ironic in the bootstrap cluster")
		ironicKustomization := input.E2EConfig.MustGetVariable("IRONIC_RELEASE_PR_TEST")
		ironicDeployLogFolder := filepath.Join(os.TempDir(), "source_cluster_logs", "ironic-deploy-logs", input.TargetCluster.GetName())
		err = BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
			Kustomization:       ironicKustomization,
			ClusterProxy:        input.BootstrapClusterProxy,
			WaitForDeployment:   true,
			WatchDeploymentLogs: true,
			LogPath:             ironicDeployLogFolder,
			DeploymentName:      "baremetal-operator-ironic",
			DeploymentNamespace: input.E2EConfig.MustGetVariable(ironicNamespace),
			WaitIntervals:       input.E2EConfig.GetIntervals("default", "wait-deployment"),
		})
		Expect(err).NotTo(HaveOccurred())
	}

	By("Reinstate BMO in Source cluster")
	bmoKustomization := input.E2EConfig.MustGetVariable("BMO_RELEASE_PR_TEST")
	bmoDeployLogFolder := filepath.Join(os.TempDir(), "source_cluster_logs", "bmo-deploy-logs", input.TargetCluster.GetName())
	By(fmt.Sprintf("Installing BMO from kustomization %s on the source cluster", bmoKustomization))
	err = BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       bmoKustomization,
		ClusterProxy:        input.BootstrapClusterProxy,
		WaitForDeployment:   true,
		WatchDeploymentLogs: true,
		LogPath:             bmoDeployLogFolder,
		DeploymentName:      "baremetal-operator-controller-manager",
		DeploymentNamespace: input.E2EConfig.MustGetVariable(ironicNamespace),
		WaitIntervals:       input.E2EConfig.GetIntervals("default", "wait-deployment"),
	})
	Expect(err).NotTo(HaveOccurred())

	By("Ensure API servers are stable before doing move")
	// Nb. This check was introduced to prevent doing move to self-hosted in an aggressive way and thus avoid flakes.
	// More specifically, it was observed that the test was failing to get objects from the API server during move, so now
	// it is tested whether the API servers are stable before starting move.
	Consistently(func() error {
		kubeSystem := &corev1.Namespace{}
		return input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
	}, "5s", "100ms").Should(Succeed(), "Failed to assert bootstrap API server stability")

	By("Move back to bootstrap cluster")
	clusterctl.Move(ctx, clusterctl.MoveInput{
		LogFolder:            filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName+"-pivot"),
		ClusterctlConfigPath: input.ClusterctlConfigPath,
		FromKubeconfigPath:   input.TargetCluster.GetKubeconfigPath(),
		ToKubeconfigPath:     input.BootstrapClusterProxy.GetKubeconfigPath(),
		Namespace:            input.Namespace,
	})

	LogFromFile(filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName+"-pivot", "logs", input.Namespace, "clusterctl-move.log"))

	By("Check that the re-pivoted cluster is up and running")
	pivotingCluster := framework.DiscoveryAndWaitForCluster(ctx, framework.DiscoveryAndWaitForClusterInput{
		Getter:    input.BootstrapClusterProxy.GetClient(),
		Namespace: input.Namespace,
		Name:      input.ClusterName,
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-cluster")...)

	By("Check that the control plane of the re-pivoted cluster is up and running")
	controlPlane := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      input.BootstrapClusterProxy.GetClient(),
		ClusterName: pivotingCluster.Name,
		Namespace:   pivotingCluster.Namespace,
	})
	Expect(controlPlane).ToNot(BeNil())

	By("Check that BMHs are in provisioned state")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    input.BootstrapClusterProxy.GetClient(),
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-object-provisioned"),
	})

	By("Check if metal3machines become ready.")
	WaitForNumMetal3MachinesReady(ctx, WaitForNumInput{
		Client:    input.BootstrapClusterProxy.GetClient(),
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-object-provisioned"),
	})

	By("Check that all machines become running.")
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    input.BootstrapClusterProxy.GetClient(),
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	By("Fetch manifest for bootstrap cluster after re-pivot")
	err = FetchManifests(input.BootstrapClusterProxy, "/tmp/manifests/")
	if err != nil {
		Logf("Error fetching manifests for bootstrap cluster before pivot: %v", err)
	}
	os.Unsetenv("KUBECONFIG_BOOTSTRAP")

	By("RE-PIVOTING TEST PASSED!")
}

func createDirIfNotExist(dirPath string) {
	err := os.MkdirAll(dirPath, 0750)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}
}

// fetchContainerLogs uses the `containerCommand` to get the logs of all `containerNames` and put them in the `folder`.
func fetchContainerLogs(containerNames *[]string, folder string, containerCommand string) {
	By("Create directories and storing container logs")
	for _, name := range *containerNames {
		logDir := filepath.Join(folder, containerCommand, name)
		By(fmt.Sprintf("Create log directory for container %s at %s", name, logDir))
		createDirIfNotExist(logDir)
		By("Fetch logs for container " + name)
		cmd := exec.CommandContext(context.Background(), "sudo", containerCommand, "logs", name) // #nosec G204:gosec
		out, err := cmd.Output()
		if err != nil {
			writeErr := os.WriteFile(filepath.Join(logDir, "stderr.log"), []byte(err.Error()), 0400)
			Expect(writeErr).ToNot(HaveOccurred())
			log.Fatal(err)
		}
		writeErr := os.WriteFile(filepath.Join(logDir, "stdout.log"), out, 0400)
		Expect(writeErr).ToNot(HaveOccurred())
	}
}
