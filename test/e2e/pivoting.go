package e2e

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
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

const (
	bmoPath                    = "BMOPATH"
	ironicTLSSetup             = "IRONIC_TLS_SETUP"
	ironicBasicAuth            = "IRONIC_BASIC_AUTH"
	Kind                       = "kind"
	NamePrefix                 = "NAMEPREFIX"
	restartContainerCertUpdate = "RESTART_CONTAINER_CERTIFICATE_UPDATED"
	ironicNamespace            = "IRONIC_NAMESPACE"
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

    ironicContainers = []string{
        "ironic",
        "ironic-inspector",
        "ironic-endpoint-keepalived",
        "ironic-log-watch",
        "dnsmasq",
    }
    generalContainers = []string{
        "httpd-infra",
        "registry",
        "sushy-tools",
        "vbmc",
    }

    By("Fetch container logs")
    ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
    fetchContainerLogs(&generalContainers)
    if ephemeralCluster == Kind {
        fetchContainerLogs(&ironicContainers)
    }

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
    ironicNamespace := &corev1.Namespace{
        ObjectMeta: metav1.ObjectMeta{
            Name: input.E2EConfig.GetVariable(ironicNamespace),
        },
    }
    _, err := targetClusterClientSet.CoreV1().Namespaces().Create(ctx, ironicNamespace, metav1.CreateOptions{})
    Expect(err).To(BeNil(), "Unable to create the Ironic namespace")

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
    labelBMOCRDs(nil)
    By("Add Labels to hardwareData CRDs")
    labelHDCRDs(nil)

    By("Install BMO")
    installIronicBMO(func() installIronicBMOInput {
        return installIronicBMOInput{
            ManagementCluster:          input.TargetCluster,
            BMOPath:                    input.E2EConfig.GetVariable(bmoPath),
            deployIronic:               false,
            deployBMO:                  true,
            deployIronicTLSSetup:       getBool(input.E2EConfig.GetVariable(ironicTLSSetup)),
            DeployIronicBasicAuth:      getBool(input.E2EConfig.GetVariable(ironicBasicAuth)),
            NamePrefix:                 input.E2EConfig.GetVariable(NamePrefix),
            RestartContainerCertUpdate: getBool(input.E2EConfig.GetVariable(restartContainerCertUpdate)),
        }
    })

    By("Install Ironic in the target cluster")
    installIronicBMO(func() installIronicBMOInput {
        return installIronicBMOInput{
            ManagementCluster:          input.TargetCluster,
            BMOPath:                    input.E2EConfig.GetVariable(bmoPath),
            deployIronic:               true,
            deployBMO:                  false,
            deployIronicTLSSetup:       getBool(input.E2EConfig.GetVariable(ironicTLSSetup)),
            DeployIronicBasicAuth:      getBool(input.E2EConfig.GetVariable(ironicBasicAuth)),
            NamePrefix:                 input.E2EConfig.GetVariable(NamePrefix),
            RestartContainerCertUpdate: getBool(input.E2EConfig.GetVariable(restartContainerCertUpdate)),
        }
    })

    By("Add labels to BMO CRDs in the target cluster")
    labelBMOCRDs(input.TargetCluster)
    By("Add Labels to hardwareData CRDs in the target cluster")
    labelHDCRDs(input.TargetCluster)
    By("Ensure API servers are stable before doing move")
    // Nb. This check was introduced to prevent doing move to self-hosted in an aggressive way and thus avoid flakes.
    // More specifically, we were observing the test failing to get objects from the API server during move, so we
    // are now testing the API servers are stable before starting move.
    Consistently(func() error {
        kubeSystem := &corev1.Namespace{}
        return input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
    }, "5s", "100ms").Should(BeNil(), "Failed to assert bootstrap API server stability")
    Consistently(func() error {
        kubeSystem := &corev1.Namespace{}
        return input.TargetCluster.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
    }, "5s", "100ms").Should(BeNil(), "Failed to assert target API server stability")

    By("Moving the cluster to self hosted")
    clusterctl.Move(ctx, clusterctl.MoveInput{
        LogFolder:            filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName+"-bootstrap"),
        ClusterctlConfigPath: input.ClusterctlConfigPath,
        FromKubeconfigPath:   input.BootstrapClusterProxy.GetKubeconfigPath(),
        ToKubeconfigPath:     input.TargetCluster.GetKubeconfigPath(),
        Namespace:            input.Namespace,
    })
    LogFromFile(filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName+"-bootstrap", "logs", input.Namespace, "clusterctl-move.log"))

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
    DeployIronicBasicAuth      bool
    NamePrefix                 string
    RestartContainerCertUpdate bool
}

func installIronicBMO(inputGetter func() installIronicBMOInput) {
    input := inputGetter()

    ironicHost := os.Getenv("CLUSTER_PROVISIONING_IP")
    path := fmt.Sprintf("%s/tools/", input.BMOPath)
    args := []string{
        strconv.FormatBool(input.deployBMO),
        strconv.FormatBool(input.deployIronic),
        strconv.FormatBool(input.DeployIronicBasicAuth),
        strconv.FormatBool(input.deployIronicTLSSetup),
        "true",
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

type RemoveIronicInput struct {
	ManagementCluster framework.ClusterProxy
	IsDeployment      bool
	Namespace         string
	NamePrefix        string
}

func removeIronic(ctx context.Context, inputGetter func() RemoveIronicInput) {
	input := inputGetter()
	if input.IsDeployment {
		deploymentName := input.NamePrefix + "-ironic"
		ironicNamespace := input.Namespace
		err := input.ManagementCluster.GetClientSet().AppsV1().Deployments(ironicNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
		Expect(err).To(BeNil(), "Failed to delete Ironic from the source cluster")
	} else {
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
			cmd = exec.Command("kubectl", "label", "--overwrite", "crds", crdName, label) // #nosec G204:gosec
		} else {
			cmd = exec.Command("kubectl", kubectlArgs, "label", "--overwrite", "crds", crdName, label) // #nosec G204:gosec
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
			cmd = exec.Command("kubectl", "label", "--overwrite", "crds", crdName, label) // #nosec G204:gosec
		} else {
			cmd = exec.Command("kubectl", kubectlArgs, "label", "--overwrite", "crds", crdName, label) // #nosec G204:gosec
		}
		err := cmd.Run()
		Expect(err).To(BeNil(), "Cannot label HD CRDs")
	}
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
	By("Remove Ironic deployment from target cluster")
	removeIronic(ctx, func() RemoveIronicInput {
		return RemoveIronicInput{
			ManagementCluster: input.TargetCluster,
			IsDeployment:      true,
			Namespace:         input.E2EConfig.GetVariable(ironicNamespace),
			NamePrefix:        input.E2EConfig.GetVariable(NamePrefix),
		}
	})
	By("Reinstate Ironic containers and BMH")
	// TODO(mboukhalfa): add this local ironic deployment case to installIronicBMO function
	ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
	if ephemeralCluster == Kind {
		bmoPath := input.E2EConfig.GetVariable("BMOPATH")
		ironicCommand := bmoPath + "/tools/run_local_ironic.sh"
		cmd := exec.Command("sh", "-c", "export CONTAINER_RUNTIME=docker; "+ironicCommand) // #nosec G204:gosec
		stdoutStderr, err := cmd.CombinedOutput()
		fmt.Printf("%s\n", stdoutStderr)
		Expect(err).To(BeNil(), "Cannot run local ironic")
	} else {
		By("Install Ironic in the bootstrap cluster")
		installIronicBMO(func() installIronicBMOInput {
			return installIronicBMOInput{
				ManagementCluster:          input.BootstrapClusterProxy,
				BMOPath:                    input.E2EConfig.GetVariable(bmoPath),
				deployIronic:               true,
				deployBMO:                  false,
				deployIronicTLSSetup:       getBool(input.E2EConfig.GetVariable(ironicTLSSetup)),
				DeployIronicBasicAuth:      getBool(input.E2EConfig.GetVariable(ironicBasicAuth)),
				NamePrefix:                 input.E2EConfig.GetVariable(NamePrefix),
				RestartContainerCertUpdate: getBool(input.E2EConfig.GetVariable(restartContainerCertUpdate)),
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
	}, "5s", "100ms").Should(BeNil(), "Failed to assert bootstrap API server stability")

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

	By("RE-PIVOTING TEST PASSED!")
}
