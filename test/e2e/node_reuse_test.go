package e2e

import (
	"errors"
	"fmt"
	"reflect"

	bmo "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha5"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/util/taints"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	kcp "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha4"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func node_reuse() {
	var (
		targetClusterClient          = targetCluster.GetClient()
		clientSet                    = targetCluster.GetClientSet()
		kubernetesVersion            = e2eConfig.GetVariable("KUBERNETES_VERSION")
		upgradedK8sVersion           = e2eConfig.GetVariable("UPGRADED_K8S_VERSION")
		numberOfControleplaneBmh int = int(*e2eConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
		numberOfWorkerBmh        int = int(*e2eConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
		numberOfAllBmh           int = numberOfControleplaneBmh + numberOfWorkerBmh
		controlplaneTaint            = &corev1.Taint{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule}
	)
	Logf("KUBERNETES VERSION: %v", kubernetesVersion)
	Logf("UPGRADED K8S VERSION: %v", upgradedK8sVersion)
	Logf("NUMBER OF CONTROLPLANE BMH: %v", numberOfControleplaneBmh)
	Logf("NUMBER OF WORKER BMH: %v", numberOfWorkerBmh)

	By("Untaint all CP nodes before scaling down machinedeployment")
	controlplaneNodes := getControlplaneNodes(clientSet)
	untaintNodes(clientSet, controlplaneNodes, controlplaneTaint)

	By("Scale own machinedeployment to 0")
	scaleMachineDeployment(ctx, targetClusterClient, 0)

	Byf("Wait until the worker is scaled down and %d BMH(s) Available", numberOfWorkerBmh)
	Eventually(
		func() int {
			bmhs := bmo.BareMetalHostList{}
			Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			filtered := filterBmhsByProvisioningState(bmhs.Items, bmo.StateAvailable)
			return len(filtered)
		}, e2eConfig.GetIntervals(specName, "wait-bmh-available")...,
	).Should(Equal(numberOfWorkerBmh))

	By("Get the provisioned BMH names and UUIDs")
	kcpBmhBeforeUpgrade := getProvisionedBmhNamesUuids(targetClusterClient)

	By("Update Metal3MachineTemplate nodeReuse field to 'True'")
	m3machineTemplateName := fmt.Sprintf("%s-controlplane", clusterName)
	updateNodeReuse(true, m3machineTemplateName, targetClusterClient)

	Byf("Update KubeadmControlPlane k8s version to %v and maxSurge to 0", upgradedK8sVersion)
	ctrlplane := kcp.KubeadmControlPlane{}
	Expect(targetClusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: clusterName}, &ctrlplane)).To(Succeed())
	patch := []byte(fmt.Sprintf(`{
		"spec": {
			"version": "%s",
			"rolloutStrategy": {
				"rollingUpdate": {
					"maxSurge": 0
				}
			}
		}
	}`, upgradedK8sVersion))
	err := targetClusterClient.Patch(ctx, &ctrlplane, client.RawPatch(types.MergePatchType, patch))
	Expect(err).To(BeNil(), "Failed to patch KubeadmControlPlane k8s version and maxSurge fields")

	By("Check if only a single machine is in Deleting state and no other new machines are in Provisioning")
	Eventually(
		func() error {
			machines := &clusterv1.MachineList{}
			err = targetCluster.GetClient().List(ctx, machines, client.InNamespace(namespace))
			if err != nil {
				Logf("Error:  %v", err)
				return err
			}
			deleting_count := 0
			for _, machine := range machines.Items {
				Expect(machine.Status.GetTypedPhase() == clusterv1.MachinePhaseProvisioning).To(BeFalse()) // Ensure no machine is provisioning
				if machine.Status.GetTypedPhase() == clusterv1.MachinePhaseDeleting {
					deleting_count++
				}
			}
			if deleting_count != 1 {
				return errors.New("No machine is in deleting state")
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "wait-machine-deleting")...,
	).Should(Succeed())

	Byf("Wait until %d BMH is in deprovisioning state", numberOfWorkerBmh)
	deprovisioning_bmh := []bmo.BareMetalHost{}
	Eventually(
		func() int {
			bmhs := bmo.BareMetalHostList{}
			Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			deprovisioning_bmh = filterBmhsByProvisioningState(bmhs.Items, bmo.StateDeprovisioning)
			return len(deprovisioning_bmh)
		}, e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning")...,
	).Should(Equal(numberOfWorkerBmh), "Deprovisioning bmhs are not equal to %d", numberOfWorkerBmh)

	By("Wait until above deprovisioning BMH is in available state again")
	Eventually(
		func() error {
			bmhs := bmo.BareMetalHostList{}
			Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())

			bmh, err := getBmhByName(bmhs.Items, deprovisioning_bmh[0].Name)
			if err != nil {
				return err
			}
			if bmh.Status.Provisioning.State != bmo.StateAvailable {
				return fmt.Errorf("The bmh [%s] is not available yet", deprovisioning_bmh[0].Name)
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning-ready")...,
	).Should(Succeed())

	By("Check if just deprovisioned BMH re-used for the next provisioning")
	Eventually(
		func() error {
			bmhs := bmo.BareMetalHostList{}
			Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			bmh, err := getBmhByName(bmhs.Items, deprovisioning_bmh[0].Name)
			if err != nil {
				return err
			}
			if bmh.Status.Provisioning.State != bmo.StateProvisioning {
				return fmt.Errorf("The bmh [%s]  is not provisioning yet", deprovisioning_bmh[0].Name)
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "wait-bmh-available-provisioning")...,
	).Should(Succeed())

	Byf("Wait until two machines become running and updated with the new %s k8s version", upgradedK8sVersion)
	Eventually(
		func() error {
			machines := &clusterv1.MachineList{}
			err = targetClusterClient.List(ctx, machines, client.InNamespace(namespace))
			if err != nil {
				Logf("Error:  %v", err)
				return err
			}

			running_upgraded_len := 0
			for _, machine := range machines.Items {
				if machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning && *machine.Spec.Version == upgradedK8sVersion {
					running_upgraded_len++
					Logf("Machine [%v] is upgraded to k8s version (%v) and in running state", machine.Name, upgradedK8sVersion)
				}
			}
			if running_upgraded_len != 2 {
				return errors.New("Waiting for two machines to be in running state")
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "wait-machine-running")...,
	).Should(Succeed())

	By("Untaint all CP nodes after the upgrade of two controlplane nodes")
	controlplaneNodes = getControlplaneNodes(clientSet)
	untaintNodes(clientSet, controlplaneNodes, controlplaneTaint)

	By("Get the provisioned BMH names and UUIDs after upgrade")
	kcpBmhAfterUpgrade := getProvisionedBmhNamesUuids(targetClusterClient)

	By("Check difference between before and after upgrade mappings")
	equal := reflect.DeepEqual(kcpBmhBeforeUpgrade, kcpBmhAfterUpgrade)
	Expect(equal).To(BeTrue(), "The same BMHs were not reused in KubeadmControlPlane test case")

	By("Put maxSurge field in KubeadmControlPlane back to default value(1)")
	Expect(targetClusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: clusterName}, &ctrlplane)).To(Succeed())
	patch = []byte(`{
		"spec": {
			"rolloutStrategy": {
				"rollingUpdate": {
					"maxSurge": 1
				}
			}
		}
	}`)
	err = targetClusterClient.Patch(ctx, &ctrlplane, client.RawPatch(types.MergePatchType, patch))
	Expect(err).To(BeNil(), "Failed to set up KCP maxSurge to 1")

	By("Scale the controlplane down to 1")
	scaleControlPlane(ctx, targetClusterClient, client.ObjectKey{Namespace: namespace, Name: clusterName}, 1)

	By("Untaint all CP nodes")
	controlplaneNodes = getControlplaneNodes(clientSet)
	untaintNodes(clientSet, controlplaneNodes, controlplaneTaint)

	Byf("Wait until controlplane is scaled down and %d BMHs are Ready", numberOfControleplaneBmh)
	Eventually(
		func() error {
			bmhs := bmo.BareMetalHostList{}

			err = targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))
			if err != nil {
				Logf("Error: %v", err)
				return err
			}

			readyBmhs := filterBmhsByProvisioningState(bmhs.Items, bmo.StateAvailable)

			if len(readyBmhs) != numberOfControleplaneBmh {
				return fmt.Errorf("BMHs available are not equal to %d", numberOfControleplaneBmh)
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "wait-bmh-available")...,
	).Should(Succeed())

	By("Scale the worker up to 1 to start testing MachineDeployment")
	scaleMachineDeployment(ctx, targetClusterClient, 1)

	Byf("Wait until %d more bmh becomes provisioned", numberOfWorkerBmh)
	Eventually(
		func() int {
			bmhs := bmo.BareMetalHostList{}
			Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			provisionedBmhs := filterBmhsByProvisioningState(bmhs.Items, bmo.StateAvailable)
			return len(provisionedBmhs)
		}, e2eConfig.GetIntervals(specName, "wait-bmh-provisioned")...,
	).Should(Equal(2))

	Byf("Wait until %d more machine becomes running", numberOfWorkerBmh)
	Eventually(
		func() int {
			machines := &clusterv1.MachineList{}
			Expect(targetClusterClient.List(ctx, machines, client.InNamespace(namespace))).To(Succeed())

			runningMachines := filterMachinesByStatusPhase(machines.Items, clusterv1.MachinePhaseRunning)
			return len(runningMachines)
		}, e2eConfig.GetIntervals(specName, "wait-machine-running")...,
	).Should(Equal(2))

	By("Get the provisioned BMH names and UUIDs before upgrade in MachineDeployment")
	mdBmhBeforeUpgrade := getProvisionedBmhNamesUuids(targetClusterClient)

	By("Update maxSurge/maxUnavailable fields to 0/1 in MachineDeployment test case")
	machineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      targetClusterClient,
		ClusterName: clusterName,
		Namespace:   namespace,
	})
	Expect(len(machineDeployments)).To(Equal(1), "Expected exactly 1 MachineDeployment")
	machineDeploy := machineDeployments[0]

	patch = []byte(`{
		"spec": {
			"strategy": {
				"rollingUpdate": {
					"maxSurge": 0,
					"maxUnavailable": 1
				}
			}
		}
	}`)
	err = targetClusterClient.Patch(ctx, machineDeploy, client.RawPatch(types.MergePatchType, patch))
	Expect(err).To(BeNil(), "Failed to patch MachineDeployment")

	By("Update Metal3MachineTemplate nodeReuse field to 'True'")
	m3machineTemplateName = fmt.Sprintf("%s-workers", clusterName)
	updateNodeReuse(true, m3machineTemplateName, targetClusterClient)

	By("List BMHs and mark all available BMHs with unhealthy annotation")
	bmhs := bmo.BareMetalHostList{}
	Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
	for _, item := range bmhs.Items {
		if item.Status.Provisioning.State == bmo.StateAvailable {
			annotateBmh(ctx, targetClusterClient, item, "capi.metal3.io/unhealthy", pointer.String(""))
		}
	}

	Byf("Upgrade the MachineDeployment k8s version from %s to %s ", kubernetesVersion, upgradedK8sVersion)
	machineDeployments = framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      targetClusterClient,
		ClusterName: clusterName,
		Namespace:   namespace,
	})
	Expect(len(machineDeployments)).To(Equal(1), "Expected exactly 1 MachineDeployment")
	machineDeploy = machineDeployments[0]

	patch = []byte(fmt.Sprintf(`{
		"spec": {
			"template": {
				"spec": {
					"version": "%s"
				}
			}
		}
	}`, upgradedK8sVersion))
	err = targetClusterClient.Patch(ctx, machineDeploy, client.RawPatch(types.MergePatchType, patch))
	Expect(err).To(BeNil(), "Failed to patch MachineDeployment")

	Byf("Wait until %d BMH(s) in deprovisioning state", numberOfWorkerBmh)
	deprovisioning_bmh = []bmo.BareMetalHost{}
	Eventually(
		func() int {
			bmhs := bmo.BareMetalHostList{}
			Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			deprovisioning_bmh = filterBmhsByProvisioningState(bmhs.Items, bmo.StateDeprovisioning)
			return len(deprovisioning_bmh)
		},
		e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning")...,
	).Should(Equal(numberOfWorkerBmh), "Deprovisioning bmhs are not equal to %d", numberOfWorkerBmh)

	By("Wait until the above deprovisioning BMH is in available state again")
	Eventually(
		func() error {
			bmhs := bmo.BareMetalHostList{}
			Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			bmh, err := getBmhByName(bmhs.Items, deprovisioning_bmh[0].Name)
			if err != nil {
				return err
			}
			if bmh.Status.Provisioning.State != bmo.StateAvailable {
				return fmt.Errorf("The bmh [%s] is not available yet", deprovisioning_bmh[0].Name)
			}
			return nil
		},
		e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning-ready")...,
	).Should(Succeed())

	By("Unmark all the available BMHs with unhealthy annotation")
	bmhs = bmo.BareMetalHostList{}
	Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
	for _, item := range bmhs.Items {
		if item.Status.Provisioning.State == bmo.StateAvailable {
			annotateBmh(ctx, targetClusterClient, item, "capi.metal3.io/unhealthy", nil)
		}
	}

	By("Check if just deprovisioned BMH re-used for next provisioning")
	Eventually(
		func() error {
			bmhs := bmo.BareMetalHostList{}
			Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			bmh, err := getBmhByName(bmhs.Items, deprovisioning_bmh[0].Name)
			if err != nil {
				return err
			}
			if bmh.Status.Provisioning.State != bmo.StateProvisioning {
				return fmt.Errorf("The bmh [%s]  is not provisioning yet", deprovisioning_bmh[0].Name)
			}
			return nil
		},
		e2eConfig.GetIntervals(specName, "wait-bmh-available-provisioning")...,
	).Should(Succeed())

	Byf("Wait until worker machine becomes running and updated with new %s k8s version", upgradedK8sVersion)
	Eventually(
		func() int {
			machines := &clusterv1.MachineList{}
			Expect(targetClusterClient.List(ctx, machines, client.InNamespace(namespace))).To(Succeed())
			running_upgraded_len := 0
			for _, machine := range machines.Items {
				if machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning && *machine.Spec.Version == upgradedK8sVersion {
					running_upgraded_len++
					Logf("Machine [%v] is upgraded to (%v) and running", machine.Name, upgradedK8sVersion)
				}
			}
			return running_upgraded_len
		}, e2eConfig.GetIntervals(specName, "wait-machine-running")...,
	).Should(Equal(2))

	By("Get provisioned BMH names and UUIDs after upgrade in MachineDeployment")
	mdBmhAfterUpgrade := getProvisionedBmhNamesUuids(targetClusterClient)

	By("Check difference between before and after upgrade mappings in MachineDeployment")
	equal = reflect.DeepEqual(mdBmhBeforeUpgrade, mdBmhAfterUpgrade)
	Expect(equal).To(BeTrue(), "The same BMHs were not reused in MachineDeployment")

	By("Scale controlplane up to 3")
	scaleControlPlane(ctx, targetClusterClient, client.ObjectKey{Namespace: namespace, Name: clusterName}, 3)

	Byf("Wait until all %d bmhs are provisioned", numberOfAllBmh)
	Eventually(
		func() error {
			bmhs = bmo.BareMetalHostList{}
			err = targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))
			if err != nil {
				Logf("Error: %v", err)
				return err
			}
			provisionedBmh := filterBmhsByProvisioningState(bmhs.Items, bmo.StateProvisioned)

			if len(provisionedBmh) != numberOfAllBmh {
				return fmt.Errorf("Not all %d bmhs are provisioned", numberOfAllBmh)
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "wait-bmh-provisioned")...,
	).Should(Succeed())

	Byf("Wait until all %d machine(s) become(s) running", numberOfAllBmh)
	Eventually(
		func() int {
			machines := &clusterv1.MachineList{}
			Expect(targetClusterClient.List(ctx, machines, client.InNamespace(namespace))).To(Succeed())
			runningMachines := filterMachinesByStatusPhase(machines.Items, clusterv1.MachinePhaseRunning)
			return len(runningMachines)
		},
		e2eConfig.GetIntervals(specName, "wait-machine-running")...,
	).Should(Equal(numberOfAllBmh))

	By("NODE_REUSE PASSED!")
}

func getControlplaneNodes(clientSet *kubernetes.Clientset) *corev1.NodeList {
	controlplaneNodesRequirement, err := labels.NewRequirement("node-role.kubernetes.io/control-plane", selection.Exists, []string{})
	Expect(err).To(BeNil(), "Failed to set up worker Node requirements")
	controlplaneNodesSelector := labels.NewSelector().Add(*controlplaneNodesRequirement)
	controlplaneListOptions = metav1.ListOptions{LabelSelector: controlplaneNodesSelector.String()}
	controlplaneNodes, err := clientSet.CoreV1().Nodes().List(ctx, controlplaneListOptions)
	Expect(err).To(BeNil(), "Failed to get controlplane nodes")
	return controlplaneNodes
}

func getProvisionedBmhNamesUuids(clusterClient client.Client) []string {
	bmhs := bmo.BareMetalHostList{}
	var nameUuidList []string
	Expect(clusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
	for _, item := range bmhs.Items {
		if item.WasProvisioned() {
			concat := "metal3/" + item.Name + "=metal3://" + (string)(item.UID)
			nameUuidList = append(nameUuidList, concat)
		}
	}
	return nameUuidList
}

func updateNodeReuse(nodeReuse bool, m3machineTemplateName string, clusterClient client.Client) {
	m3machineTemplate := capm3.Metal3MachineTemplate{}
	Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3machineTemplateName}, &m3machineTemplate)).To(Succeed())
	helper, err := patch.NewHelper(&m3machineTemplate, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	m3machineTemplate.Spec.NodeReuse = nodeReuse
	Expect(helper.Patch(ctx, &m3machineTemplate)).To(Succeed())

	// verify that nodereuse is true
	Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3machineTemplateName}, &m3machineTemplate)).To(Succeed())
	Expect(m3machineTemplate.Spec.NodeReuse).To(BeTrue())
}

func untaintNodes(clientSet *kubernetes.Clientset, nodes *corev1.NodeList, taint *corev1.Taint) {
	for i := range nodes.Items {
		newNode, changed, err := taints.RemoveTaint(&nodes.Items[i], taint)
		Expect(err).To(BeNil(), "Failed to remove taint")
		if changed {
			_, err = clientSet.CoreV1().Nodes().Update(ctx, newNode, metav1.UpdateOptions{})
			Expect(err).To(BeNil(), "Failed to update nodes")
		}
	}
}

func filterMachinesByStatusPhase(machines []clusterv1.Machine, phase clusterv1.MachinePhase) (result []clusterv1.Machine) {
	for _, machine := range machines {
		if machine.Status.GetTypedPhase() == phase {
			result = append(result, machine)
		}
	}
	return
}

func getBmhByName(bmhs []bmo.BareMetalHost, name string) (bmo.BareMetalHost, error) {
	for _, bmh := range bmhs {
		if bmh.Name == name {
			return bmh, nil
		}
	}
	return bmo.BareMetalHost{}, errors.New("Not found")
}
