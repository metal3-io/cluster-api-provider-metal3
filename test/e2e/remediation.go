package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	rebootAnnotation    = "reboot.metal3.io"
	poweroffAnnotation  = "reboot.metal3.io/poweroff"
	unhealthyAnnotation = "capi.metal3.io/unhealthy"
	defaultNamespace    = "default"
)

type RemediationInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
	ClusterctlConfigPath  string
}

/*
 * Remediation Test
 * The remediation test focuses on verifying various annotations and actions related to remediation in the CAPM3.
 * It ensures that the cluster can recover from different failure scenarios and perform necessary actions for remediation.
 *
 * Test Steps:
 *   - 1. Reboot Annotation: This step marks a worker BareMetalHost (BMH) for reboot and waits for the associated Virtual Machine (VM) to transition to the "shutoff" state and then to the "running" state.
 *   - 2. Poweroff Annotation: The test verifies the power off and power on actions by turning off and on the specified machines.
 *   - 3. Inspection Annotation: The test runs an inspection test alongside the remediation steps. The inspection test verifies the inspection annotation functionality.
 *   - 4. Unhealthy Annotation: This step tests the unhealthy annotation by marking a BMH as unhealthy and ensuring it is not picked up for provisioning.
 *   - 5. Metal3 Data Template: The test creates a new Metal3DataTemplate (M3DT), then creates a new Metal3MachineTemplate (M3MT), and updates the MachineDeployment (MD) to point to the new M3MT. It then waits for the old worker to deprovision.
 *
 * The following code snippet represents the workflow of the remediation test:
 *
 * // Function: remediation
 *
 * 	func remediation(ctx context.Context, inputGetter func() RemediationInput) {
 * 		Logf("Starting remediation tests")
 * 		input := inputGetter()
 * 		// Step 1: Reboot Annotation
 * 		// ...
 * 		// Step 2: Poweroff Annotation
 * 		// ...
 * 		// Step 3: Inspection Annotation
 * 		// ...
 * 		// Step 4: Unhealthy Annotation
 * 		// ...
 * 		// Step 5: Metal3 Data Template
 * 		// ...
 * 		Logf("REMEDIATION TESTS PASSED!")
 * 	}
 *
 * The remediation test ensures that the CAPM3 can effectively remediate worker nodes by performing necessary actions and annotations. It helps ensure the stability and resiliency of the cluster by allowing the cluster to recover from failure scenarios and successfully restore nodes to the desired state.
 */
func remediation(ctx context.Context, inputGetter func() RemediationInput) {
	Logf("Starting remediation tests")
	input := inputGetter()
	numberOfWorkers := int(*input.E2EConfig.MustGetInt32PtrVariable("WORKER_MACHINE_COUNT"))
	numberOfControlplane := int(*input.E2EConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
	numberOfAllBmh := numberOfWorkers + numberOfControlplane

	bootstrapClient := input.BootstrapClusterProxy.GetClient()
	targetClient := input.TargetCluster.GetClient()

	controlplaneM3Machines, workerM3Machines := GetMetal3Machines(ctx, bootstrapClient, input.ClusterName, input.Namespace)
	Expect(controlplaneM3Machines).To(HaveLen(numberOfControlplane))
	Expect(workerM3Machines).To(HaveLen(numberOfWorkers))

	getBmhFromM3Machine := func(m3Machine infrav1.Metal3Machine) (result bmov1alpha1.BareMetalHost) {
		Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: Metal3MachineToBmhName(m3Machine)}, &result)).To(Succeed())
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

	workerMachineName, err := Metal3MachineToMachineName(workerM3Machine)
	Expect(err).ToNot(HaveOccurred())
	workerNodeName := workerMachineName
	vmName := BmhToVMName(workerBmh)

	ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClient)

	By("Checking that rebooted node becomes Ready")
	Logf("Marking a BMH '%s' for reboot", workerBmh.GetName())
	AnnotateBmh(ctx, bootstrapClient, workerBmh, rebootAnnotation, ptr.To(""))
	waitForVmsState([]string{vmName}, shutoff, input.SpecName, input.E2EConfig.GetIntervals(input.SpecName, "wait-vm-state")...)
	waitForVmsState([]string{vmName}, running, input.SpecName, input.E2EConfig.GetIntervals(input.SpecName, "wait-vm-state")...)
	waitForNodeStatus(ctx, targetClient, client.ObjectKey{Namespace: defaultNamespace, Name: workerNodeName}, corev1.ConditionTrue, input.SpecName, input.E2EConfig.GetIntervals(input.SpecName, "wait-vm-state")...)

	By("Power cycling worker node")
	powerCycle(ctx, bootstrapClient, targetClient, bmhToMachineSlice{{
		baremetalhost: &workerBmh,
		metal3machine: &workerM3Machine,
	}}, input.SpecName, input.E2EConfig)
	By("Power cycling 1 control plane node")
	powerCycle(ctx, bootstrapClient, targetClient, bmhsAndMachines[:1], input.SpecName, input.E2EConfig)
	By("Power cycling 2 control plane nodes")
	powerCycle(ctx, bootstrapClient, targetClient, bmhsAndMachines[1:3], input.SpecName, input.E2EConfig)

	ListMetal3Machines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClient)

	By("Testing unhealthy and inspection annotations")
	By("Scaling down KCP to 1 replica")
	newReplicaCount := int32(1)
	ScaleKubeadmControlPlane(ctx, bootstrapClient, client.ObjectKey{Namespace: "metal3", Name: "test1"}, newReplicaCount)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  2,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	ListMetal3Machines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClient)

	// Calling an inspection tests here for now until we have a parallelism enabled in e2e framework.
	inspection(ctx, func() InspectionInput {
		return InspectionInput{
			E2EConfig:             input.E2EConfig,
			BootstrapClusterProxy: input.BootstrapClusterProxy,
			SpecName:              input.SpecName,
			Namespace:             input.Namespace,
			ClusterctlConfigPath:  input.ClusterctlConfigPath,
		}
	})

	ListMetal3Machines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClient)

	Logf("Start checking unhealthy annotation")
	Logf("Annotating BMH as unhealthy")
	AnnotateBmh(ctx, bootstrapClient, workerBmh, unhealthyAnnotation, ptr.To(""))

	By("Deleting a worker machine")
	workerMachine := GetMachine(ctx, bootstrapClient, client.ObjectKey{Namespace: input.Namespace, Name: workerMachineName})
	Expect(bootstrapClient.Delete(ctx, &workerMachine)).To(Succeed(), "Failed to delete worker Machine")

	Logf("Waiting for worker BMH to be in Available state")
	Eventually(func(g Gomega) {
		g.Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: workerBmh.Name}, &workerBmh)).To(Succeed())
		g.Expect(workerBmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateAvailable))
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation")...).Should(Succeed())

	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  2,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	ListMetal3Machines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClient)

	By("Scaling up machine deployment to 3 replicas")
	ScaleMachineDeployment(ctx, bootstrapClient, input.ClusterName, input.Namespace, 3)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioning, WaitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  1,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	By("Waiting for one BMH to become provisioned")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  3,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	Logf("Verifying that the unhealthy BMH doesn't go to provisioning")
	Consistently(func(g Gomega) {
		bmhs, err := GetAllBmhs(ctx, bootstrapClient, input.Namespace)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(FilterBmhsByProvisioningState(bmhs, bmov1alpha1.StateProvisioned)).To(HaveLen(3))
		g.Expect(FilterBmhsByProvisioningState(bmhs, bmov1alpha1.StateProvisioning)).To(BeEmpty())
	}, input.E2EConfig.GetIntervals(input.SpecName, "monitor-provisioning")...).Should(Succeed())

	Logf("Annotating BMH as healthy and waiting for them all to be provisioned")
	AnnotateBmh(ctx, bootstrapClient, workerBmh, unhealthyAnnotation, nil)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	By("Waiting for all Machines to be Running")
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	By("UNHEALTHY ANNOTATION CHECK PASSED!")

	ListMetal3Machines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClient)

	By("Scaling machine deployment down to 1")
	ScaleMachineDeployment(ctx, bootstrapClient, input.ClusterName, input.Namespace, 1)
	By("Waiting for 2 old workers to deprovision")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  2,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	ListMetal3Machines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClient)

	By("Testing Metal3DataTemplate reference")
	Logf("Creating a new Metal3DataTemplate")
	m3dataTemplate := infrav1.Metal3DataTemplate{}
	m3dataTemplateName := input.ClusterName + "-workers-template"
	newM3dataTemplateName := "test-new-m3dt"
	Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: m3dataTemplateName}, &m3dataTemplate)).To(Succeed())

	newM3DataTemplate := m3dataTemplate.DeepCopy()
	cleanObjectMeta(&newM3DataTemplate.ObjectMeta)

	newM3DataTemplate.Spec.MetaData = m3dataTemplate.Spec.MetaData
	newM3DataTemplate.Spec.NetworkData = m3dataTemplate.Spec.NetworkData
	newM3DataTemplate.Spec.ClusterName = input.ClusterName
	newM3DataTemplate.Spec.TemplateReference = m3dataTemplateName

	newM3DataTemplate.ObjectMeta.Name = newM3dataTemplateName
	newM3DataTemplate.ObjectMeta.Namespace = m3dataTemplate.Namespace

	err = bootstrapClient.Create(ctx, newM3DataTemplate)
	Expect(err).NotTo(HaveOccurred())

	By("Creating a new Metal3MachineTemplate")
	m3machineTemplate := infrav1.Metal3MachineTemplate{}
	m3machineTemplateName := input.ClusterName + "-workers"
	Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: m3machineTemplateName}, &m3machineTemplate)).To(Succeed())
	newM3MachineTemplateName := "test-new-m3mt"

	newM3MachineTemplate := m3machineTemplate.DeepCopy()
	cleanObjectMeta(&newM3MachineTemplate.ObjectMeta)

	newM3MachineTemplate.Spec.Template.Spec.Image = m3machineTemplate.Spec.Template.Spec.Image
	newM3MachineTemplate.Spec.Template.Spec.DataTemplate.Name = newM3dataTemplateName
	newM3MachineTemplate.ObjectMeta.Name = newM3MachineTemplateName

	Expect(bootstrapClient.Create(ctx, newM3MachineTemplate)).To(Succeed(), "Failed to create new Metal3MachineTemplate")

	By("Pointing MachineDeployment to the new Metal3MachineTemplate")
	deployment := clusterv1.MachineDeployment{}
	Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName}, &deployment)).To(Succeed())

	helper, err := patch.NewHelper(&deployment, bootstrapClient)
	Expect(err).NotTo(HaveOccurred())

	deployment.Spec.Template.Spec.InfrastructureRef = corev1.ObjectReference{
		Kind:       "Metal3MachineTemplate",
		APIVersion: input.E2EConfig.MustGetVariable("APIVersion"),
		Name:       newM3MachineTemplateName,
	}
	deployment.Spec.Strategy.RollingUpdate.MaxUnavailable = &intstr.IntOrString{IntVal: 1}
	Expect(helper.Patch(ctx, &deployment)).To(Succeed())

	By("Waiting for the old worker to deprovision")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  2,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	By("Waiting for single Metal3Data to refer to the old template")
	Eventually(func(g Gomega) {
		datas := infrav1.Metal3DataList{}
		g.Expect(bootstrapClient.List(ctx, &datas, client.InNamespace(input.Namespace))).To(Succeed())
		filtered := filterM3DataByReference(datas.Items, m3dataTemplateName)
		g.Expect(filtered).To(HaveLen(1))
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-deployment")...).Should(Succeed())

	ListMetal3Machines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, bootstrapClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClient)

	By("Scaling up KCP to 3 replicas")
	ScaleKubeadmControlPlane(ctx, bootstrapClient, client.ObjectKey{Namespace: "metal3", Name: "test1"}, 3)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	Byf("Waiting for all %d machines to be Running", numberOfAllBmh)
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	By("REMEDIATION TESTS PASSED!")
}

type bmhToMachine struct {
	baremetalhost *bmov1alpha1.BareMetalHost
	metal3machine *infrav1.Metal3Machine
}
type bmhToMachineSlice []bmhToMachine

func (btm bmhToMachine) String() string {
	return fmt.Sprintf("machineSet{baremetalhost: %s, metal3machine: %s}",
		btm.baremetalhost.GetName(),
		btm.metal3machine.GetName(),
	)
}

func (btms bmhToMachineSlice) getBMHs() (hosts []bmov1alpha1.BareMetalHost) {
	for _, ms := range btms {
		if ms.baremetalhost != nil {
			hosts = append(hosts, *ms.baremetalhost)
		}
	}
	return
}

func (btms bmhToMachineSlice) getVMNames() (names []string) {
	for _, host := range btms.getBMHs() {
		names = append(names, BmhToVMName(host))
	}
	return
}

func (btms bmhToMachineSlice) GetMachineNames() (names []string) {
	for _, ms := range btms {
		if ms.metal3machine != nil {
			name, err := Metal3MachineToMachineName(*ms.metal3machine)
			Expect(err).NotTo(HaveOccurred())
			names = append(names, name)
		}
	}
	return
}

func (btms bmhToMachineSlice) getNodeNames() []string {
	// nodes have the same names as machines
	return btms.GetMachineNames()
}

// listVms returns the names of libvirt VMs having given state.
func listVms(state vmState) []string {
	var cmd *exec.Cmd // gosec Subprocess launched with variable
	switch state {
	case running:
		cmd = exec.Command("sudo", "virsh", "list", "--name", "--state-running")
	case shutoff:
		cmd = exec.Command("sudo", "virsh", "list", "--name", "--state-shutoff")
	case paused:
		cmd = exec.Command("sudo", "virsh", "list", "--name", "--state-paused")
	case other:
		cmd = exec.Command("sudo", "virsh", "list", "--name", "--state-other")
	}

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

func filterM3DataByReference(datas []infrav1.Metal3Data, referenceName string) (result []infrav1.Metal3Data) {
	for _, data := range datas {
		if data.Spec.TemplateReference == referenceName {
			result = append(result, data)
		}
	}
	return
}

func waitForVmsState(vmNames []string, state vmState, _ string, interval ...interface{}) {
	Byf("Waiting for VMs %v to become '%s'", vmNames, state)
	Eventually(func() []string {
		return listVms(state)
	}, interval...).Should(ContainElements(vmNames))
}

func monitorNodesStatus(ctx context.Context, g Gomega, c client.Client, namespace string, names []string, status corev1.ConditionStatus, _ string, interval ...interface{}) {
	Byf("Ensuring Nodes %v consistently have ready=%s status", names, status)
	g.Consistently(
		func() error {
			for _, node := range names {
				if err := assertNodeStatus(ctx, c, client.ObjectKey{Namespace: namespace, Name: node}, status); err != nil {
					return err
				}
			}
			return nil
		}, interval...).Should(Succeed())
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
			return fmt.Errorf("node %s has status '%s', should have '%s'", name.Name, condition.Status, status)
		}
	}
	return fmt.Errorf("node %s missing condition \"Ready\"", name.Name)
}

func waitForNodeStatus(ctx context.Context, client client.Client, name client.ObjectKey, status corev1.ConditionStatus, _ string, interval ...interface{}) {
	Byf("Waiting for Node '%s' to have ready=%s status", name, status)
	Eventually(
		func() error { return assertNodeStatus(ctx, client, name, status) },
		interval...,
	).Should(Succeed())
}

// powerCycle tests the poweroff annotation be turning given machines off and on.
func powerCycle(ctx context.Context, c client.Client, workloadClient client.Client, machines bmhToMachineSlice, specName string, e2eConfig *clusterctl.E2EConfig) {
	Byf("Power cycling %d machines", len(machines))

	Logf("Marking %d BMHs for power off", len(machines))
	for _, set := range machines {
		AnnotateBmh(ctx, c, *set.baremetalhost, poweroffAnnotation, ptr.To(""))
	}
	waitForVmsState(machines.getVMNames(), shutoff, specName, e2eConfig.GetIntervals(specName, "wait-vm-state")...)

	// power on
	Logf("Marking %d BMHs for power on", len(machines))
	for _, set := range machines {
		AnnotateBmh(ctx, c, *set.baremetalhost, poweroffAnnotation, nil)
	}

	waitForVmsState(machines.getVMNames(), running, specName, e2eConfig.GetIntervals(specName, "wait-vm-state")...)
	for _, nodeName := range machines.getNodeNames() {
		waitForNodeStatus(ctx, workloadClient, client.ObjectKey{Namespace: defaultNamespace, Name: nodeName}, corev1.ConditionTrue, specName, e2eConfig.GetIntervals(specName, "wait-vm-state")...)
	}

	By("waiting for nodes to consistently have Ready status")
	Eventually(func(g Gomega) {
		monitorNodesStatus(ctx, g, workloadClient, defaultNamespace, machines.getNodeNames(), corev1.ConditionTrue, specName, e2eConfig.GetIntervals(specName, "monitor-vm-state")...)
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
