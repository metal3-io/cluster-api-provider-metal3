package e2e

import (
	"context"
	"path/filepath"
	"reflect"
	"time"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1beta1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeReuseInput struct {
	E2EConfig      *clusterctl.E2EConfig
	ClusterProxy   framework.ClusterProxy
	SpecName       string
	ClusterName    string
	Namespace      string
	ArtifactFolder string
}

// NodeReuse verifies the feature of reusing the same node after upgrading kcp/md nodes.
func NodeReuse(ctx context.Context, inputGetter func() NodeReuseInput) {
	Logf("Starting node reuse tests [node_reuse]")
	input := inputGetter()
	clusterClient := input.ClusterProxy.GetClient()
	clientSet := input.ClusterProxy.GetClientSet()
	fromK8sVersion := input.E2EConfig.MustGetVariable("KUBERNETES_PATCH_FROM_VERSION")
	toK8sVersion := input.E2EConfig.MustGetVariable("KUBERNETES_PATCH_TO_VERSION")
	numberOfWorkers := int(*input.E2EConfig.MustGetInt32PtrVariable("WORKER_MACHINE_COUNT"))
	numberOfControlplane := int(*input.E2EConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
	numberOfAllBmh := numberOfWorkers + numberOfControlplane

	var (
		controlplaneTaints = []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule}}
		emptyTaint = []corev1.Taint{}
	)

	const (
		artifactoryURL = "https://artifactory.nordix.org/artifactory/metal3/images/k8s"
		imagesURL      = "http://172.22.0.1/images"
		ironicImageDir = "/opt/metal3-dev-env/ironic/html/images"
		nodeReuseLabel = "infrastructure.cluster.x-k8s.io/node-reuse"
	)

	Logf("KUBERNETES VERSION: %v", fromK8sVersion)
	Logf("UPGRADED K8S VERSION: %v", toK8sVersion)
	Logf("NUMBER OF CONTROLPLANE BMH: %v", numberOfControlplane)
	Logf("NUMBER OF WORKER BMH: %v", numberOfWorkers)

	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, clusterClient)

	By("Untaint all CP nodes before scaling down machinedeployment [node_reuse]")
	for controlplaneNodesPresent := 0; controlplaneNodesPresent < numberOfControlplane; {
		for untaintedNodeCount := 1; untaintedNodeCount > 0; {
			controlplaneNodes := getControlplaneNodes(ctx, clientSet)
			controlplaneNodesPresent = len(controlplaneNodes.Items)
			untaintedNodeCount = untaintNodes(ctx, clusterClient, controlplaneNodes, controlplaneTaints)
			time.Sleep(10 * time.Second)
		}
	}

	By("Scale down MachineDeployment to 0 [node_reuse]")
	// this removes the worker node(s)
	ScaleMachineDeployment(ctx, clusterClient, input.ClusterName, input.Namespace, 0)
	numberOfWorkers = 0

	Byf("Wait until the worker is scaled down and %d BMH(s) Available [node_reuse]", numberOfAllBmh-numberOfControlplane-numberOfWorkers)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh - numberOfControlplane - numberOfWorkers,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-available"),
	})

	By("Wait for all pods to be running after workers were scaled down to 0 [node_reuse]")
	// Give time for the pods to settle before starting the upgrade process
	framework.WaitForPodListCondition(ctx, framework.WaitForPodListConditionInput{
		Lister:      clusterClient,
		ListOptions: &client.ListOptions{LabelSelector: labels.Everything(), Namespace: "kube-system"},
		Condition:   framework.PhasePodCondition(corev1.PodRunning),
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-all-pod-to-be-running-on-target-cluster")...)

	By("Get the provisioned BMH names and UUIDs [node_reuse]")
	kcpBmhBeforeUpgrade := getProvisionedBmhNamesUuids(ctx, input.Namespace, clusterClient)

	By("Download image [node_reuse]")
	imageURL, imageChecksum := EnsureImage(toK8sVersion)

	By("Set nodeReuse field to 'True' and create new KCP Metal3MachineTemplate with upgraded image to boot [node_reuse]")
	m3MachineTemplateName := input.ClusterName + "-controlplane"
	updateNodeReuse(ctx, input.Namespace, true, m3MachineTemplateName, clusterClient)
	newM3MachineTemplateName := input.ClusterName + "-new-controlplane"
	CreateNewM3MachineTemplate(ctx, input.Namespace, newM3MachineTemplateName, m3MachineTemplateName, clusterClient, imageURL, imageChecksum)

	Byf("Update KCP to upgrade k8s version and binaries from %s to %s [node_reuse]", fromK8sVersion, toK8sVersion)
	kcpObj := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      clusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	helper, err := patch.NewHelper(kcpObj, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.MachineTemplate.Spec.InfrastructureRef.Name = newM3MachineTemplateName
	kcpObj.Spec.Version = toK8sVersion
	kcpObj.Spec.Rollout.Strategy.RollingUpdate.MaxSurge.IntVal = 0
	Logf("Disable tainting of CP nodes during the remainder of the node reuse test [node_reuse]")
	kcpObj.Spec.KubeadmConfigSpec.JoinConfiguration.NodeRegistration.Taints = ptr.To(emptyTaint)
	kcpObj.Spec.KubeadmConfigSpec.InitConfiguration.NodeRegistration.Taints = ptr.To(emptyTaint)

	By("Fetch cluster logs and manifests before patching KCP [node_reuse]")
	err = FetchClusterLogs(input.ClusterProxy, filepath.Join(input.ArtifactFolder, "node_reuse", "preUpgrade-kcp", "logs"))
	if err != nil {
		Logf("Error: %v", err)
	}

	err = FetchManifests(input.ClusterProxy, filepath.Join(input.ArtifactFolder, "node_reuse", "preUpgrade-kcp", "manifests"))
	if err != nil {
		Logf("Error fetching manifests: %v", err)
	}

	patchRequestError := helper.Patch(ctx, kcpObj)
	if patchRequestError != nil {
		Logf("Error while patching KCP: %s [node_reuse]", patchRequestError.Error())
	}
	Expect(patchRequestError).NotTo(HaveOccurred())
	// From this point onward all the checks run in a async parallel way next to the scale in upgrade process

	By("Check if only a single machine is in Deleting state and no other new machines are in Provisioning state [node_reuse]")
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseDeleting, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  1,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-deleting"),
	})
	// Since we do scale in, no Machine should start provisioning yet (the old must be deleted first)
	machineList := &clusterv1.MachineList{}
	Expect(clusterClient.List(ctx, machineList, client.InNamespace(input.Namespace))).To(Succeed())
	Expect(FilterMachinesByPhase(machineList.Items, clusterv1.MachinePhaseProvisioning)).To(BeEmpty())

	Byf("Wait until 1 BMH is in deprovisioning state [node_reuse]")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateDeprovisioning, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  1,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-deprovisioning"),
	})

	Logf("Find the deprovisioning BMH [node_reuse]")
	bmhList := bmov1alpha1.BareMetalHostList{}
	Expect(clusterClient.List(ctx, &bmhList, client.InNamespace(input.Namespace))).To(Succeed())
	deprovisioningBmhs := FilterBmhsByProvisioningState(bmhList.Items, bmov1alpha1.StateDeprovisioning)
	Expect(deprovisioningBmhs).To(HaveLen(1))
	key := types.NamespacedName{Name: deprovisioningBmhs[0].Name, Namespace: input.Namespace}

	By("Wait until above deprovisioning BMH is in available state again [node_reuse]")
	Eventually(
		func(g Gomega) {
			bmh := bmov1alpha1.BareMetalHost{}
			g.Expect(clusterClient.Get(ctx, key, &bmh)).To(Succeed())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateAvailable))
		}, input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-deprovisioning-available")...,
	).Should(Succeed())

	By("Fetch cluster logs and manifests after patching KCP [node_reuse]")
	err = FetchClusterLogs(input.ClusterProxy, filepath.Join(input.ArtifactFolder, "node_reuse", "postUpgrade-kcp", "logs"))
	if err != nil {
		Logf("Error: %v", err)
	}
	err = FetchManifests(input.ClusterProxy, filepath.Join(input.ArtifactFolder, "node_reuse", "postUpgrade-kcp", "manifests"))
	if err != nil {
		Logf("Error fetching manifests: %v", err)
	}
	By("Check if just deprovisioned BMH re-used for the next provisioning [node_reuse]")
	Eventually(
		func(g Gomega) {
			bmh := bmov1alpha1.BareMetalHost{}
			g.Expect(clusterClient.Get(ctx, key, &bmh)).To(Succeed())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateProvisioning))
		}, input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-available-provisioning")...,
	).Should(Succeed())

	Byf("Wait until all %v KCP machines become running and updated with new %s k8s version [node_reuse]", numberOfControlplane, toK8sVersion)
	runningAndUpgradedKCPMachines := func(machine clusterv1.Machine) bool {
		running := machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning
		upgraded := machine.Spec.Version == toK8sVersion
		_, isControlPlane := machine.GetLabels()[clusterv1.MachineControlPlaneLabel]
		return running && upgraded && isControlPlane
	}
	WaitForNumMachines(ctx, runningAndUpgradedKCPMachines, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfControlplane,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, clusterClient)

	By("Wait for all the pods to be running on the cluster after the upgrade process has ended [node_reuse]")
	framework.WaitForPodListCondition(ctx, framework.WaitForPodListConditionInput{
		Lister:      clusterClient,
		ListOptions: &client.ListOptions{LabelSelector: labels.Everything(), Namespace: "kube-system"},
		Condition:   framework.PhasePodCondition(corev1.PodRunning),
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-all-pod-to-be-running-on-target-cluster")...)
	Logf("The CP upgrade process has ended [node_reuse]")

	By("Get the provisioned BMH names and UUIDs after upgrade [node_reuse]")
	kcpBmhAfterUpgrade := getProvisionedBmhNamesUuids(ctx, input.Namespace, clusterClient)

	By("Check difference between before and after upgrade mappings")
	equal := reflect.DeepEqual(kcpBmhBeforeUpgrade, kcpBmhAfterUpgrade)
	Expect(equal).To(BeTrue(), "The same BMHs were not reused in KubeadmControlPlane test case")

	By("Put maxSurge field in KubeadmControlPlane back to default value(1) [node_reuse]")
	kcpObj = framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      clusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	helper, err = patch.NewHelper(kcpObj, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.Rollout.Strategy.RollingUpdate.MaxSurge.IntVal = 1
	for range 3 {
		err = helper.Patch(ctx, kcpObj)
		if err == nil {
			break
		}
		time.Sleep(30 * time.Second)
	}

	By("Scale the controlplane down to 1 [node_reuse]")
	ScaleKubeadmControlPlane(ctx, clusterClient, client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName}, 1)
	numberOfControlplane = 1
	Byf("Wait until controlplane is scaled down and %d BMHs are Available [node_reuse]", (numberOfAllBmh - numberOfControlplane - numberOfWorkers))
	WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh - numberOfControlplane - numberOfWorkers,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-cp-available"),
	})

	By("Wait for 1 CP nodes to be present after CP was scaled to 1 [node_reuse]")
	for controlplaneNodesPresent := 0; controlplaneNodesPresent != numberOfControlplane; {
		controlplaneNodes := getControlplaneNodes(ctx, clientSet)
		controlplaneNodesPresent = len(controlplaneNodes.Items)
		time.Sleep(5 * time.Second)
	}

	By("Wait for all the pods to be running on the cluster after CP was scaled to 1 [node_reuse]")
	framework.WaitForPodListCondition(ctx, framework.WaitForPodListConditionInput{
		Lister:      clusterClient,
		ListOptions: &client.ListOptions{LabelSelector: labels.Everything(), Namespace: "kube-system"},
		Condition:   framework.PhasePodCondition(corev1.PodRunning),
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-all-pod-to-be-running-on-target-cluster")...)

	By("Cluster components listed after CP was scaled to 1 [node_reuse]")
	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, clusterClient)

	By("Get MachineDeployment")
	machineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      clusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	Expect(machineDeployments).To(HaveLen(1), "Expected exactly 1 MachineDeployment")
	machineDeploy := machineDeployments[0]

	By("Get Metal3MachineTemplate name for MachineDeployment [node_reuse]")
	m3MachineTemplateName = input.ClusterName + "-workers"

	By("Scale the worker up to 1 to start testing MachineDeployment [node_reuse]")
	ScaleMachineDeployment(ctx, clusterClient, input.ClusterName, input.Namespace, 1)
	numberOfWorkers = 1

	Byf("Wait until the worker BMH becomes provisioned [node_reuse]")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfControlplane + numberOfWorkers,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-provisioned"),
	})

	Byf("Wait until worker machine becomes running with %s k8s version [node_reuse]", fromK8sVersion)
	runningWorkerMachines := func(machine clusterv1.Machine) bool {
		running := machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning
		upgraded := machine.Spec.Version == fromK8sVersion
		_, isControlPlane := machine.GetLabels()[clusterv1.MachineControlPlaneLabel]
		return running && upgraded && !isControlPlane
	}
	Byf("Wait until the worker machine becomes running [node_reuse]")
	WaitForNumMachines(ctx, runningWorkerMachines, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfWorkers,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	By("Wait for all the pods to be running on the cluster after worker node was scaled up to 1 [node_reuse]")
	framework.WaitForPodListCondition(ctx, framework.WaitForPodListConditionInput{
		Lister:      clusterClient,
		ListOptions: &client.ListOptions{LabelSelector: labels.Everything(), Namespace: "kube-system"},
		Condition:   framework.PhasePodCondition(corev1.PodRunning),
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-all-pod-to-be-running-on-target-cluster")...)

	By("Get the provisioned BMH names and UUIDs before starting upgrade in MachineDeployment [node_reuse]")
	mdBmhBeforeUpgrade := getProvisionedBmhNamesUuids(ctx, input.Namespace, clusterClient)

	By("List all available BMHs, remove nodeReuse label from them if any [node_reuse]")
	bmhs := bmov1alpha1.BareMetalHostList{}
	Expect(clusterClient.List(ctx, &bmhs, client.InNamespace(input.Namespace))).To(Succeed())
	for _, item := range bmhs.Items {
		if item.Status.Provisioning.State == bmov1alpha1.StateAvailable {
			// We make sure that all available BMHs are choosable by removing nodeReuse label
			// set on them while testing KCP node reuse scenario previously.
			DeleteNodeReuseLabelFromHost(ctx, clusterClient, item, nodeReuseLabel)
		}
	}

	// Verify labels were removed
	Eventually(func(g Gomega) {
		bmhs := bmov1alpha1.BareMetalHostList{}
		g.Expect(clusterClient.List(ctx, &bmhs, client.InNamespace(input.Namespace))).To(Succeed())
		for _, item := range bmhs.Items {
			if item.Status.Provisioning.State == bmov1alpha1.StateAvailable {
				g.Expect(item.Labels).NotTo(HaveKey(nodeReuseLabel))
			}
		}
	}, 30*time.Second, 5*time.Second).Should(Succeed())

	By("Set nodeReuse field to 'True' and create new Metal3MachineTemplate for MD with upgraded image to boot [node_reuse]")
	updateNodeReuse(ctx, input.Namespace, true, m3MachineTemplateName, clusterClient)
	newM3MachineTemplateName = input.ClusterName + "-new-workers"
	CreateNewM3MachineTemplate(ctx, input.Namespace, newM3MachineTemplateName, m3MachineTemplateName, clusterClient, imageURL, imageChecksum)

	Byf("Update MD to upgrade k8s version and binaries from %s to %s", fromK8sVersion, toK8sVersion)
	// Note: We have only 4 nodes (3 control-plane and 1 worker) so we
	// must allow maxUnavailable 1 here or it will get stuck.
	helper, err = patch.NewHelper(machineDeploy, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	machineDeploy.Spec.Rollout.Strategy.RollingUpdate.MaxSurge.IntVal = 0
	machineDeploy.Spec.Rollout.Strategy.RollingUpdate.MaxUnavailable.IntVal = 1
	machineDeploy.Spec.Template.Spec.InfrastructureRef.Name = newM3MachineTemplateName
	machineDeploy.Spec.Template.Spec.Version = toK8sVersion

	By("Fetch cluster logs and manifests before patching worker [node_reuse]")
	err = FetchClusterLogs(input.ClusterProxy, filepath.Join(input.ArtifactFolder, "node_reuse", "preUpgrade-worker", "logs"))
	if err != nil {
		Logf("Error: %v", err)
	}
	err = FetchManifests(input.ClusterProxy, filepath.Join(input.ArtifactFolder, "node_reuse", "preUpgrade-worker", "manifests"))
	if err != nil {
		Logf("Error fetching manifests: %v", err)
	}

	Expect(helper.Patch(ctx, machineDeploy)).To(Succeed())

	Byf("Wait until %d BMH(s) in deprovisioning state [node_reuse]", 1)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateDeprovisioning, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  1,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-deprovisioning"),
	})

	Logf("Find the deprovisioning BMH [node_reuse]")
	bmhList = bmov1alpha1.BareMetalHostList{}
	Expect(clusterClient.List(ctx, &bmhList, client.InNamespace(input.Namespace))).To(Succeed())
	deprovisioningBmhs = FilterBmhsByProvisioningState(bmhList.Items, bmov1alpha1.StateDeprovisioning)
	Expect(deprovisioningBmhs).To(HaveLen(1))
	key = types.NamespacedName{Name: deprovisioningBmhs[0].Name, Namespace: input.Namespace}

	By("Wait until the above deprovisioning BMH is in available state again [node_reuse]")
	Eventually(
		func(g Gomega) {
			bmh := bmov1alpha1.BareMetalHost{}
			g.Expect(clusterClient.Get(ctx, key, &bmh)).To(Succeed())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateAvailable))
		},
		input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-deprovisioning-available")...,
	).Should(Succeed())

	By("Fetch cluster logs and manifests after patching worker [node_reuse]")
	err = FetchClusterLogs(input.ClusterProxy, filepath.Join(input.ArtifactFolder, "node_reuse", "postUpgrade-worker", "logs"))
	if err != nil {
		Logf("Error: %v", err)
	}
	err = FetchManifests(input.ClusterProxy, filepath.Join(input.ArtifactFolder, "node_reuse", "postUpgrade-worker", "manifests"))
	if err != nil {
		Logf("Error fetching manifests: %v", err)
	}

	By("Check if just deprovisioned BMH re-used for next provisioning [node_reuse]")
	Eventually(
		func(g Gomega) {
			bmh := bmov1alpha1.BareMetalHost{}
			key := types.NamespacedName{Name: deprovisioningBmhs[0].Name, Namespace: input.Namespace}
			g.Expect(clusterClient.Get(ctx, key, &bmh)).To(Succeed())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateProvisioning))
		},
		input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-available-provisioning")...,
	).Should(Succeed())

	Byf("Wait until worker machine becomes running and updated with new %s k8s version [node_reuse]", toK8sVersion)
	runningAndUpgradedWorkerMachines := func(machine clusterv1.Machine) bool {
		running := machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning
		upgraded := machine.Spec.Version == toK8sVersion
		_, isControlPlane := machine.GetLabels()[clusterv1.MachineControlPlaneLabel]
		return running && upgraded && !isControlPlane
	}
	WaitForNumMachines(ctx, runningAndUpgradedWorkerMachines, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfWorkers,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	By("Get provisioned BMH names and UUIDs after upgrade in MachineDeployment [node_reuse]")
	mdBmhAfterUpgrade := getProvisionedBmhNamesUuids(ctx, input.Namespace, clusterClient)

	By("Check difference between before and after upgrade mappings in MachineDeployment [node_reuse]")
	equal = reflect.DeepEqual(mdBmhBeforeUpgrade, mdBmhAfterUpgrade)
	Expect(equal).To(BeTrue(), "The same BMHs were not reused in MachineDeployment")

	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, clusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, clusterClient)

	By("Scale controlplane up to 3 [node_reuse]")
	ScaleKubeadmControlPlane(ctx, clusterClient, client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName}, 3)
	numberOfControlplane = 3
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfControlplane + numberOfWorkers,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-provisioned"),
	})

	Byf("Wait until all %d machine(s) become(s) running [node_reuse]", numberOfControlplane+numberOfWorkers)
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfControlplane + numberOfWorkers,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	By("Wait for all the pods to be running at the end of the node_reuse [node_reuse]")
	framework.WaitForPodListCondition(ctx, framework.WaitForPodListConditionInput{
		Lister:      clusterClient,
		ListOptions: &client.ListOptions{LabelSelector: labels.Everything(), Namespace: "kube-system"},
		Condition:   framework.PhasePodCondition(corev1.PodRunning),
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-all-pod-to-be-running-on-target-cluster")...)

	//NOTES: At the end of node reuse the KCP is still configured to not taint the nodes
	By("NODE REUSE TESTS PASSED!")
}

func getProvisionedBmhNamesUuids(ctx context.Context, namespace string, clusterClient client.Client) []string {
	bmhs := bmov1alpha1.BareMetalHostList{}
	var nameUUIDList []string
	Expect(clusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
	for _, item := range bmhs.Items {
		if item.WasProvisioned() {
			concat := "metal3/" + item.Name + "=metal3://" + (string)(item.UID)
			nameUUIDList = append(nameUUIDList, concat)
		}
	}
	return nameUUIDList
}

func updateNodeReuse(ctx context.Context, namespace string, nodeReuse bool, m3MachineTemplateName string, clusterClient client.Client) {
	m3machineTemplate := infrav1beta1.Metal3MachineTemplate{}
	Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3MachineTemplateName}, &m3machineTemplate)).To(Succeed())
	helper, err := patch.NewHelper(&m3machineTemplate, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	m3machineTemplate.Spec.NodeReuse = nodeReuse
	Expect(helper.Patch(ctx, &m3machineTemplate)).To(Succeed())

	// verify that nodeReuse field is updated
	Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3MachineTemplateName}, &m3machineTemplate)).To(Succeed())
	Expect(m3machineTemplate.Spec.NodeReuse).To(BeEquivalentTo(nodeReuse))
}
