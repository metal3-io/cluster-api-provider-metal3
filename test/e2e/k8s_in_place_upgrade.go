package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type InPlaceUpgradeInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
}

func InPlaceUpgrade(ctx context.Context, inputGetter func() InPlaceUpgradeInput) {
	Logf("Starting in-place Kubernetes upgrade tests")
	input := inputGetter()
	managementClusterClient := input.BootstrapClusterProxy.GetClient()
	targetClusterClient := input.TargetCluster.GetClient()
	upgradedK8sVersion := input.E2EConfig.MustGetVariable("KUBERNETES_VERSION")
	fromK8sVersion := input.E2EConfig.MustGetVariable("KUBERNETES_VERSION_UPGRADE_FROM")
	numberOfControlplane := int(*input.E2EConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))

	Logf("FROM K8S VERSION: %v", fromK8sVersion)
	Logf("UPGRADED K8S VERSION: %v", upgradedK8sVersion)
	Logf("NUMBER OF CONTROLPLANE: %v", numberOfControlplane)

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	// Get the initial machine UIDs before upgrade to verify in-place upgrade (no rollout)
	By("Capture machine UIDs before upgrade to verify no rollout occurs")
	machineList := &clusterv1.MachineList{}
	Expect(managementClusterClient.List(ctx, machineList, client.InNamespace(input.Namespace))).To(Succeed())
	initialMachineUIDs := make(map[string]string) // map[machineName]UID
	for _, machine := range machineList.Items {
		initialMachineUIDs[machine.Name] = string(machine.UID)
		Logf("Tracking machine %s with UID %s", machine.Name, machine.UID)
	}
	Logf("Captured %d machine UIDs before upgrade", len(initialMachineUIDs))

	// Download and ensure node image is available locally
	By("Download and ensure image is available locally")
	imageURL, imageChecksum := EnsureImage(upgradedK8sVersion)

	Logf("Image URL: %s", imageURL)
	Logf("Image Checksum: %s", imageChecksum)

	// Get the cluster object
	By("Get the Cluster object")
	cluster := &clusterv1.Cluster{}
	Expect(managementClusterClient.Get(ctx, client.ObjectKey{
		Namespace: input.Namespace,
		Name:      input.ClusterName,
	}, cluster)).To(Succeed())

	By("Create new KCP Metal3MachineTemplate with upgraded image to boot")
	m3MachineTemplateName := input.ClusterName + "-controlplane"
	newM3MachineTemplateName := input.ClusterName + "-new-controlplane"
	CreateNewM3MachineTemplate(ctx, input.Namespace, newM3MachineTemplateName, m3MachineTemplateName, managementClusterClient, imageURL, imageChecksum)

	Byf("Update KCP to upgrade k8s version and binaries from %s to %s", fromK8sVersion, upgradedK8sVersion)
	kcpObj := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      managementClusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	helper, err := patch.NewHelper(kcpObj, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.MachineTemplate.Spec.InfrastructureRef.Name = newM3MachineTemplateName
	kcpObj.Spec.Version = upgradedK8sVersion
	kcpObj.Spec.Rollout.Strategy.RollingUpdate.MaxSurge.IntVal = 0
	Expect(helper.Patch(ctx, kcpObj)).To(Succeed())

	// Wait for CP nodes to be upgraded
	Byf("Wait for %d CP node(s) to be upgraded and running", numberOfControlplane)
	runningAndUpgraded := func(machine clusterv1.Machine) bool {
		running := machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning
		upgraded := machine.Spec.Version == upgradedK8sVersion
		_, isControlPlane := machine.GetLabels()[clusterv1.MachineControlPlaneLabel]
		return running && upgraded && isControlPlane
	}
	WaitForNumMachines(ctx, runningAndUpgraded, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfControlplane,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	Logf("CP nodes upgraded successfully to %s", upgradedK8sVersion)

	// Verify CP machines were not replaced (in-place upgrade)
	By("Verify CP machines were not replaced (no rollout)")
	cpMachineList := &clusterv1.MachineList{}
	Expect(managementClusterClient.List(ctx, cpMachineList,
		client.InNamespace(input.Namespace),
		client.MatchingLabels{clusterv1.MachineControlPlaneLabel: ""})).To(Succeed())
	for _, machine := range cpMachineList.Items {
		initialUID, exists := initialMachineUIDs[machine.Name]
		Expect(exists).To(BeTrue(), "CP machine %s should exist in initial machine list", machine.Name)
		Expect(string(machine.UID)).To(Equal(initialUID),
			"CP machine %s UID should not change (expected: %s, got: %s) - in-place upgrade should not replace machines",
			machine.Name, initialUID, machine.UID)
		Logf("✓ CP machine %s has same UID - confirmed in-place upgrade", machine.Name)
	}

	// Scale up control plane nodes to 5
	By("Scale up control plane nodes to 5")

	kcpObj = framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      managementClusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	helper, err = patch.NewHelper(kcpObj, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.Replicas = ptr.To[int32](5)
	Expect(helper.Patch(ctx, kcpObj)).To(Succeed())

	Logf("Control plane nodes scaled to 5 replicas")
	// Wait for all 5 CP nodes to be running with new version
	By("Wait for all 5 CP nodes to be running with new K8s version")
	WaitForNumMachines(ctx, runningAndUpgraded, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  5,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})
	Logf("All 5 control plane nodes running successfully with %s", upgradedK8sVersion)

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	By("IN-PLACE K8S UPGRADE TESTS PASSED!")
}
