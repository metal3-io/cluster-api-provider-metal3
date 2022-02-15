package e2e

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bmo "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
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

const (
	rebootAnnotation    = "reboot.metal3.io"
	poweroffAnnotation  = "reboot.metal3.io/poweroff"
	unhealthyAnnotation = "capi.metal3.io/unhealthy"
)

const defaultNamespace = "default"

func remediation() {
	Logf("Starting remediation tests")

	bootstrapClient := bootstrapClusterProxy.GetClient()
	targetClient := targetCluster.GetClient()
	allMachinesCount := int(controlPlaneMachineCount + workerMachineCount)

	controlplaneM3Machines, workerM3Machines := getMetal3Machines(ctx, bootstrapClient, clusterName, namespace)
	Expect(controlplaneM3Machines).To(HaveLen(int(controlPlaneMachineCount)))
	Expect(workerM3Machines).To(HaveLen(int(workerMachineCount)))

	getBmhFromM3Machine := func(m3Machine capm3.Metal3Machine) (result bmo.BareMetalHost) {
		Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: metal3MachineToBmhName(m3Machine)}, &result)).To(Succeed())
		return result
	}

	bmhsAndMachines := make([]bmhToMachine, len(controlplaneM3Machines))
	for i, m3machine := range controlplaneM3Machines {
		theBmh := getBmhFromM3Machine(m3machine)
		bmhsAndMachines[i] = bmhToMachine{
			baremetalhost: &theBmh,
			metal3machine: &controlplaneM3Machines[i],
		}
	}

	workerM3Machine := workerM3Machines[0]
	workerBmh := getBmhFromM3Machine(workerM3Machine)

	workerMachineName, err := metal3MachineToMachineName(workerM3Machine)
	Expect(err).ToNot(HaveOccurred())
	workerNodeName := workerMachineName
	vmName := bmhToVMName(workerBmh)

	By("Checking that rebooted node becomes Ready")
	Logf("Marking a BMH '%s' for reboot", workerBmh.GetName())
	annotateBmh(ctx, bootstrapClient, workerBmh, rebootAnnotation, pointer.String(""))
	waitForVmsState([]string{vmName}, shutoff, specName)
	waitForVmsState([]string{vmName}, running, specName)
	waitForNodeStatus(ctx, targetClient, client.ObjectKey{Namespace: defaultNamespace, Name: workerNodeName}, corev1.ConditionTrue, specName)

	By("Power cycling worker node")
	powerCycle(ctx, bootstrapClient, targetClient, bmhToMachineSlice{{
		baremetalhost: &workerBmh,
		metal3machine: &workerM3Machine,
	}}, specName)
	By("Power cycling 1 control plane node")
	powerCycle(ctx, bootstrapClient, targetClient, bmhsAndMachines[:1], specName)
	By("Power cycling 2 control plane nodes")
	powerCycle(ctx, bootstrapClient, targetClient, bmhsAndMachines[1:3], specName)

	By("Testing unhealthy and inspection annotations")
	newReplicaCount := 1
	scaleKubeadmControlPlane(ctx, bootstrapClient, client.ObjectKey{Namespace: "metal3", Name: "test1"}, newReplicaCount)

	Logf("Waiting for 2 BMHs to be in Available state")
	Eventually(func(g Gomega) error {
		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		if err != nil {
			Logf("Error: %v", err)
			return err
		}
		g.Expect(filterBmhsByProvisioningState(bmhs, bmo.StateAvailable)).To(HaveLen(newReplicaCount + len(workerM3Machines)))
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(Succeed())

	// Calling an inspection tests here for now until we have a parallelism enabled in e2e framework.
	inspection()

	Logf("Start checking unhealthy annotation")
	Logf("Annotating BMH as unhealthy")
	annotateBmh(ctx, bootstrapClient, workerBmh, unhealthyAnnotation, pointer.String(""))

	By("Deleting a worker machine")
	workerMachine := getMachine(ctx, bootstrapClient, client.ObjectKey{Namespace: namespace, Name: workerMachineName})
	Expect(bootstrapClient.Delete(ctx, &workerMachine)).To(Succeed(), "Failed to delete worker Machine")

	Logf("Waiting for worker BMH to be in Available state")
	Eventually(func(g Gomega) error {
		g.Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: workerBmh.Name}, &workerBmh)).To(Succeed())
		g.Expect(workerBmh.Status.Provisioning.State).To(Equal(bmo.StateAvailable))
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(Succeed())

	Logf("Waiting for 2 BMHs to be Provisioned")
	Eventually(func(g Gomega) error {
		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		if err != nil {
			Logf("Error: %v", err)
			return err
		}
		provisioningBMHs := filterBmhsByProvisioningState(bmhs, bmo.StateProvisioned)
		if len(provisioningBMHs) != 2 {
			return errors.New("Waiting for 2 BMHs to be Provisioned")
		}
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(Succeed())

	By("Scaling up machine deployment to 3 replicas")
	scaleMachineDeployment(ctx, bootstrapClient, clusterName, namespace, 3)

	By("Waiting for one BMH to start provisioning")
	Eventually(func(g Gomega) error {
		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		if err != nil {
			Logf("Error: %v", err)
			return err
		}
		g.Expect(filterBmhsByProvisioningState(bmhs, bmo.StateProvisioning)).To(HaveLen(1))
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...)

	Logf("Verifying that only one BMH starts provisioning")
	Consistently(func() error {
		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		if err != nil {
			Logf("Error: %v", err)
			return err
		}
		Expect(filterBmhsByProvisioningState(bmhs, bmo.StateProvisioned)).To(HaveLen(3))
		Expect(filterBmhsByProvisioningState(bmhs, bmo.StateProvisioning)).To(HaveLen(1))
		return nil
	}, e2eConfig.GetIntervals(specName, "monitor-provisioning")...)

	Logf("Annotating BMH as healthy")
	annotateBmh(ctx, bootstrapClient, workerBmh, unhealthyAnnotation, nil)

	Byf("Waiting for all (%d) BMHs to be Provisioned", allMachinesCount)
	Eventually(func(g Gomega) error {
		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		if err != nil {
			Logf("Error: %v", err)
			return err
		}
		g.Expect(filterBmhsByProvisioningState(bmhs, bmo.StateProvisioned)).To(HaveLen(allMachinesCount))
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...)

	By("Waiting for all Machines to be Running")
	Eventually(func(g Gomega) error {
		machines := clusterv1.MachineList{}
		g.Expect(bootstrapClient.List(ctx, &machines, client.InNamespace(namespace))).To(Succeed())
		g.Expect(filterMachinesByPhase(machines.Items, "Running")).To(HaveLen(allMachinesCount))
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...)

	By("UNHEALTHY ANNOTATION CHECK PASSED!")

	By("Scaling machine deployment down to 1")
	scaleMachineDeployment(ctx, bootstrapClient, clusterName, namespace, 1)

	By("Waiting for 2 BMHs to be in Available state")
	Eventually(func(g Gomega) error {
		bmhs := bmo.BareMetalHostList{}
		g.Expect(bootstrapClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
		g.Expect(filterBmhsByProvisioningState(bmhs.Items, bmo.StateAvailable)).To(HaveLen(2))
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(Succeed())

	By("Testing Metal3DataTemplate reference")
	Logf("Creating a new Metal3DataTemplate")
	m3dataTemplate := capm3.Metal3DataTemplate{}
	m3dataTemplateName := fmt.Sprintf("%s-workers-template", clusterName)
	newM3dataTemplateName := "test-new-m3dt"
	Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3dataTemplateName}, &m3dataTemplate)).To(Succeed())

	newM3DataTemplate := m3dataTemplate.DeepCopy()
	cleanObjectMeta(&newM3DataTemplate.ObjectMeta)

	newM3DataTemplate.Spec.MetaData = m3dataTemplate.Spec.MetaData
	newM3DataTemplate.Spec.NetworkData = m3dataTemplate.Spec.NetworkData
	newM3DataTemplate.Spec.ClusterName = clusterName
	newM3DataTemplate.Spec.TemplateReference = m3dataTemplateName

	newM3DataTemplate.ObjectMeta.Name = newM3dataTemplateName
	newM3DataTemplate.ObjectMeta.Namespace = m3dataTemplate.Namespace

	err = bootstrapClient.Create(ctx, newM3DataTemplate)
	Expect(err).NotTo(HaveOccurred())

	By("Creating a new Metal3MachineTemplate")
	m3machineTemplate := capm3.Metal3MachineTemplate{}
	m3machineTemplateName := fmt.Sprintf("%s-workers", clusterName)
	Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3machineTemplateName}, &m3machineTemplate)).To(Succeed())
	newM3MachineTemplateName := "test-new-m3mt"

	newM3MachineTemplate := m3machineTemplate.DeepCopy()
	cleanObjectMeta(&newM3MachineTemplate.ObjectMeta)

	newM3MachineTemplate.Spec.Template.Spec.Image = m3machineTemplate.Spec.Template.Spec.Image
	newM3MachineTemplate.Spec.Template.Spec.DataTemplate.Name = newM3dataTemplateName
	newM3MachineTemplate.ObjectMeta.Name = newM3MachineTemplateName

	Expect(bootstrapClient.Create(ctx, newM3MachineTemplate)).To(Succeed(), "Failed to create new Metal3MachineTemplate")

	By("Pointing MachineDeployment to the new Metal3MachineTemplate")
	deployment := clusterv1.MachineDeployment{}
	Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: clusterName}, &deployment)).To(Succeed())

	helper, err := patch.NewHelper(&deployment, bootstrapClient)
	Expect(err).NotTo(HaveOccurred())

	deployment.Spec.Template.Spec.InfrastructureRef = corev1.ObjectReference{
		Kind:       "Metal3MachineTemplate",
		APIVersion: e2eConfig.GetVariable("APIVersion"),
		Name:       newM3MachineTemplateName,
	}
	deployment.Spec.Strategy.RollingUpdate.MaxUnavailable = &intstr.IntOrString{IntVal: 1}
	Expect(helper.Patch(ctx, &deployment)).To(Succeed())

	By("Waiting for 2 BMHs to be in Available state")
	Eventually(func(g Gomega) error {
		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		if err != nil {
			Logf("Error: %v", err)
			return err
		}
		filtered := filterBmhsByProvisioningState(bmhs, bmo.StateAvailable)
		Logf("There are %d BMHs in state %s", len(filtered), bmo.StateAvailable)
		g.Expect(filtered).To(HaveLen(2))
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(Succeed())

	By("Waiting for single Metal3Data to refer to the old template")
	Eventually(func(g Gomega) {
		datas := capm3.Metal3DataList{}
		g.Expect(bootstrapClient.List(ctx, &datas, client.InNamespace(namespace))).To(Succeed())
		filtered := filterM3DataByReference(datas.Items, m3dataTemplateName)
		g.Expect(filtered).To(HaveLen(1))
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())

	By("Scaling up KCP to 3 replicas")
	scaleKubeadmControlPlane(ctx, bootstrapClient, client.ObjectKey{Namespace: "metal3", Name: "test1"}, 3)

	Byf("Waiting for all %d BMHs to be Provisioned", allMachinesCount)
	Eventually(func(g Gomega) {
		bmhs := bmo.BareMetalHostList{}
		g.Expect(bootstrapClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
		g.Expect(filterBmhsByProvisioningState(bmhs.Items, bmo.StateProvisioned)).To(HaveLen(allMachinesCount))
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(Succeed())

	Byf("Waiting for all %d machines to be Running", allMachinesCount)
	Eventually(func(g Gomega) {
		machines := clusterv1.MachineList{}
		g.Expect(bootstrapClient.List(ctx, &machines, client.InNamespace(namespace))).To(Succeed())
		g.Expect(filterMachinesByPhase(machines.Items, "Running")).To(HaveLen(allMachinesCount))
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(Succeed())

	By("REMEDIATION TESTS PASSED!")
}

type bmhToMachine struct {
	baremetalhost *bmo.BareMetalHost
	metal3machine *capm3.Metal3Machine
}
type bmhToMachineSlice []bmhToMachine

func (btm bmhToMachine) String() string {
	return fmt.Sprintf("machineSet{baremetalhost: %s, metal3machine: %s}",
		btm.baremetalhost.GetName(),
		btm.metal3machine.GetName(),
	)
}

func (btms bmhToMachineSlice) getBMHs() (hosts []bmo.BareMetalHost) {
	for _, ms := range btms {
		if ms.baremetalhost != nil {
			hosts = append(hosts, *ms.baremetalhost)
		}
	}
	return
}

func (btms bmhToMachineSlice) getVMNames() (names []string) {
	for _, host := range btms.getBMHs() {
		names = append(names, bmhToVMName(host))
	}
	return
}

func (btms bmhToMachineSlice) getMachineNames() (names []string) {
	for _, ms := range btms {
		if ms.metal3machine != nil {
			name, err := metal3MachineToMachineName(*ms.metal3machine)
			Expect(err).NotTo(HaveOccurred())
			names = append(names, name)
		}
	}
	return
}

func (btms bmhToMachineSlice) getNodeNames() []string {
	// nodes have the same names as machines
	return btms.getMachineNames()
}

// metal3MachineToMachineName finds the releveant owner reference in Metal3Machine
// and returns the name of corresponding Machine.
func metal3MachineToMachineName(m3machine capm3.Metal3Machine) (string, error) {
	ownerReferences := m3machine.GetOwnerReferences()
	for _, reference := range ownerReferences {
		if reference.Kind == "Machine" {
			return reference.Name, nil
		}
	}
	return "", fmt.Errorf("metal3machine missing a \"Machine\" kind owner reference")
}

func metal3MachineToBmhName(m3machine capm3.Metal3Machine) string {
	return strings.Replace(m3machine.GetAnnotations()["metal3.io/BareMetalHost"], "metal3/", "", 1)
}

// Derives the name of a VM created by metal3-dev-env from the name of a BareMetalHost object.
func bmhToVMName(host bmo.BareMetalHost) string {
	return strings.ReplaceAll(host.Name, "-", "_")
}

// listVms returns the names of libvirt VMs having given state.
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

	lines := strings.Split(string(result), "\n")
	// virsh may return some empty lines which need to be removed
	i := 0
	for _, line := range lines {
		if line != "" {
			lines[i] = line
			i++
		}
	}
	return lines[:i]
}

func getAllBmhs(ctx context.Context, c client.Client, namespace, specName string) ([]bmo.BareMetalHost, error) {
	bmhs := bmo.BareMetalHostList{}
	err := c.List(ctx, &bmhs, client.InNamespace(namespace))
	return bmhs.Items, err
}

func getMetal3Machines(ctx context.Context, c client.Client, cluster, namespace string) ([]capm3.Metal3Machine, []capm3.Metal3Machine) {
	var controlplane, workers []capm3.Metal3Machine
	allMachines := &capm3.Metal3MachineList{}
	Expect(c.List(ctx, allMachines, client.InNamespace(namespace))).To(Succeed())

	for _, machine := range allMachines.Items {
		if strings.Contains(machine.ObjectMeta.Name, "workers") {
			workers = append(workers, machine)
		} else {
			controlplane = append(controlplane, machine)
		}
	}

	return controlplane, workers
}

func filterM3DataByReference(datas []capm3.Metal3Data, referenceName string) (result []capm3.Metal3Data) {
	for _, data := range datas {
		if data.Spec.TemplateReference == referenceName {
			result = append(result, data)
		}
	}
	return
}

func waitForVmsState(vmNames []string, state vmState, specName string) {
	Byf("Waiting for VMs %v to become '%s'", vmNames, state)
	Eventually(func() []string {
		return listVms(state)
	}, e2eConfig.GetIntervals(specName, "wait-vm-state")...).Should(ContainElements(vmNames))
}

func monitorNodesStatus(ctx context.Context, g Gomega, c client.Client, namespace string, names []string, status corev1.ConditionStatus, specName string) {
	Byf("Ensuring Nodes %v consistently have ready=%s status", names, status)
	g.Consistently(
		func() error {
			for _, node := range names {
				if err := assertNodeStatus(ctx, c, client.ObjectKey{Namespace: namespace, Name: node}, status); err != nil {
					return err
				}
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "monitor-vm-state")...).Should(Succeed())
}

func assertNodeStatus(ctx context.Context, client client.Client, name client.ObjectKey, status corev1.ConditionStatus) error {
	node := &corev1.Node{}
	if err := client.Get(ctx, name, node); err != nil {
		return err
	}
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			if status == condition.Status {
				return nil
			}
			return fmt.Errorf("Node %s has status '%s', should have '%s'", name.Name, condition.Status, status)
		}
	}
	return fmt.Errorf("Node %s missing condition \"Ready\"", name.Name)
}

func waitForNodeStatus(ctx context.Context, client client.Client, name client.ObjectKey, status corev1.ConditionStatus, specName string) {
	Byf("Waiting for Node '%s' to have ready=%s status", name, status)
	Eventually(
		func() error { return assertNodeStatus(ctx, client, name, status) },
		e2eConfig.GetIntervals(specName, "wait-vm-state")...,
	).Should(Succeed())
}

// powerCycle tests the poweroff annotation be turning given machines off and on.
func powerCycle(ctx context.Context, c client.Client, workloadClient client.Client, machines bmhToMachineSlice, specName string) {
	Byf("Power cycling %d machines", len(machines))

	Logf("Marking %d BMHs for power off", len(machines))
	for _, set := range machines {
		annotateBmh(ctx, c, *set.baremetalhost, poweroffAnnotation, pointer.String(""))
	}
	waitForVmsState(machines.getVMNames(), shutoff, specName)

	// power on
	Logf("Marking %d BMHs for power on", len(machines))
	for _, set := range machines {
		annotateBmh(ctx, c, *set.baremetalhost, poweroffAnnotation, nil)
	}

	waitForVmsState(machines.getVMNames(), running, specName)
	for _, nodeName := range machines.getNodeNames() {
		waitForNodeStatus(ctx, workloadClient, client.ObjectKey{Namespace: defaultNamespace, Name: nodeName}, corev1.ConditionTrue, specName)
	}

	By("waiting for nodes to consistently have Ready status")
	Eventually(func(g Gomega) {
		monitorNodesStatus(ctx, g, workloadClient, defaultNamespace, machines.getNodeNames(), corev1.ConditionTrue, specName)
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(Succeed())
}

// cleanObjectMeta clears object meta after copying from the original object.
func cleanObjectMeta(om *metav1.ObjectMeta) {
	om.UID = ""
	om.Finalizers = nil
	om.ManagedFields = nil
	om.ResourceVersion = ""
	om.OwnerReferences = nil
}

// Get the machine object given its object name.
func getMachine(ctx context.Context, c client.Client, name client.ObjectKey) (result clusterv1.Machine) {
	Expect(c.Get(ctx, name, &result)).To(Succeed())
	return
}
