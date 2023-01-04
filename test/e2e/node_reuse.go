package e2e

import (
	"context"
	"fmt"
	"reflect"
	"time"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeReuseInput struct {
	E2EConfig         *clusterctl.E2EConfig
	ManagementCluster framework.ClusterProxy
	TargetCluster     framework.ClusterProxy
	SpecName          string
	ClusterName       string
	Namespace         string
}

func nodeReuse(ctx context.Context, inputGetter func() NodeReuseInput) {
	Logf("Starting node reuse tests")
	input := inputGetter()
	targetClusterClient := input.TargetCluster.GetClient()
	managementClusterClient := input.ManagementCluster.GetClient()
	clientSet := input.TargetCluster.GetClientSet()
	kubernetesVersion := input.E2EConfig.GetVariable("FROM_K8S_VERSION")
	upgradedK8sVersion := input.E2EConfig.GetVariable("KUBERNETES_VERSION")
	numberOfWorkers := int(*input.E2EConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
	numberOfControlplane := int(*input.E2EConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
	numberOfAllBmh := numberOfWorkers + numberOfControlplane

	var (
		controlplaneTaints = []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule}}
	)

	const (
		artifactoryURL = "https://artifactory.nordix.org/artifactory/metal3/images/k8s"
		imagesURL      = "http://172.22.0.1/images"
		ironicImageDir = "/opt/metal3-dev-env/ironic/html/images"
		nodeReuseLabel = "infrastructure.cluster.x-k8s.io/node-reuse"
	)

	Logf("KUBERNETES VERSION: %v", kubernetesVersion)
	Logf("UPGRADED K8S VERSION: %v", upgradedK8sVersion)
	Logf("NUMBER OF CONTROLPLANE BMH: %v", numberOfControlplane)
	Logf("NUMBER OF WORKER BMH: %v", numberOfWorkers)

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	By("Untaint all CP nodes before scaling down machinedeployment")
	controlplaneNodes := getControlplaneNodes(ctx, clientSet)
	untaintNodes(ctx, targetClusterClient, controlplaneNodes, controlplaneTaints)

	By("Scale down MachineDeployment to 0")
	ScaleMachineDeployment(ctx, managementClusterClient, input.ClusterName, input.Namespace, 0)

	Byf("Wait until the worker is scaled down and %d BMH(s) Available", 1)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  1,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-available"),
	})

	By("Get the provisioned BMH names and UUIDs")
	kcpBmhBeforeUpgrade := getProvisionedBmhNamesUuids(ctx, input.Namespace, managementClusterClient)

	By("Download image")
	imageURL, imageChecksum := EnsureImage(upgradedK8sVersion)
	By("Set nodeReuse field to 'True' and create new KCP Metal3MachineTemplate with upgraded image to boot")
	m3machineTemplateName := fmt.Sprintf("%s-controlplane", input.ClusterName)
	updateNodeReuse(ctx, input.Namespace, true, m3machineTemplateName, managementClusterClient)
	newM3machineTemplateName := fmt.Sprintf("%s-new-controlplane", input.ClusterName)
	createNewM3machineTemplate(ctx, input.Namespace, newM3machineTemplateName, m3machineTemplateName, managementClusterClient, imageURL, imageChecksum, "raw", "md5")

	Byf("Update KCP to upgrade k8s version and binaries from %s to %s", kubernetesVersion, upgradedK8sVersion)
	kcpObj := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      managementClusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	helper, err := patch.NewHelper(kcpObj, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.MachineTemplate.InfrastructureRef.Name = newM3machineTemplateName
	kcpObj.Spec.Version = upgradedK8sVersion
	kcpObj.Spec.RolloutStrategy.RollingUpdate.MaxSurge.IntVal = 0
	Expect(helper.Patch(ctx, kcpObj)).To(Succeed())

	By("Check if only a single machine is in Deleting state and no other new machines are in Provisioning state")
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseDeleting, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  1,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-deleting"),
	})
	// Since we do scale in, no Machine should start provisioning yet (the old must be deleted first)
	machineList := &clusterv1.MachineList{}
	Expect(managementClusterClient.List(ctx, machineList, client.InNamespace(input.Namespace))).To(Succeed())
	Expect(FilterMachinesByPhase(machineList.Items, clusterv1.MachinePhaseProvisioning)).To(HaveLen(0))

	Byf("Wait until 1 BMH is in deprovisioning state")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateDeprovisioning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  1,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-deprovisioning"),
	})

	Logf("Find the deprovisioning BMH")
	bmhList := bmov1alpha1.BareMetalHostList{}
	Expect(managementClusterClient.List(ctx, &bmhList, client.InNamespace(input.Namespace))).To(Succeed())
	deprovisioningBmhs := FilterBmhsByProvisioningState(bmhList.Items, bmov1alpha1.StateDeprovisioning)
	Expect(deprovisioningBmhs).To(HaveLen(1))
	key := types.NamespacedName{Name: deprovisioningBmhs[0].Name, Namespace: input.Namespace}

	By("Wait until above deprovisioning BMH is in available state again")
	Eventually(
		func(g Gomega) {
			bmh := bmov1alpha1.BareMetalHost{}
			g.Expect(managementClusterClient.Get(ctx, key, &bmh)).To(Succeed())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateAvailable))
		}, input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-deprovisioning-available")...,
	).Should(Succeed())

	By("Check if just deprovisioned BMH re-used for the next provisioning")
	Eventually(
		func(g Gomega) {
			bmh := bmov1alpha1.BareMetalHost{}
			g.Expect(managementClusterClient.Get(ctx, key, &bmh)).To(Succeed())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateProvisioning))
		}, input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-available-provisioning")...,
	).Should(Succeed())

	Byf("Wait until two machines become running and updated with the new %s k8s version", upgradedK8sVersion)
	runningAndUpgraded := func(machine clusterv1.Machine) bool {
		running := machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning
		upgraded := *machine.Spec.Version == upgradedK8sVersion
		return (running && upgraded)
	}
	WaitForNumMachines(ctx, runningAndUpgraded, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  2,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	By("Untaint CP nodes after upgrade of two controlplane nodes")
	controlplaneNodes = getControlplaneNodes(ctx, clientSet)
	untaintNodes(ctx, targetClusterClient, controlplaneNodes, controlplaneTaints)

	Byf("Wait until all %v KCP machines become running and updated with new %s k8s version", numberOfControlplane, upgradedK8sVersion)
	WaitForNumMachines(ctx, runningAndUpgraded, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfControlplane,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	By("Get the provisioned BMH names and UUIDs after upgrade")
	kcpBmhAfterUpgrade := getProvisionedBmhNamesUuids(ctx, input.Namespace, managementClusterClient)

	By("Check difference between before and after upgrade mappings")
	equal := reflect.DeepEqual(kcpBmhBeforeUpgrade, kcpBmhAfterUpgrade)
	Expect(equal).To(BeTrue(), "The same BMHs were not reused in KubeadmControlPlane test case")

	By("Put maxSurge field in KubeadmControlPlane back to default value(1)")
	kcpObj = framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      managementClusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	helper, err = patch.NewHelper(kcpObj, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.RolloutStrategy.RollingUpdate.MaxSurge.IntVal = 1
	for retry := 0; retry < 3; retry++ {
		err = helper.Patch(ctx, kcpObj)
		if err == nil {
			break
		}
		time.Sleep(30 * time.Second)
	}

	By("Untaint all CP nodes")
	// The rest of CP nodes may take time to be untaintable
	// We have untainted the 2 first CPs
	for untaintedNodeCount := 0; untaintedNodeCount < numberOfControlplane-2; {
		controlplaneNodes = getControlplaneNodes(ctx, clientSet)
		untaintedNodeCount = untaintNodes(ctx, targetClusterClient, controlplaneNodes, controlplaneTaints)
		time.Sleep(10 * time.Second)
	}

	By("Scale the controlplane down to 1")
	ScaleKubeadmControlPlane(ctx, managementClusterClient, client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName}, 1)

	Byf("Wait until controlplane is scaled down and %d BMHs are Available", numberOfControlplane)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfControlplane,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-cp-available"),
	})

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	By("Get MachineDeployment")
	machineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      managementClusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	Expect(len(machineDeployments)).To(Equal(1), "Expected exactly 1 MachineDeployment")
	machineDeploy := machineDeployments[0]

	By("Get Metal3MachineTemplate name for MachineDeployment")
	m3machineTemplateName = fmt.Sprintf("%s-workers", input.ClusterName)

	By("Point to proper Metal3MachineTemplate in MachineDeployment")
	pointMDtoM3mt(ctx, input.Namespace, input.ClusterName, m3machineTemplateName, machineDeploy.Name, managementClusterClient)

	By("Scale the worker up to 1 to start testing MachineDeployment")
	ScaleMachineDeployment(ctx, managementClusterClient, input.ClusterName, input.Namespace, 1)

	Byf("Wait until the worker BMH becomes provisioned")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  2,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-provisioned"),
	})

	Byf("Wait until the worker machine becomes running")
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  2,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	By("Get the provisioned BMH names and UUIDs before starting upgrade in MachineDeployment")
	mdBmhBeforeUpgrade := getProvisionedBmhNamesUuids(ctx, input.Namespace, managementClusterClient)

	By("List all available BMHs, remove nodeReuse label from them if any")
	bmhs := bmov1alpha1.BareMetalHostList{}
	Expect(managementClusterClient.List(ctx, &bmhs, client.InNamespace(input.Namespace))).To(Succeed())
	for _, item := range bmhs.Items {
		if item.Status.Provisioning.State == bmov1alpha1.StateAvailable {
			// We make sure that all available BMHs are choosable by removing nodeReuse label
			// set on them while testing KCP node reuse scenario previously.
			DeleteNodeReuseLabelFromHost(ctx, managementClusterClient, item, nodeReuseLabel)
		}
	}

	By("Set nodeReuse field to 'True' and create new Metal3MachineTemplate for MD with upgraded image to boot")
	updateNodeReuse(ctx, input.Namespace, true, m3machineTemplateName, managementClusterClient)
	newM3machineTemplateName = fmt.Sprintf("%s-new-workers", input.ClusterName)
	createNewM3machineTemplate(ctx, input.Namespace, newM3machineTemplateName, m3machineTemplateName, managementClusterClient, imageURL, imageChecksum, "raw", "md5")

	Byf("Update MD to upgrade k8s version and binaries from %s to %s", kubernetesVersion, upgradedK8sVersion)
	// Note: We have only 4 nodes (3 control-plane and 1 worker) so we
	// must allow maxUnavailable 1 here or it will get stuck.
	helper, err = patch.NewHelper(machineDeploy, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	machineDeploy.Spec.Strategy.RollingUpdate.MaxSurge.IntVal = 0
	machineDeploy.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal = 1
	machineDeploy.Spec.Template.Spec.InfrastructureRef.Name = newM3machineTemplateName
	machineDeploy.Spec.Template.Spec.Version = &upgradedK8sVersion
	Expect(helper.Patch(ctx, machineDeploy)).To(Succeed())

	Byf("Wait until %d BMH(s) in deprovisioning state", 1)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateDeprovisioning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  1,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-deprovisioning"),
	})

	Logf("Find the deprovisioning BMH")
	bmhList = bmov1alpha1.BareMetalHostList{}
	Expect(managementClusterClient.List(ctx, &bmhList, client.InNamespace(input.Namespace))).To(Succeed())
	deprovisioningBmhs = FilterBmhsByProvisioningState(bmhList.Items, bmov1alpha1.StateDeprovisioning)
	Expect(deprovisioningBmhs).To(HaveLen(1))
	key = types.NamespacedName{Name: deprovisioningBmhs[0].Name, Namespace: input.Namespace}

	By("Wait until the above deprovisioning BMH is in available state again")
	Eventually(
		func(g Gomega) {
			bmh := bmov1alpha1.BareMetalHost{}
			g.Expect(managementClusterClient.Get(ctx, key, &bmh)).To(Succeed())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateAvailable))
		},
		input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-deprovisioning-available")...,
	).Should(Succeed())

	By("Check if just deprovisioned BMH re-used for next provisioning")
	Eventually(
		func(g Gomega) {
			bmh := bmov1alpha1.BareMetalHost{}
			key := types.NamespacedName{Name: deprovisioningBmhs[0].Name, Namespace: input.Namespace}
			g.Expect(managementClusterClient.Get(ctx, key, &bmh)).To(Succeed())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateProvisioning))
		},
		input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-available-provisioning")...,
	).Should(Succeed())

	Byf("Wait until worker machine becomes running and updated with new %s k8s version", upgradedK8sVersion)
	WaitForNumMachines(ctx, runningAndUpgraded, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  2,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	By("Get provisioned BMH names and UUIDs after upgrade in MachineDeployment")
	mdBmhAfterUpgrade := getProvisionedBmhNamesUuids(ctx, input.Namespace, managementClusterClient)

	By("Check difference between before and after upgrade mappings in MachineDeployment")
	equal = reflect.DeepEqual(mdBmhBeforeUpgrade, mdBmhAfterUpgrade)
	Expect(equal).To(BeTrue(), "The same BMHs were not reused in MachineDeployment")

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	By("Scale controlplane up to 3")
	ScaleKubeadmControlPlane(ctx, managementClusterClient, client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName}, 3)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-provisioned"),
	})

	Byf("Wait until all %d machine(s) become(s) running", numberOfAllBmh)
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  numberOfAllBmh,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	By("NODE REUSE TESTS PASSED!")
}

func getControlplaneNodes(ctx context.Context, clientSet *kubernetes.Clientset) *corev1.NodeList {
	controlplaneNodesRequirement, err := labels.NewRequirement("node-role.kubernetes.io/control-plane", selection.Exists, []string{})
	Expect(err).To(BeNil(), "Failed to set up worker Node requirements")
	controlplaneNodesSelector := labels.NewSelector().Add(*controlplaneNodesRequirement)
	controlplaneListOptions := metav1.ListOptions{LabelSelector: controlplaneNodesSelector.String()}
	controlplaneNodes, err := clientSet.CoreV1().Nodes().List(ctx, controlplaneListOptions)
	Expect(err).To(BeNil(), "Failed to get controlplane nodes")
	Logf("controlplaneNodes found %v", len(controlplaneNodes.Items))
	return controlplaneNodes
}

func getProvisionedBmhNamesUuids(ctx context.Context, namespace string, managementClusterClient client.Client) []string {
	bmhs := bmov1alpha1.BareMetalHostList{}
	var nameUUIDList []string
	Expect(managementClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
	for _, item := range bmhs.Items {
		if item.WasProvisioned() {
			concat := "metal3/" + item.Name + "=metal3://" + (string)(item.UID)
			nameUUIDList = append(nameUUIDList, concat)
		}
	}
	return nameUUIDList
}

func updateNodeReuse(ctx context.Context, namespace string, nodeReuse bool, m3machineTemplateName string, managementClusterClient client.Client) {
	m3machineTemplate := infrav1.Metal3MachineTemplate{}
	Expect(managementClusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3machineTemplateName}, &m3machineTemplate)).To(Succeed())
	helper, err := patch.NewHelper(&m3machineTemplate, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	m3machineTemplate.Spec.NodeReuse = nodeReuse
	Expect(helper.Patch(ctx, &m3machineTemplate)).To(Succeed())

	// verify that nodeReuse field is updated
	Expect(managementClusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3machineTemplateName}, &m3machineTemplate)).To(Succeed())
	Expect(m3machineTemplate.Spec.NodeReuse).To(BeEquivalentTo(nodeReuse))
}

func pointMDtoM3mt(ctx context.Context, namespace string, clusterName string, m3mtname, mdName string, managementClusterClient client.Client) {
	md := clusterv1.MachineDeployment{}
	Expect(managementClusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: mdName}, &md)).To(Succeed())
	helper, err := patch.NewHelper(&md, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	md.Spec.Template.Spec.InfrastructureRef.Name = m3mtname
	Expect(helper.Patch(ctx, &md)).To(Succeed())

	// verify that MachineDeployment is pointing to exact m3mt where nodeReuse is set to 'True'
	Expect(managementClusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: mdName}, &md)).To(Succeed())
	Expect(md.Spec.Template.Spec.InfrastructureRef.Name).To(BeEquivalentTo(fmt.Sprintf("%s-workers", clusterName)))
}

func untaintNodes(ctx context.Context, targetClusterClient client.Client, nodes *corev1.NodeList, taints []corev1.Taint) (count int) {
	count = 0
	for i := range nodes.Items {
		Logf("Untainting node %v ...", nodes.Items[i].Name)
		newNode, changed := removeTaint(&nodes.Items[i], taints)
		if changed {
			patchHelper, err := patch.NewHelper(&nodes.Items[i], targetClusterClient)
			Expect(err).To(BeNil())
			Expect(patchHelper.Patch(ctx, newNode)).To(Succeed(), "Failed to patch node")
			count++
		}
	}
	return
}

func removeTaint(node *corev1.Node, taints []corev1.Taint) (*corev1.Node, bool) {
	newNode := node.DeepCopy()
	nodeTaints := newNode.Spec.Taints
	if len(nodeTaints) == 0 {
		return newNode, false
	}

	if !taintExists(nodeTaints, taints) {
		return newNode, false
	}

	newTaints, _ := deleteTaint(nodeTaints, taints)
	newNode.Spec.Taints = newTaints
	return newNode, true
}

func taintExists(taints []corev1.Taint, taintsToFind []corev1.Taint) bool {
	for _, taint := range taints {
		for i := range taintsToFind {
			if taint.MatchTaint(&taintsToFind[i]) {
				return true
			}
		}
	}
	return false
}

func deleteTaint(taints []corev1.Taint, taintsToDelete []corev1.Taint) ([]corev1.Taint, bool) {
	newTaints := []corev1.Taint{}
	deleted := false
	for i := range taints {
		currentTaintDeleted := false
		for _, taintToDelete := range taintsToDelete {
			if taintToDelete.MatchTaint(&taints[i]) {
				deleted = true
				currentTaintDeleted = true
			}
		}
		if !currentTaintDeleted {
			newTaints = append(newTaints, taints[i])
		}
	}
	return newTaints, deleted
}

func createNewM3machineTemplate(ctx context.Context, namespace string, newM3machineTemplateName string, m3machineTemplateName string, clusterClient client.Client, imageURL string, imageChecksum string, checksumType string, imageFormat string) {
	m3machineTemplate := infrav1.Metal3MachineTemplate{}
	Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3machineTemplateName}, &m3machineTemplate)).To(Succeed())

	newM3MachineTemplate := m3machineTemplate.DeepCopy()
	cleanObjectMeta(&newM3MachineTemplate.ObjectMeta)

	newM3MachineTemplate.Spec.Template.Spec.Image.URL = imageURL
	newM3MachineTemplate.Spec.Template.Spec.Image.Checksum = imageChecksum
	newM3MachineTemplate.Spec.Template.Spec.Image.DiskFormat = &checksumType
	newM3MachineTemplate.Spec.Template.Spec.Image.ChecksumType = &imageFormat
	newM3MachineTemplate.ObjectMeta.Name = newM3machineTemplateName

	Expect(clusterClient.Create(ctx, newM3MachineTemplate)).To(Succeed(), "Failed to create new Metal3MachineTemplate")
}
