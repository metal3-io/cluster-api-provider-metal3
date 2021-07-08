package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type vmState string

const (
	running vmState = "running"
	paused  vmState = "paused"
	shutoff vmState = "shutoff"
	other   vmState = "other"
)
const rebootAnnotation = "reboot.metal3.io"
const poweroffAnnotation = "reboot.metal3.io/poweroff"

var _ = Describe("Remediation Pivoting", func() {
	var (
		ctx                      = context.TODO()
		specName                 = "metal3"
		namespace                = "metal3"
		cluster                  *clusterv1.Cluster
		clusterName              = "test1"
		clusterctlLogFolder      string
		controlPlaneMachineCount int64 = 3
		workerMachineCount       int64 = 1
	)

	// functions defined here to use the local variables

	assertNodeStatus := func(client client.Client, name types.NamespacedName, status v1.ConditionStatus) error {
		node := &v1.Node{}
		Expect(client.Get(ctx, name, node)).To(Succeed())
		for _, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				if status == condition.Status {
					return nil
				} else {
					return fmt.Errorf("Node %s has status '%s', should have '%s'", name.Name, condition.Status, status)
				}
			}
		}
		return fmt.Errorf("Node %s missing condition \"Ready\"", name.Name)
	}

	waitForVmState := func(vmName string, state vmState) {
		Expect(strings.TrimSpace(vmName)).NotTo(Equal(""))
		By(fmt.Sprintf("Waiting for VM %s to become '%s'", vmName, state))
		Eventually(func() error {
			vms := listVms(state)
			for _, name := range vms {
				if name == vmName {
					return nil
				}
			}
			return fmt.Errorf("VM '%s' not listed with state '%s'", vmName, state)
		}, e2eConfig.GetIntervals(specName, "wait-vm-state")...).Should(Succeed())
	}

	waitForNodeStatus := func(client client.Client, name types.NamespacedName, status v1.ConditionStatus) {
		By(fmt.Sprintf("Waiting for Node %s to have ready=%s status", name.Name, status))
		Eventually(
			func() error { return assertNodeStatus(client, name, status) },
			e2eConfig.GetIntervals(specName, "wait-vm-state")...,
		).Should(Succeed())
	}

	monitorNodeStatus := func(client client.Client, name types.NamespacedName, status v1.ConditionStatus) {
		// TODO look int gomega 1.14 to use assertions in the func

		By(fmt.Sprintf("Ensuring Node %s consistently hsa ready=%s status", name.Name, status))
		Consistently(
			func() error { return assertNodeStatus(client, name, status) },
			e2eConfig.GetIntervals(specName, "monitor-vm-state")...,
		).Should(Succeed())
	}

	rebootBmh := func(client client.Client, host bmh.BareMetalHost) {
		helper, err := patch.NewHelper(&host, client)
		Expect(err).ToNot(HaveOccurred())

		annotations := host.GetAnnotations()

		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[rebootAnnotation] = ""
		host.SetAnnotations(annotations)
		Expect(helper.Patch(ctx, &host)).To(Succeed())
	}

	// powerCycleBmh := func(client client.Client, host bmh.BareMetalHost) {
	// 	helper, err := patch.NewHelper(&host, client)
	// 	Expect(err).ToNot(HaveOccurred())

	// 	annotations := host.GetAnnotations()
	// 	if annotations == nil {
	// 		annotations = make(map[string]string)
	// 	}
	// 	annotations[poweroffAnnotation] = ""
	// 	host.SetAnnotations(annotations)
	// 	Expect(helper.Patch(ctx, &host)).To(Succeed())

	// 	delete(annotations, poweroffAnnotation)
	// 	host.SetAnnotations(annotations)
	// 	Expect(helper.Patch(ctx, &host)).To(Succeed())
	// }

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

	It("Apply cluster", func() {
		By("Creating a a cluster with 3 control-plane and 1 worker nodes")
		cluster = clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
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
				ControlPlaneMachineCount: &controlPlaneMachineCount,
				WorkerMachineCount:       &workerMachineCount,
			},
			CNIManifestPath:              e2eTestsPath + "/data/cni/calico/calico.yaml",
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}).Cluster

	})

	It("Run test: remediation", func() {

		// log.Logf("Waiting for the cluster infrastructure to be detected")
		// cluster := framework.DiscoveryAndWaitForCluster(ctx, framework.DiscoveryAndWaitForClusterInput{
		// 	Getter:    input.ClusterProxy.GetClient(),
		// 	Namespace: input.ConfigCluster.Namespace,
		// 	Name:      input.ConfigCluster.ClusterName,
		// }, input.WaitForClusterIntervals...)

		By("Checking that rebooted node becomes Ready")
		targetCluster := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
		targetClient := targetCluster.GetClient()

		fmt.Println("KubeconfigPath:", bootstrapClusterProxy.GetKubeconfigPath())
		client := bootstrapClusterProxy.GetClient()
		machines := &clusterv1.MachineList{}
		Eventually(func() error {
			if err := client.List(ctx, machines, byClusterOptions(clusterName, namespace)...); err != nil {
				return err
			}

			Expect(machines.Items).To((HaveLen(int(controlPlaneMachineCount + workerMachineCount))))
			for _, machine := range machines.Items {
				if !strings.EqualFold(machine.Status.Phase, "running") { // Case insensitive comparison
					return errors.New("Machine is not in 'running' phase")
				}
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(Succeed())

		ctrlM3Machines, workerM3Machines, err := getMetal3Machines(ctx, client, clusterName, namespace)
		Expect(err).NotTo(HaveOccurred())

		for _, m3machine := range ctrlM3Machines {
			fmt.Printf("m3 name: %s bmh name: %s \n", m3machine.ObjectMeta.Name, metal3MachineToBmhName(m3machine))
		}
		for _, m3machine := range workerM3Machines {
			fmt.Printf("m3 name: %s bmh name: %s \n", m3machine.ObjectMeta.Name, metal3MachineToBmhName(m3machine))
		}

		workerToReboot := workerM3Machines[0]
		bmhToReboot := bmh.BareMetalHost{}
		Expect(client.Get(ctx,
			types.NamespacedName{Namespace: namespace, Name: metal3MachineToBmhName(workerToReboot)},
			&bmhToReboot)).To(Succeed())

		machineName, err := metal3MachineToMachineName(workerToReboot)
		Expect(err).ToNot(HaveOccurred())
		nodeName := machineName
		vmName := bmhToVmName(bmhToReboot)

		By("Marking a BMH for reboot")
		rebootBmh(client, bmhToReboot)

		waitForVmState(vmName, shutoff)

		// Note: what is reported in the CLI as NotReady, is initially unknown status
		// This call will wait for the actual "False" condition status
		waitForNodeStatus(targetClient, types.NamespacedName{Namespace: "default", Name: nodeName}, v1.ConditionFalse)
		waitForVmState(vmName, running)
		waitForNodeStatus(targetClient, types.NamespacedName{Namespace: "default", Name: nodeName}, v1.ConditionTrue)
		monitorNodeStatus(targetClient, types.NamespacedName{Namespace: "default", Name: nodeName}, v1.ConditionTrue)

		// power cycle

		helper, err := patch.NewHelper(&bmhToReboot, client)
		Expect(err).ToNot(HaveOccurred())

		annotations := bmhToReboot.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[poweroffAnnotation] = ""
		bmhToReboot.SetAnnotations(annotations)
		Expect(helper.Patch(ctx, &bmhToReboot)).To(Succeed())

		waitForVmState(vmName, shutoff)
		waitForNodeStatus(targetClient, types.NamespacedName{Namespace: "default", Name: nodeName}, v1.ConditionUnknown)
		monitorNodeStatus(targetClient, types.NamespacedName{Namespace: "default", Name: nodeName}, v1.ConditionUnknown)

		// power on
		delete(annotations, poweroffAnnotation)
		bmhToReboot.SetAnnotations(annotations)
		Expect(helper.Patch(ctx, &bmhToReboot)).To(Succeed())
		waitForVmState(vmName, running)
		waitForNodeStatus(targetClient, types.NamespacedName{Namespace: "default", Name: nodeName}, v1.ConditionTrue)
		monitorNodeStatus(targetClient, types.NamespacedName{Namespace: "default", Name: nodeName}, v1.ConditionTrue)

	})

})

func metal3MachineToMachineName(m3machine v1alpha4.Metal3Machine) (string, error) {
	ownerReferences := m3machine.GetOwnerReferences()
	for _, reference := range ownerReferences {
		if reference.Kind == "Machine" {
			return reference.Name, nil
		}
	}
	return "", fmt.Errorf("metal3machine missing a \"Machine\" kind owner reference")
}

func metal3MachineToBmhName(m3machine v1alpha4.Metal3Machine) string {
	return strings.Replace(m3machine.GetAnnotations()["metal3.io/BareMetalHost"], "metal3/", "", 1)
}

// Derives the name of a VM created by metal3-dev-env from the name of a BareMetalHost object
func bmhToVmName(host bmh.BareMetalHost) string {
	return strings.ReplaceAll(host.Name, "-", "_")
}

func listVms(state vmState) []string {
	var flag string
	switch state {
	case running:
		flag = "--state-running"
	case shutoff:
		flag = "--state-shutoff"
	case paused:
		flag = "--state-paused"
	case other:
		flag = "--state-other"
	}

	cmd := exec.Command("virsh", "list", "--name", flag)
	result, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())

	// virsh may return some empty lines which need to be removed
	lines := strings.Split(string(result), "\n")
	i := 0
	for _, line := range lines {
		if line != "" {
			lines[i] = line
			i++
		}
	}
	return lines[:i]
}

func getAllBMH(ctx context.Context, client client.Client, clusterName, namespace, specName string) bmh.BareMetalHostList {

	bmhs := bmh.BareMetalHostList{}
	Eventually(func() error {
		if err := client.List(ctx, &bmhs, byClusterOptions(clusterName, namespace)...); err != nil {
			fmt.Println(err)
			return err
		}
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-bmh")...).Should(Succeed())

	for _, item := range bmhs.Items {
		fmt.Printf("bmh: %+v\n", item)
		fmt.Printf("bmh annotations: %+v\n", item.GetAnnotations())
	}

	return bmhs
}

func getMetal3Machines(ctx context.Context, client client.Client, cluster, namespace string) (controlplane, workers []v1alpha4.Metal3Machine, err error) {
	allMachines := &v1alpha4.Metal3MachineList{}
	if err = client.List(ctx, allMachines, byClusterOptions(cluster, namespace)...); err != nil {
		return
	}
	for _, machine := range allMachines.Items {
		if strings.Contains(machine.ObjectMeta.Name, "workers") {
			workers = append(workers, machine)
		} else {
			controlplane = append(controlplane, machine)
		}
	}
	return
}

// byClusterOptions returns a set of ListOptions that allows to identify all the objects belonging to a Cluster.
func byClusterOptions(name, namespace string) []client.ListOption {
	return []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			clusterv1.ClusterLabelName: name,
		},
	}
}
