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
	"k8s.io/utils/pointer"

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
				// fmt.Printf("Node %s has status '%s', should have '%s'\n", name.Name, condition.Status, status)
				if status == condition.Status {
					return nil
				} else {
					return fmt.Errorf("Node %s has status '%s', should have '%s'", name.Name, condition.Status, status)
				}
			}
		}
		return fmt.Errorf("Node %s missing condition \"Ready\"", name.Name)
	}

	waitForVmsState := func(vmNames []string, state vmState) {
		By(fmt.Sprintf("Waiting for VMs %#v to become '%s'", vmNames, state))
		Eventually(func() {
			vms := listVms(state)
			Expect(vms).To(ContainElements(vmNames))
		}, e2eConfig.GetIntervals(specName, "wait-vm-state")...).Should(Succeed())
	}

	waitForNodeStatus := func(client client.Client, name types.NamespacedName, status v1.ConditionStatus) {
		By(fmt.Sprintf("Waiting for Node '%s' to have ready=%s status", name, status))
		Eventually(
			func() error { return assertNodeStatus(client, name, status) },
			e2eConfig.GetIntervals(specName, "wait-vm-state")...,
		).Should(Succeed())
	}

	monitorNodesStatus := func(client client.Client, namespace string, names []string, status v1.ConditionStatus) {
		// TODO look int gomega 1.14 to use assertions in the func

		By(fmt.Sprintf("Ensuring Nodes %#v consistently have ready=%s status", names, status))
		Consistently(
			func() error {
				for _, node := range names {
					if err := assertNodeStatus(client, types.NamespacedName{Namespace: namespace, Name: node}, status); err != nil {
						return err
					}
				}
				return nil
			}, e2eConfig.GetIntervals(specName, "monitor-vm-state")...,
		).Should(Succeed())
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

		_ = func() {
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

			controlM3Machines, workerM3Machines, err := getMetal3Machines(ctx, client, clusterName, namespace)
			Expect(err).NotTo(HaveOccurred())

			getBmhFromM3Machine := func(m3Machine v1alpha4.Metal3Machine) (result bmh.BareMetalHost) {
				Expect(client.Get(ctx,
					types.NamespacedName{Namespace: namespace, Name: metal3MachineToBmhName(m3Machine)},
					&result)).To(Succeed())
				return result
			}

			controlMachineSets := make([]machineSet, len(controlM3Machines))
			for i, m3machine := range controlM3Machines {
				theBmh := getBmhFromM3Machine(m3machine)
				controlMachineSets[i] = machineSet{
					baremetalhost: &theBmh,
					metal3machine: &controlM3Machines[i],
				}
			}
			fmt.Printf("controlMachineSets: %s\n", controlMachineSets)

			for _, m3machine := range workerM3Machines {
				fmt.Printf("m3 name: %s bmh name: %s \n", m3machine.ObjectMeta.Name, metal3MachineToBmhName(m3machine))
			}

			workerM3Machine := workerM3Machines[0]
			workerBmh := getBmhFromM3Machine(workerM3Machine)

			machineName, err := metal3MachineToMachineName(workerM3Machine)
			Expect(err).ToNot(HaveOccurred())
			nodeName := machineName
			vmName := bmhToVmName(workerBmh)

			By("Marking a BMH for reboot")
			annotateBmh(ctx, client, workerBmh, rebootAnnotation, pointer.String(""))

			waitForVmsState([]string{vmName}, shutoff)

			// Note: what is reported in the CLI as NotReady, is initially unknown status
			// This call will wait for the actual "False" condition status
			waitForNodeStatus(targetClient, types.NamespacedName{Namespace: "default", Name: nodeName}, v1.ConditionUnknown)
			waitForVmsState([]string{vmName}, running)
			waitForNodeStatus(targetClient, types.NamespacedName{Namespace: "default", Name: nodeName}, v1.ConditionTrue)
			monitorNodesStatus(targetClient, "defaul", []string{nodeName}, v1.ConditionTrue)

			// power cycle

			powerCycle := func(machines machineSetSlice) error {
				By(fmt.Sprintf("Power cycling %d machines", len(machines)))
				for _, set := range machines {
					Expect(annotateBmh(ctx, client, *set.baremetalhost, poweroffAnnotation, pointer.String(""))).To(Succeed())
				}
				waitForVmsState(machines.getVmNames(), shutoff)

				// power on
				By("Marking a BMH for power on")
				for _, set := range machines {
					Expect(annotateBmh(ctx, client, *set.baremetalhost, poweroffAnnotation, nil)).To(Succeed())
				}

				// waitForVmState(vmName, running)
				waitForVmsState(machines.getVmNames(), running)
				for _, nodeName := range machines.getNodeNames() {
					waitForNodeStatus(targetClient, types.NamespacedName{Namespace: "default", Name: nodeName}, v1.ConditionTrue)
				}
				monitorNodesStatus(targetClient, "default", machines.getNodeNames(), v1.ConditionTrue)
				return nil
			}

			powerCycle(machineSetSlice{
				{
					baremetalhost: &workerBmh,
					metal3machine: &workerM3Machine,
				},
			})

			powerCycle(controlMachineSets[:1])
			powerCycle(controlMachineSets[1:3])

		}

		By("Testing unhealthy annotation")
	})

})

type machineSet struct {
	baremetalhost *bmh.BareMetalHost
	metal3machine *v1alpha4.Metal3Machine
}
type machineSetSlice []machineSet

func (ms machineSet) String() string {
	return fmt.Sprintf("machineSet{%p bmh:%s, %p metal3machine:%s}",
		ms.baremetalhost, ms.baremetalhost.GetName(),
		ms.metal3machine, ms.metal3machine.GetName(),
	)
}

func (msl machineSetSlice) getBMHs() (hosts []bmh.BareMetalHost) {
	for _, ms := range msl {
		if ms.baremetalhost != nil {
			hosts = append(hosts, *ms.baremetalhost)
		}
	}
	return
}

func (msl machineSetSlice) getVmNames() (names []string) {
	for _, host := range msl.getBMHs() {
		names = append(names, bmhToVmName(host))
	}
	return
}

func (msl machineSetSlice) getMachineNames() (names []string) {
	for _, ms := range msl {
		if ms.metal3machine != nil {
			name, err := metal3MachineToMachineName(*ms.metal3machine)
			Expect(err).NotTo(HaveOccurred())
			names = append(names, name)
		}
	}
	return
}

func (msl machineSetSlice) getNodeNames() []string {
	// nodes have the same names as machines
	return msl.getMachineNames()
}

func annotateBmh(ctx context.Context, client client.Client, host bmh.BareMetalHost, key string, value *string) error {
	helper, err := patch.NewHelper(&host, client)
	if err != nil {
		return err
	}
	annotations := host.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if value == nil {
		delete(annotations, key)
	} else {
		annotations[key] = *value
	}
	host.SetAnnotations(annotations)
	return helper.Patch(ctx, &host)
}

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
