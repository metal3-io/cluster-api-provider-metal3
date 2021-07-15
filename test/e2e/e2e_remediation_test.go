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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	kcp "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
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

		By("Checking that rebooted node becomes Ready")
		targetCluster := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
		targetClient := targetCluster.GetClient()

		fmt.Println("KubeconfigPath:", bootstrapClusterProxy.GetKubeconfigPath())
		bootstrapClient := bootstrapClusterProxy.GetClient()

		getMachine := func(name client.ObjectKey) (result clusterv1.Machine) {
			By(fmt.Sprintf("Getting machine %s", name))
			Expect(bootstrapClient.Get(ctx, name, &result)).To(Succeed())
			return
		}

		machines := &clusterv1.MachineList{}
		Eventually(func() error {
			if err := bootstrapClient.List(ctx, machines, client.InNamespace(namespace)); err != nil {
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

		controlM3Machines, workerM3Machines, err := getMetal3Machines(ctx, bootstrapClient, clusterName, namespace)
		Expect(err).NotTo(HaveOccurred())

		allMachinesCount := len(controlM3Machines) + len(workerM3Machines)

		getBmhFromM3Machine := func(m3Machine v1alpha4.Metal3Machine) (result bmh.BareMetalHost) {
			Expect(bootstrapClient.Get(ctx,
				client.ObjectKey{Namespace: namespace, Name: metal3MachineToBmhName(m3Machine)},
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
		annotateBmh(ctx, bootstrapClient, workerBmh, rebootAnnotation, pointer.String(""))

		waitForVmsState([]string{vmName}, shutoff, specName)

		// Note: what is reported in the CLI as NotReady, is initially unknown status
		// This call will wait for the actual "False" condition status
		waitForNodeStatus(ctx, targetClient, client.ObjectKey{Namespace: "default", Name: nodeName}, v1.ConditionUnknown, specName)
		waitForVmsState([]string{vmName}, running, specName)
		waitForNodeStatus(ctx, targetClient, client.ObjectKey{Namespace: "default", Name: nodeName}, v1.ConditionTrue, specName)
		monitorNodesStatus(ctx, targetClient, "defaul", []string{nodeName}, v1.ConditionTrue, specName)

		// power cycle

		powerCycle := func(machines machineSetSlice) error {
			By(fmt.Sprintf("Power cycling %d machines", len(machines)))
			for _, set := range machines {
				Expect(annotateBmh(ctx, bootstrapClient, *set.baremetalhost, poweroffAnnotation, pointer.String(""))).To(Succeed())
			}
			waitForVmsState(machines.getVmNames(), shutoff, specName)

			// power on
			By("Marking a BMH for power on")
			for _, set := range machines {
				Expect(annotateBmh(ctx, bootstrapClient, *set.baremetalhost, poweroffAnnotation, nil)).To(Succeed())
			}

			// waitForVmState(vmName, running)
			waitForVmsState(machines.getVmNames(), running, specName)
			for _, nodeName := range machines.getNodeNames() {
				waitForNodeStatus(ctx, targetClient, client.ObjectKey{Namespace: "default", Name: nodeName}, v1.ConditionTrue, specName)
			}
			monitorNodesStatus(ctx, targetClient, "default", machines.getNodeNames(), v1.ConditionTrue, specName)
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

		By("Testing unhealthy annotation")
		newReplicaCount := 1
		scaleControlPlane(ctx, bootstrapClient, client.ObjectKey{Namespace: "metal3", Name: "test1"}, newReplicaCount)

		By("Waiting for 2 BMHs to be Ready")
		Eventually(
			func() error {
				bmhs := bmh.BareMetalHostList{}
				Expect(bootstrapClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
				// fmt.Printf("Looking for %s BMH among %#+v\n", bmh.StateReady, bmhs.Items)
				Expect(filterBmhsByProvisioningState(bmhs.Items, bmh.StateReady)).To(HaveLen(newReplicaCount + len(workerM3Machines)))
				return nil
			},
			e2eConfig.GetIntervals(specName, "wait-machine-remediation")...,
		).Should(Succeed())

		By("Annotating BMH as unhealthy")
		annotateBmh(ctx, bootstrapClient, workerBmh, "capi.metal3.io/unhealthy", pointer.String(""))
		defer annotateBmh(ctx, bootstrapClient, workerBmh, "capi.metal3.io/unhealthy", nil) // TODO delete before merging. This should be set as part of the test

		By("Deleting a worker machine")

		workerMachine := getMachine(client.ObjectKey{Namespace: namespace, Name: machineName})
		Expect(bootstrapClient.Delete(ctx, &workerMachine)).To(Succeed(), "Failed to delete worker Machine")

		By("Waiting for worker BMH to be in ready state")
		Eventually(
			func() error {
				Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: workerBmh.Name}, &workerBmh)).To(Succeed())
				fmt.Printf("workerBmh.status.provisioning.state: %s; expected %s\n", workerBmh.Status.Provisioning.State, bmh.StateReady)
				Expect(workerBmh.Status.Provisioning.State).To(Equal(bmh.StateReady))
				return nil
			},
			e2eConfig.GetIntervals(specName, "wait-machine-remediation")...,
		).Should(Succeed())

		By("Waiting for 2 BMHs to be Provisioned")
		Eventually(
			func() error {
				bmhs := bmh.BareMetalHostList{}
				Expect(bootstrapClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
				Expect(filterBmhsByProvisioningState(bmhs.Items, bmh.StateProvisioned)).To(HaveLen(2))
				return nil
			},
			e2eConfig.GetIntervals(specName, "wait-machine-remediation")...,
		).Should(Succeed())

		By("Scaling up machine deployment")
		Expect(scaleMachineDeployment(
			ctx, bootstrapClient, client.ObjectKey{Namespace: namespace, Name: clusterName}, 2),
		).To(Succeed())

		By("Verifying that none of the BMHs start provisioning")
		Consistently(
			func() error {
				bmhs := bmh.BareMetalHostList{}
				Expect(bootstrapClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
				Expect(filterBmhsByProvisioningState(bmhs.Items, bmh.StateProvisioned)).To(HaveLen(3))
				Expect(filterBmhsByProvisioningState(bmhs.Items, bmh.StateProvisioning)).To(HaveLen(0))
				return nil
			},
			e2eConfig.GetIntervals(specName, "monitor-provisioning")...,
		)

		By("Annotating BMH as healthy")
		annotateBmh(ctx, bootstrapClient, workerBmh, "capi.metal3.io/unhealthy", nil)

		By(fmt.Sprintf("Waiting for all-1 (%d)527 BMHs to be Provisioned", allMachinesCount-1))
		Eventually(
			func() error {

				bmhs := getAllBMH(ctx, bootstrapClient, namespace, specName)
				Expect(bootstrapClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
				Expect(filterBmhsByProvisioningState(bmhs.Items, bmh.StateProvisioned)).To(HaveLen(allMachinesCount - 1))
				return nil
			},
			e2eConfig.GetIntervals(specName, "wait-machine-remediation")...,
		)

		By("Waiting for all-1 machines to be Running")
		Eventually(
			func() error {
				machines := clusterv1.MachineList{}
				Expect(bootstrapClient.List(ctx, &machines, client.InNamespace(namespace))).To(Succeed())
				Expect(filterMachinesByPhase(machines.Items, "Running")).To(HaveLen(allMachinesCount - 1))
				return nil
			},
			e2eConfig.GetIntervals(specName, "wait-machine-remediation")...,
		)

		By("Scaling down machine deployment")
		Expect(scaleMachineDeployment(
			ctx, bootstrapClient, client.ObjectKey{Namespace: namespace, Name: clusterName}, 1),
		).To(Succeed())

		By("Waiting for 1 BMH to be Ready")
		// fails
		Eventually(
			func() error {
				bmhs := bmh.BareMetalHostList{}
				Expect(bootstrapClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
				Expect(filterBmhsByProvisioningState(bmhs.Items, bmh.StateReady)).To(HaveLen(1))
				return nil
			},
			e2eConfig.GetIntervals(specName, "wait-machine-remediation")...,
		).Should(Succeed())

		By("Testing Metal3DataTemplate reference")

		By("Creating a new Metal3DataTemplate")
		m3dataTemplate := v1alpha4.Metal3DataTemplate{}
		m3dataTemplateName := fmt.Sprintf("%s-workers-template", clusterName)
		newM3dataTemplateName := "test-new-m3dt"
		bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3dataTemplateName}, &m3dataTemplate)

		newM3DataTemplate := v1alpha4.Metal3DataTemplate{
			Spec: v1alpha4.Metal3DataTemplateSpec{
				MetaData:          m3dataTemplate.Spec.MetaData,
				NetworkData:       m3dataTemplate.Spec.NetworkData,
				ClusterName:       clusterName,
				TemplateReference: m3dataTemplateName,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      newM3dataTemplateName,
				Namespace: m3dataTemplate.Namespace,
			},
		}

		(bootstrapClient.Create(ctx, &newM3DataTemplate))
		// TODO restore before merging
		// Expect(err).NotTo(HaveOccurred())

		By("Creating a new Metal3MachineTemplate")
		m3machineTemplate := v1alpha4.Metal3MachineTemplate{}
		m3machineTemplateName := fmt.Sprintf("%s-workers", clusterName)
		bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3machineTemplateName}, &m3machineTemplate)
		newM3MachineTemplateName := "test-new-m3mt"

		// 	            spec:
		//       dataTemplate:
		//         name: "test-new-m3dt"
		//       image: "{{ m3mt.resources[0].spec.template.spec.image }}"

		newM3MachineTemplate := v1alpha4.Metal3MachineTemplate{
			Spec: v1alpha4.Metal3MachineTemplateSpec{
				Template: v1alpha4.Metal3MachineTemplateResource{
					Spec: v1alpha4.Metal3MachineSpec{
						Image: m3machineTemplate.Spec.Template.Spec.Image,
						DataTemplate: &v1.ObjectReference{
							Kind:      "Metal3DataTemplate",
							Namespace: namespace,
							Name:      newM3dataTemplateName,
						},
					},
				},
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      newM3MachineTemplateName,
				Namespace: m3machineTemplate.Namespace,
			},
		}

		bootstrapClient.Create(ctx, &newM3MachineTemplate)
		// TODO
		// Expect(bootstrapClient.Create(ctx, &newM3MachineTemplate)).To(Succeed(), "Failed to create new M3MachineTemplate")

		By("Pointing MachineDeployment to the new Metal3MachineTemplate")
		deployment := clusterv1.MachineDeployment{}
		bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: clusterName}, &deployment)

		helper, err := patch.NewHelper(&deployment, bootstrapClient)
		Expect(err).NotTo(HaveOccurred())

		deployment.Spec.Template.Spec.InfrastructureRef = v1.ObjectReference{
			Kind:      "Metal3MachineTemplate",
			Namespace: namespace,
			Name:      newM3MachineTemplateName,
		}
		deployment.Spec.Strategy.RollingUpdate.MaxUnavailable = &intstr.IntOrString{IntVal: 1}
		Expect(helper.Patch(ctx, &deployment)).To(Succeed())

		By("Waiting for 1 BMH to be Ready")
		Eventually(
			func() error {
				bmhs := bmh.BareMetalHostList{}
				Expect(bootstrapClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
				filtered := filterBmhsByProvisioningState(bmhs.Items, bmh.StateReady)
				fmt.Printf("There are %d BMHs in state %s\n", len(filtered), bmh.StateReady)
				Expect(filtered).To(HaveLen(1))
				return nil
			},
			e2eConfig.GetIntervals(specName, "wait-machine-remediation")...,
		).Should(Succeed())

		By("Waiting for 1 Metal3Data to refer to the old template")
		Eventually(
			func() error {
				datas := v1alpha4.Metal3DataList{}
				Expect(bootstrapClient.List(ctx, &datas, client.InNamespace(namespace))).To(Succeed())
				filtered := filterM3DataByReference(datas.Items, m3dataTemplateName)
				fmt.Printf("There are %d Metal3Data refering to %s\n", len(filtered), m3dataTemplateName)
				Expect(filtered).To(HaveLen(1))
				return nil
			},
			e2eConfig.GetIntervals(specName, "wait-machine-remediation")...,
		).Should(Succeed())

		scaleControlPlane(ctx, bootstrapClient, client.ObjectKey{Namespace: "metal3", Name: "test1"}, 3)

		By(fmt.Sprintf("Waiting for all-1 (%d) BMHs to be Provisioned", allMachinesCount-1))
		Eventually(
			func() error {
				bmhs := bmh.BareMetalHostList{}
				Expect(bootstrapClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
				Expect(filterBmhsByProvisioningState(bmhs.Items, bmh.StateProvisioned)).To(HaveLen(allMachinesCount))
				return nil
			},
			e2eConfig.GetIntervals(specName, "wait-machine-remediation")...,
		)

		By("Waiting for all machines to be Running")
		Eventually(
			func() error {
				machines := clusterv1.MachineList{}
				Expect(bootstrapClient.List(ctx, &machines, client.InNamespace(namespace))).To(Succeed())
				Expect(filterMachinesByPhase(machines.Items, "Running")).To(HaveLen(allMachinesCount))
				return nil
			},
			e2eConfig.GetIntervals(specName, "wait-machine-remediation")...,
		)
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

func scaleControlPlane(ctx context.Context, c client.Client, name client.ObjectKey, newReplicaCount int) {
	ctrlplane := kcp.KubeadmControlPlane{}
	Expect(c.Get(ctx, name, &ctrlplane)).To(Succeed())
	helper, err := patch.NewHelper(&ctrlplane, c)
	Expect(err).To(BeNil())

	ctrlplane.Spec.Replicas = pointer.Int32Ptr(int32(newReplicaCount))
	Expect(helper.Patch(ctx, &ctrlplane)).To(Succeed())
}

func scaleMachineDeployment(ctx context.Context, client client.Client, name client.ObjectKey, newReplicas int) error {
	deployment := clusterv1.MachineDeployment{}
	client.Get(ctx, name, &deployment)

	helper, err := patch.NewHelper(&deployment, client)
	if err != nil {
		return err
	}
	deployment.Spec.Replicas = pointer.Int32(int32(newReplicas))
	return helper.Patch(ctx, &deployment)
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

func getAllBMH(ctx context.Context, c client.Client, namespace, specName string) []bmh.BareMetalHost {
	bmhs := bmh.BareMetalHostList{}
	err := c.List(ctx, &bmhs, client.InNamespace(namespace))
	Expect(err).NotTo(HaveOccurred())
	return bmhs.Items
}

func getMetal3Machines(ctx context.Context, c client.Client, cluster, namespace string) (controlplane, workers []v1alpha4.Metal3Machine, err error) {
	allMachines := &v1alpha4.Metal3MachineList{}
	if err = c.List(ctx, allMachines, client.InNamespace(namespace)); err != nil {
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

func filterBmhsByProvisioningState(bmhs []bmh.BareMetalHost, state bmh.ProvisioningState) (result []bmh.BareMetalHost) {
	for _, bmh := range bmhs {
		if bmh.Status.Provisioning.State == state {
			result = append(result, bmh)
		}
	}
	fmt.Printf("There are %d BMHs in state '%s'\n", len(result), state)
	return
}

func filterMachinesByPhase(machines []clusterv1.Machine, phase string) (result []clusterv1.Machine) {
	for _, machine := range machines {
		if machine.Status.Phase == phase {
			result = append(result, machine)
		}
	}
	fmt.Printf("There are %d machines in phase '%s'\n", len(result), phase)
	return
}

func filterM3DataByReference(datas []v1alpha4.Metal3Data, referenceName string) (result []v1alpha4.Metal3Data) {

	for _, data := range datas {
		if data.Spec.TemplateReference == referenceName {
			result = append(result, data)
		}
	}
	fmt.Printf("There are %d Metal3Data with reference '%s'\n", len(result), referenceName)
	return
}

func waitForVmsState(vmNames []string, state vmState, specName string) {
	By(fmt.Sprintf("Waiting for VMs %#v to become '%s'", vmNames, state))
	Eventually(func() {
		vms := listVms(state)
		Expect(vms).To(ContainElements(vmNames))
	}, e2eConfig.GetIntervals(specName, "wait-vm-state")...).Should(Succeed())
}

// functions defined here to use the local variables
func monitorNodesStatus(ctx context.Context, c client.Client, namespace string, names []string, status v1.ConditionStatus, specName string) {
	// TODO look int gomega 1.14 to use assertions in the func

	By(fmt.Sprintf("Ensuring Nodes %#v consistently have ready=%s status", names, status))
	Consistently(
		func() error {
			for _, node := range names {
				if err := assertNodeStatus(ctx, c, client.ObjectKey{Namespace: namespace, Name: node}, status); err != nil {
					return err
				}
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "monitor-vm-state")...,
	).Should(Succeed())
}

func assertNodeStatus(ctx context.Context, client client.Client, name client.ObjectKey, status v1.ConditionStatus) error {
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

func waitForNodeStatus(ctx context.Context, client client.Client, name client.ObjectKey, status v1.ConditionStatus, specName string) {
	By(fmt.Sprintf("Waiting for Node '%s' to have ready=%s status", name, status))
	Eventually(
		func() error { return assertNodeStatus(ctx, client, name, status) },
		e2eConfig.GetIntervals(specName, "wait-vm-state")...,
	).Should(Succeed())
}
