package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func remediation() {
	Logf("Starting remediation tests")

	bootstrapClient := bootstrapClusterProxy.GetClient()
	targetClient := targetCluster.GetClient()

	controlplaneM3Machines, workerM3Machines := getMetal3Machines(ctx, bootstrapClient, clusterName, namespace)
	Expect(controlplaneM3Machines).To(HaveLen(numberOfControlplane))
	Expect(workerM3Machines).To(HaveLen(numberOfWorkers))

	getBmhFromM3Machine := func(m3Machine infrav1.Metal3Machine) (result bmov1alpha1.BareMetalHost) {
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

	listBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))
	listMetal3Machines(ctx, bootstrapClient, client.InNamespace(namespace))
	listMachines(ctx, bootstrapClient, client.InNamespace(namespace))
	listNodes(ctx, targetClient)

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

	listMetal3Machines(ctx, bootstrapClient, client.InNamespace(namespace))
	listMachines(ctx, bootstrapClient, client.InNamespace(namespace))
	listNodes(ctx, targetClient)

	By("Testing unhealthy and inspection annotations")
	By("Scaling down KCP to 1 replica")
	newReplicaCount := 1
	scaleKubeadmControlPlane(ctx, bootstrapClient, client.ObjectKey{Namespace: "metal3", Name: "test1"}, newReplicaCount)
	waitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, waitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  2,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-remediation"),
	})

	listMetal3Machines(ctx, bootstrapClient, client.InNamespace(namespace))
	listMachines(ctx, bootstrapClient, client.InNamespace(namespace))
	listNodes(ctx, targetClient)

	// Calling an inspection tests here for now until we have a parallelism enabled in e2e framework.
	inspection()

	listMetal3Machines(ctx, bootstrapClient, client.InNamespace(namespace))
	listMachines(ctx, bootstrapClient, client.InNamespace(namespace))
	listNodes(ctx, targetClient)

	Logf("Start checking unhealthy annotation")
	Logf("Annotating BMH as unhealthy")
	annotateBmh(ctx, bootstrapClient, workerBmh, unhealthyAnnotation, pointer.String(""))

	By("Deleting a worker machine")
	workerMachine := getMachine(ctx, bootstrapClient, client.ObjectKey{Namespace: namespace, Name: workerMachineName})
	Expect(bootstrapClient.Delete(ctx, &workerMachine)).To(Succeed(), "Failed to delete worker Machine")

	Logf("Waiting for worker BMH to be in Available state")
	Eventually(func(g Gomega) {
		g.Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: workerBmh.Name}, &workerBmh)).To(Succeed())
		g.Expect(workerBmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateAvailable))
	}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(Succeed())

	waitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, waitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  2,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-remediation"),
	})

	listMetal3Machines(ctx, bootstrapClient, client.InNamespace(namespace))
	listMachines(ctx, bootstrapClient, client.InNamespace(namespace))
	listNodes(ctx, targetClient)

	By("Scaling up machine deployment to 3 replicas")
	scaleMachineDeployment(ctx, bootstrapClient, clusterName, namespace, 3)
	waitForNumBmhInState(ctx, bmov1alpha1.StateProvisioning, waitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  1,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-remediation"),
	})

	By("Waiting for one BMH to become provisioned")
	waitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, waitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  3,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-remediation"),
	})

	Logf("Verifying that the unhealthy BMH doesn't go to provisioning")
	Consistently(func(g Gomega) {
		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(filterBmhsByProvisioningState(bmhs, bmov1alpha1.StateProvisioned)).To(HaveLen(3))
		g.Expect(filterBmhsByProvisioningState(bmhs, bmov1alpha1.StateProvisioning)).To(HaveLen(0))
	}, e2eConfig.GetIntervals(specName, "monitor-provisioning")...).Should(Succeed())

	Logf("Annotating BMH as healthy and waiting for them all to be provisioned")
	annotateBmh(ctx, bootstrapClient, workerBmh, unhealthyAnnotation, nil)
	waitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, waitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-remediation"),
	})

	By("Waiting for all Machines to be Running")
	waitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, waitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-remediation"),
	})

	By("UNHEALTHY ANNOTATION CHECK PASSED!")

	listMetal3Machines(ctx, bootstrapClient, client.InNamespace(namespace))
	listMachines(ctx, bootstrapClient, client.InNamespace(namespace))
	listNodes(ctx, targetClient)

	By("Scaling machine deployment down to 1")
	scaleMachineDeployment(ctx, bootstrapClient, clusterName, namespace, 1)
	By("Waiting for 2 old workers to deprovision")
	waitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, waitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  2,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-remediation"),
	})

	listMetal3Machines(ctx, bootstrapClient, client.InNamespace(namespace))
	listMachines(ctx, bootstrapClient, client.InNamespace(namespace))
	listNodes(ctx, targetClient)

	By("Testing Metal3DataTemplate reference")
	Logf("Creating a new Metal3DataTemplate")
	m3dataTemplate := infrav1.Metal3DataTemplate{}
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
	m3machineTemplate := infrav1.Metal3MachineTemplate{}
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

	By("Waiting for the old worker to deprovision")
	waitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, waitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  2,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-remediation"),
	})

	By("Waiting for single Metal3Data to refer to the old template")
	Eventually(func(g Gomega) {
		datas := infrav1.Metal3DataList{}
		g.Expect(bootstrapClient.List(ctx, &datas, client.InNamespace(namespace))).To(Succeed())
		filtered := filterM3DataByReference(datas.Items, m3dataTemplateName)
		g.Expect(filtered).To(HaveLen(1))
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())

	listMetal3Machines(ctx, bootstrapClient, client.InNamespace(namespace))
	listMachines(ctx, bootstrapClient, client.InNamespace(namespace))
	listNodes(ctx, targetClient)

	By("Scaling up KCP to 3 replicas")
	scaleKubeadmControlPlane(ctx, bootstrapClient, client.ObjectKey{Namespace: "metal3", Name: "test1"}, 3)
	waitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, waitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-remediation"),
	})

	Byf("Waiting for all %d machines to be Running", numberOfAllBmh)
	waitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, waitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-remediation"),
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

	cmd := exec.Command("sudo", "virsh", "list", "--name", flag)
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
