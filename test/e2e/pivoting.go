package e2e

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	containerTypes "github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
	numberOfWorkers := int(*input.E2EConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
	numberOfControlplane := int(*input.E2EConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
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
	ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
	fetchContainerLogs(&generalContainers, input.ArtifactFolder, input.E2EConfig.GetVariable("CONTAINER_RUNTIME"))
	if ephemeralCluster == Kind {
		fetchContainerLogs(&ironicContainers, input.ArtifactFolder, input.E2EConfig.GetVariable("CONTAINER_RUNTIME"))
	}

	By("Fetch manifest for bootstrap cluster before pivot")
	kconfigPathBootstrap := input.BootstrapClusterProxy.GetKubeconfigPath()
	os.Setenv("KUBECONFIG_BOOTSTRAP", kconfigPathBootstrap)
	path := filepath.Join(os.Getenv("CAPM3PATH"), "scripts")
	cmd := exec.Command("./fetch_manifests.sh") // #nosec G204:gosec
	cmd.Dir = path
	_ = cmd.Run()

	By("Fetch target cluster kubeconfig for target cluster log collection")
	kconfigPathWorkload := input.TargetCluster.GetKubeconfigPath()
	os.Setenv("KUBECONFIG_WORKLOAD", kconfigPathWorkload)
	Logf("Save kubeconfig in temp folder for project-infra target log collection")
	// TODO(smoshiur1237): This is a workaround to copy the target kubeconfig and enable project-infra
	// target log collection. There is possibility to handle the kubeconfig in better way.
	// KubeconfigPathTemp will be used by project-infra target log collection only incase of failed e2e test
	kubeconfigPathTemp := "/tmp/kubeconfig-test1.yaml"
	cmd = exec.Command("cp", kconfigPathWorkload, kubeconfigPathTemp) // #nosec G204:gosec
	stdoutStderr, er := cmd.CombinedOutput()
	Logf("%s\n", stdoutStderr)
	Expect(er).ToNot(HaveOccurred(), "Cannot fetch target cluster kubeconfig")

	By("Remove Ironic containers from the source cluster")
	isIronicDeployment := true
	if ephemeralCluster == Kind {
		isIronicDeployment = false
	}
	removeIronic(ctx, func() RemoveIronicInput {
		return RemoveIronicInput{
			ManagementCluster: input.BootstrapClusterProxy,
			IsDeployment:      isIronicDeployment,
			Namespace:         input.E2EConfig.GetVariable(ironicNamespace),
			NamePrefix:        input.E2EConfig.GetVariable(NamePrefix),
		}
	})

	By("Create Ironic namespace")
	targetClusterClientSet := input.TargetCluster.GetClientSet()
	ironicNamespaceObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: input.E2EConfig.GetVariable(ironicNamespace),
		},
	}
	_, err = targetClusterClientSet.CoreV1().Namespaces().Create(ctx, ironicNamespaceObj, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred(), "Unable to create the Ironic namespace")

	By("Initialize Provider component in target cluster")
	clusterctl.Init(ctx, clusterctl.InitInput{
		KubeconfigPath:          input.TargetCluster.GetKubeconfigPath(),
		ClusterctlConfigPath:    input.E2EConfig.GetVariable("CONFIG_FILE_PATH"),
		CoreProvider:            config.ClusterAPIProviderName + ":" + os.Getenv("CAPIRELEASE"),
		BootstrapProviders:      []string{config.KubeadmBootstrapProviderName + ":" + os.Getenv("CAPIRELEASE")},
		ControlPlaneProviders:   []string{config.KubeadmControlPlaneProviderName + ":" + os.Getenv("CAPIRELEASE")},
		InfrastructureProviders: []string{config.Metal3ProviderName + ":" + os.Getenv("CAPM3RELEASE")},
		LogFolder:               filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName+"-pivoting"),
	})

	LogFromFile(filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName+"-pivoting", "clusterctl-init.log"))

	By("Add labels to BMO CRDs")
	labelBMOCRDs(ctx, input.BootstrapClusterProxy)
	By("Add Labels to hardwareData CRDs")
	labelHDCRDs(ctx, input.BootstrapClusterProxy)

	By("Install Ironic in the target cluster")
	installIronicBMO(ctx, func() installIronicBMOInput {
		return installIronicBMOInput{
			ManagementCluster:          input.TargetCluster,
			BMOPath:                    input.E2EConfig.GetVariable(bmoPath),
			deployIronic:               true,
			deployBMO:                  false,
			deployIronicTLSSetup:       getBool(input.E2EConfig.GetVariable(ironicTLSSetup)),
			deployIronicBasicAuth:      getBool(input.E2EConfig.GetVariable(ironicBasicAuth)),
			deployIronicKeepalived:     getBool(input.E2EConfig.GetVariable(ironicKeepalived)),
			deployIronicMariadb:        getBool(input.E2EConfig.GetVariable(ironicMariadb)),
			Namespace:                  ironicNamespaceObj.Name,
			NamePrefix:                 input.E2EConfig.GetVariable(NamePrefix),
			RestartContainerCertUpdate: getBool(input.E2EConfig.GetVariable(restartContainerCertUpdate)),
			E2EConfig:                  input.E2EConfig,
			SpecName:                   input.SpecName,
		}
	})

	By("Install BMO")
	installIronicBMO(ctx, func() installIronicBMOInput {
		return installIronicBMOInput{
			ManagementCluster:          input.TargetCluster,
			BMOPath:                    input.E2EConfig.GetVariable(bmoPath),
			deployIronic:               false,
			deployBMO:                  true,
			deployIronicTLSSetup:       getBool(input.E2EConfig.GetVariable(ironicTLSSetup)),
			deployIronicBasicAuth:      getBool(input.E2EConfig.GetVariable(ironicBasicAuth)),
			deployIronicKeepalived:     getBool(input.E2EConfig.GetVariable(ironicKeepalived)),
			deployIronicMariadb:        getBool(input.E2EConfig.GetVariable(ironicMariadb)),
			Namespace:                  ironicNamespaceObj.Name,
			NamePrefix:                 input.E2EConfig.GetVariable(NamePrefix),
			RestartContainerCertUpdate: getBool(input.E2EConfig.GetVariable(restartContainerCertUpdate)),
			E2EConfig:                  input.E2EConfig,
			SpecName:                   input.SpecName,
		}
	})

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
			Namespace:         input.E2EConfig.GetVariable(ironicNamespace),
			Name:              input.E2EConfig.GetVariable(NamePrefix) + "-controller-manager",
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

type installIronicBMOInput struct {
	ManagementCluster          framework.ClusterProxy
	BMOPath                    string
	deployIronic               bool
	deployBMO                  bool
	deployIronicTLSSetup       bool
	deployIronicBasicAuth      bool
	deployIronicKeepalived     bool
	deployIronicMariadb        bool
	Namespace                  string
	NamePrefix                 string
	RestartContainerCertUpdate bool
	E2EConfig                  *clusterctl.E2EConfig
	SpecName                   string
}

func installIronicBMO(ctx context.Context, inputGetter func() installIronicBMOInput) {
	input := inputGetter()

	ironicHost := os.Getenv("CLUSTER_BARE_METAL_PROVISIONER_IP")
	path := fmt.Sprintf("%s/tools/", input.BMOPath)

	args := []string{}
	if input.deployBMO {
		args = append(args, "-b")
	}
	if input.deployIronic {
		args = append(args, "-i")
	}
	if input.deployIronicTLSSetup {
		args = append(args, "-t")
	}
	if !input.deployIronicBasicAuth {
		args = append(args, "-n")
	}
	if input.deployIronicKeepalived {
		args = append(args, "-k")
	}
	if input.deployIronicMariadb {
		args = append(args, "-m")
	}

	env := []string{
		fmt.Sprintf("IRONIC_HOST=%s", ironicHost),
		fmt.Sprintf("IRONIC_HOST_IP=%s", ironicHost),
		fmt.Sprintf("KUBECTL_ARGS=--kubeconfig=%s", input.ManagementCluster.GetKubeconfigPath()),
		fmt.Sprintf("NAMEPREFIX=%s", input.NamePrefix),
		fmt.Sprintf("RESTART_CONTAINER_CERTIFICATE_UPDATED=%s", strconv.FormatBool(input.RestartContainerCertUpdate)),
		"USER=ubuntu",
	}
	cmd := exec.Command("./deploy.sh", args...) // #nosec G204:gosec
	cmd.Dir = path
	cmd.Env = append(env, os.Environ()...)

	stdoutStderr, er := cmd.CombinedOutput()
	Logf("%s\n", stdoutStderr)
	Expect(er).ToNot(HaveOccurred(), "Failed to deploy Ironic")
	deploymentNameList := []string{}
	if input.deployIronic {
		deploymentNameList = append(deploymentNameList, ironicSuffix)
	}
	if input.deployBMO {
		deploymentNameList = append(deploymentNameList, "-controller-manager")
	}
	// Wait for the deployments to become available
	clientSet := input.ManagementCluster.GetClientSet()
	for _, name := range deploymentNameList {
		deployment, err := clientSet.AppsV1().Deployments(input.Namespace).Get(ctx, input.NamePrefix+name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred(), "Unable to get the deployment %s in namespace %s \n error message: %s", input.NamePrefix+name, input.Namespace, err)
		framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
			Getter:     input.ManagementCluster.GetClient(),
			Deployment: deployment,
		}, input.E2EConfig.GetIntervals(input.SpecName, "wait-deployment")...)
	}
}

type RemoveIronicInput struct {
	ManagementCluster framework.ClusterProxy
	IsDeployment      bool
	Namespace         string
	NamePrefix        string
}

func removeIronic(ctx context.Context, inputGetter func() RemoveIronicInput) {
	input := inputGetter()
	if input.IsDeployment {
		deploymentName := input.NamePrefix + ironicSuffix
		RemoveDeployment(ctx, func() RemoveDeploymentInput {
			return RemoveDeploymentInput{
				ManagementCluster: input.ManagementCluster,
				Namespace:         input.Namespace,
				Name:              deploymentName,
			}
		})
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
	numberOfWorkers := int(*input.E2EConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
	numberOfControlplane := int(*input.E2EConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
	numberOfAllBmh := numberOfWorkers + numberOfControlplane

	By("Fetch logs from target cluster after pivot")
	err := FetchClusterLogs(input.TargetCluster, filepath.Join(clusterLogCollectionBasePath, "afterPivot"))
	if err != nil {
		Logf("Error: %v", err)
	}

	By("Fetch manifest for workload cluster after pivot")
	path := filepath.Join(os.Getenv("CAPM3PATH"), "scripts")
	cmd := exec.Command("./fetch_manifests.sh") // #nosec G204:gosec
	cmd.Dir = path
	_ = cmd.Run()
	os.Unsetenv("KUBECONFIG_WORKLOAD")

	By("Remove Ironic deployment from target cluster")
	removeIronic(ctx, func() RemoveIronicInput {
		return RemoveIronicInput{
			ManagementCluster: input.TargetCluster,
			IsDeployment:      true,
			Namespace:         input.E2EConfig.GetVariable(ironicNamespace),
			NamePrefix:        input.E2EConfig.GetVariable(NamePrefix),
		}
	})

	By("Reinstate BMO in Source cluster")
	installIronicBMO(ctx, func() installIronicBMOInput {
		return installIronicBMOInput{
			ManagementCluster:          input.BootstrapClusterProxy,
			BMOPath:                    input.E2EConfig.GetVariable(bmoPath),
			deployIronic:               false,
			deployBMO:                  true,
			deployIronicTLSSetup:       getBool(input.E2EConfig.GetVariable(ironicTLSSetup)),
			deployIronicBasicAuth:      getBool(input.E2EConfig.GetVariable(ironicBasicAuth)),
			deployIronicKeepalived:     getBool(input.E2EConfig.GetVariable(ironicKeepalived)),
			deployIronicMariadb:        getBool(input.E2EConfig.GetVariable(ironicMariadb)),
			Namespace:                  input.E2EConfig.GetVariable(ironicNamespace),
			NamePrefix:                 input.E2EConfig.GetVariable(NamePrefix),
			RestartContainerCertUpdate: getBool(input.E2EConfig.GetVariable(restartContainerCertUpdate)),
			E2EConfig:                  input.E2EConfig,
			SpecName:                   input.SpecName,
		}
	})

	By("Reinstate Ironic containers and BMH")
	// TODO(mboukhalfa): add this local ironic deployment case to installIronicBMO function
	ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
	if ephemeralCluster == Kind {
		bmoPath := input.E2EConfig.GetVariable("BMOPATH")
		ironicCommand := bmoPath + "/tools/run_local_ironic.sh"
		//#nosec G204:gosec
		cmd := exec.Command("sh", "-c", "export CONTAINER_RUNTIME=docker; "+ironicCommand)
		stdoutStderr, err := cmd.CombinedOutput()
		fmt.Printf("%s\n", stdoutStderr)
		Expect(err).ToNot(HaveOccurred(), "Cannot run local ironic")
	} else {
		By("Install Ironic in the bootstrap cluster")
		installIronicBMO(ctx, func() installIronicBMOInput {
			return installIronicBMOInput{
				ManagementCluster:          input.BootstrapClusterProxy,
				BMOPath:                    input.E2EConfig.GetVariable(bmoPath),
				deployIronic:               true,
				deployBMO:                  false,
				deployIronicTLSSetup:       getBool(input.E2EConfig.GetVariable(ironicTLSSetup)),
				deployIronicBasicAuth:      getBool(input.E2EConfig.GetVariable(ironicBasicAuth)),
				deployIronicKeepalived:     getBool(input.E2EConfig.GetVariable(ironicKeepalived)),
				deployIronicMariadb:        getBool(input.E2EConfig.GetVariable(ironicMariadb)),
				Namespace:                  input.E2EConfig.GetVariable(ironicNamespace),
				NamePrefix:                 input.E2EConfig.GetVariable(NamePrefix),
				RestartContainerCertUpdate: getBool(input.E2EConfig.GetVariable(restartContainerCertUpdate)),
				E2EConfig:                  input.E2EConfig,
				SpecName:                   input.SpecName,
			}
		})
	}

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
	path = filepath.Join(os.Getenv("CAPM3PATH"), "scripts")
	cmd = exec.Command("./fetch_manifests.sh") // #nosec G204:gosec
	cmd.Dir = path
	_ = cmd.Run()
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
		By(fmt.Sprintf("Fetch logs for container %s", name))
		cmd := exec.Command("sudo", containerCommand, "logs", name) // #nosec G204:gosec
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
