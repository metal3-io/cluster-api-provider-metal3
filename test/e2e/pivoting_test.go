package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/utils/pointer"

	bmo "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	//"sigs.k8s.io/cluster-api/test/e2e/internal/log"
	dockerTypes "github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	framework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Doing Pivoting", func() {
	var (
		ctx                 = context.TODO()
		specName            = "metal3"
		namespace           = "metal3"
		cluster             *clusterv1.Cluster
		clusterName         = "test1"
		clusterctlLogFolder string
	)

	BeforeEach(func() {
		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)

		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, cluster, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})

	Context("Creating a highly available control-plane cluster", func() {
		It("Should create a cluster with 1 control-plane and 1 worker nodes", func() {
			By("Creating a cluster")
			result := clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ha",
					Namespace:                namespace,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				CNIManifestPath:              e2eTestsPath + "/data/cni/calico/calico.yaml",
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			})
			cluster = result.Cluster

			By("Remove Ironic containers in the source cluster")
			ironicContainerList := []string{
				"ironic-api",
				"ironic-conductor",
				"ironic-inspector",
				"dnsmasq",
				"mariadb",
				"ironic-endpoint-keepalived",
				"ironic-log-watch",
				"ironic-inspector-log-watch",
			}
			// run docker command to delete
			//TODO: Delete the link below
			// https://gist.github.com/frikky/e2efcea6c733ea8d8d015b7fe8a91bf6
			dockerClient, err := docker.NewEnvClient()
			Expect(err).To(BeNil(), "Unable to get docker client")
			removeOptions := dockerTypes.ContainerRemoveOptions{
				Force: true,
			}
			for _, container := range ironicContainerList {
				err = dockerClient.ContainerRemove(ctx, container, removeOptions)
				Expect(err).To(BeNil(), "Unable to delete the container %s, ", container, err)
			}

			By("Create Ironic namespace")
			targetCluster := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
			targetClusterClient := targetCluster.GetClientSet()
			// get namespace from env. Need to check
			ironicNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: e2eConfig.GetVariable(IronicNamespace),
				},
			}
			_, err = targetClusterClient.CoreV1().Namespaces().Create(ironicNamespace)
			Expect(err).To(BeNil(), "Unable to create the Ironic namespace")

			By("Initialize Provider component in target cluster")
			clusterctl.Init(context.TODO(), clusterctl.InitInput{
				KubeconfigPath:          targetCluster.GetKubeconfigPath(),
				ClusterctlConfigPath:    clusterctlConfigPath,
				CoreProvider:            config.ClusterAPIProviderName,
				BootstrapProviders:      []string{config.KubeadmBootstrapProviderName},
				ControlPlaneProviders:   []string{config.KubeadmControlPlaneProviderName},
				InfrastructureProviders: e2eConfig.InfrastructureProviders(),
				LogFolder:               filepath.Join(artifactFolder, "clusters", clusterName+"-pivoting"),
			})

			By("Configure Ironic Configmap")
			configureIronicConfigmap(true)

			By("Install Ironic in the target cluster")
			installIronic(targetCluster)

			By("Reinstate Ironic Configmap")
			configureIronicConfigmap(false)

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

			//log.Logf("Waiting for the cluster to be reconciled after moving to self hosted")
			pivotingCluster := framework.DiscoveryAndWaitForCluster(ctx, framework.DiscoveryAndWaitForClusterInput{
				Getter:    targetCluster.GetClient(),
				Namespace: namespace,
				Name:      cluster.Name,
			}, e2eConfig.GetIntervals(specName, "wait-cluster")...)

			controlPlane := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
				Lister:      targetCluster.GetClient(),
				ClusterName: pivotingCluster.Name,
				Namespace:   pivotingCluster.Namespace,
			})
			Expect(controlPlane).ToNot(BeNil())

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
			}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(BeNil())

			By("Check if metal3machines become provisioned.")
			Eventually(func() error {
				m3Machines := &capm3.Metal3MachineList{}
				if err := targetCluster.GetClient().List(ctx, m3Machines, client.InNamespace(namespace)); err != nil {
					return err
				}
				for _, m3Machine := range m3Machines.Items {
					if !m3Machine.Status.Ready { // Case insensitive comparison
						return errors.New("Metal3Machines cannot be ready")
					}
				}
				return nil
			}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(BeNil())

			By("Check if bmh is in provisioned state")
			Eventually(func() error {
				bmoList := &bmo.BareMetalHostList{}
				if err := targetCluster.GetClient().List(ctx, bmoList, client.InNamespace(namespace)); err != nil {
					return err
				}
				for _, bmo := range bmoList.Items {
					if !bmo.WasProvisioned() { // Case insensitive comparison
						return errors.New("BMOs cannot be provisioned")
					}
				}
				return nil
			}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(BeNil())

			By("PASSED!")
		})
	})
})

func configureIronicConfigmap(isIronicDeployed bool) {
	ironicConfigmap := fmt.Sprintf("%s/ironic-deployment/keepalived/ironic_bmo_configmap.env", e2eConfig.GetVariable(BmoPath))
	newIronicConfigmap := fmt.Sprintf("%s/ironic_bmo_configmap.env", e2eConfig.GetVariable(IronicDataDir))
	backupIronicConfigmap := fmt.Sprintf("%s/ironic-deployment/keepalived/ironic_bmo_configmap.env.orig", e2eConfig.GetVariable(BmoPath))
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

func installIronic(targetCluster framework.ClusterProxy) {
	// TODO: Delete the comments
	// - name: Install Ironic
	//     shell: "{{ BMOPATH }}/tools/deploy.sh false true {{ IRONIC_TLS_SETUP }} {{ IRONIC_BASIC_AUTH }} true"
	//     environment:
	//       IRONIC_HOST: "{{ IRONIC_HOST }}"
	//       IRONIC_HOST_IP: "{{ IRONIC_HOST_IP }}"
	//       KUBECTL_ARGS: "{{ KUBECTL_ARGS }}"
	path := fmt.Sprintf("%s/tools/", e2eConfig.GetVariable(BmoPath))
	args := []string{
		"false",
		"true",
		e2eConfig.GetVariable(IronicTLSEnable),
		e2eConfig.GetVariable(IronicBasicAuth),
		"true",
	}
	env := []string{
		fmt.Sprintf("IRONIC_HOST=%s", e2eConfig.GetVariable(IronicHost)),
		fmt.Sprintf("IRONIC_HOST_IP=%s", e2eConfig.GetVariable(IronicHost)),
		fmt.Sprintf("KUBECTL_ARGS=--kubeconfig=%s", targetCluster.GetKubeconfigPath()),
		"USER=ubuntu",
	}
	cmd := exec.Command("./deploy.sh", args...)
	cmd.Dir = path
	cmd.Env = append(env, os.Environ()...)

	err := cmd.Start()
	Expect(err).To(BeNil(), "Fail to deploy Ironic")
	err = cmd.Wait()
	Expect(err).To(BeNil(), "Fail to deploy Ironic")
}
