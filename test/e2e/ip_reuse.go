package e2e

import (
	"context"
	"reflect"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IPReuseInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
}

func IPReuse(ctx context.Context, inputGetter func() IPReuseInput) {
	Logf("Starting IP reuse tests")
	input := inputGetter()
	targetClusterClient := input.TargetCluster.GetClient()
	managementClusterClient := input.BootstrapClusterProxy.GetClient()
	fromK8sVersion := input.E2EConfig.MustGetVariable("KUBERNETES_VERSION_UPGRADE_FROM")
	toK8sVersion := input.E2EConfig.MustGetVariable("KUBERNETES_VERSION")
	numberOfControlplane := *input.E2EConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT")
	numberOfWorkers := *input.E2EConfig.MustGetInt32PtrVariable("WORKER_MACHINE_COUNT")
	numberOfAllBmh := numberOfControlplane + numberOfWorkers

	// Download node image
	Byf("Download image %s", toK8sVersion)
	imageURL, imageChecksum := EnsureImage(toK8sVersion)

	// Upgrade KCP
	By("Create new KCP Metal3MachineTemplate with upgraded image to boot")
	KCPm3MachineTemplateName := input.ClusterName + "-controlplane"
	KCPNewM3MachineTemplateName := input.ClusterName + "-new-controlplane"
	CreateNewM3MachineTemplate(ctx, input.Namespace, KCPNewM3MachineTemplateName, KCPm3MachineTemplateName, managementClusterClient, imageURL, imageChecksum)

	Byf("Update KCP to upgrade k8s version and binaries from %s to %s", fromK8sVersion, toK8sVersion)
	kcpObj := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      managementClusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	helper, err := patch.NewHelper(kcpObj, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.MachineTemplate.Spec.InfrastructureRef.Name = KCPNewM3MachineTemplateName
	kcpObj.Spec.Version = toK8sVersion
	kcpObj.Spec.Rollout.Strategy.RollingUpdate.MaxSurge.IntVal = 0

	Expect(helper.Patch(ctx, kcpObj)).To(Succeed())

	Byf("Wait until %d Control Plane machines become running and updated with the new %s k8s version", numberOfControlplane, toK8sVersion)
	runningAndUpgradedKCPMachines := func(machine clusterv1.Machine) bool {
		running := machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning
		upgraded := machine.Spec.Version == toK8sVersion
		_, isControlPlane := machine.GetLabels()[clusterv1.MachineControlPlaneLabel]
		return running && upgraded && isControlPlane
	}
	WaitForNumMachines(ctx, runningAndUpgradedKCPMachines, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  int(numberOfControlplane),
		Intervals: input.E2EConfig.GetIntervals(input.Namespace, "wait-machine-running"),
	})

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	By("Get the IPPools in the cluster")
	baremetalv4Pool, provisioningPool := GetIPPools(ctx, managementClusterClient, input.ClusterName, input.Namespace)
	Expect(baremetalv4Pool).To(HaveLen(1))
	Expect(provisioningPool).To(HaveLen(1))
	baremetalv4PoolName := baremetalv4Pool[0].Name
	provisioningPoolName := provisioningPool[0].Name

	By("Get new values of spec.Preallocations field for baremetal IPPool")
	bmv4PoolPreallocations, err := GenerateIPPoolPreallocations(ctx, baremetalv4Pool[0], baremetalv4PoolName, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	Logf("new values of spec.Preallocations field for baremetal IPPool is: %s", bmv4PoolPreallocations)
	By("Get new values of spec.Preallocations field for provisioning IPPool")
	provPoolPreallocations, err := GenerateIPPoolPreallocations(ctx, provisioningPool[0], provisioningPoolName, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	Logf("new values of spec.Preallocations field for provisioning IPPool is: %s", provPoolPreallocations)

	By("Patch baremetal IPPool with new Preallocations field and values")
	Expect(managementClusterClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: baremetalv4PoolName}, &baremetalv4Pool[0])).To(Succeed())
	bmv4helper, err := patch.NewHelper(&baremetalv4Pool[0], managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	baremetalv4Pool[0].Spec.PreAllocations = bmv4PoolPreallocations
	Expect(bmv4helper.Patch(ctx, &baremetalv4Pool[0])).To(Succeed())

	By("Patch provisioning IPPool with new Preallocations field and values")
	Expect(managementClusterClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: provisioningPoolName}, &provisioningPool[0])).To(Succeed())
	provhelper, err := patch.NewHelper(&provisioningPool[0], managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	provisioningPool[0].Spec.PreAllocations = provPoolPreallocations
	Expect(provhelper.Patch(ctx, &provisioningPool[0])).To(Succeed())

	// patch MD maxUnavailable and k8s version to start a parallel upgrade
	By("Get MachineDeployment")
	machineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      managementClusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	Expect(machineDeployments).To(HaveLen(1), "Expected exactly 1 MachineDeployment")
	md := machineDeployments[0]

	By("Create new worker Metal3MachineTemplate with upgraded image to boot")
	m3MachineTemplateName := md.Spec.Template.Spec.InfrastructureRef.Name
	newM3MachineTemplateName := m3MachineTemplateName + "-new"
	CreateNewM3MachineTemplate(ctx, input.Namespace, newM3MachineTemplateName, m3MachineTemplateName, managementClusterClient, imageURL, imageChecksum)

	Byf("Update MachineDeployment maxUnavailable to number of workers and k8s version from %s to %s", fromK8sVersion, toK8sVersion)
	helper, err = patch.NewHelper(md, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	md.Spec.Template.Spec.InfrastructureRef.Name = newM3MachineTemplateName
	md.Spec.Template.Spec.Version = toK8sVersion
	md.Spec.Rollout.Strategy.RollingUpdate.MaxSurge.IntVal = 0
	md.Spec.Rollout.Strategy.RollingUpdate.MaxUnavailable.IntVal = numberOfWorkers
	Expect(helper.Patch(ctx, md)).To(Succeed())

	Byf("Wait until %d BMH(s) in deprovisioning state", numberOfWorkers)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateDeprovisioning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  int(numberOfWorkers),
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-deprovisioning"),
	})

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	Byf("Wait until all %d machine(s) become(s) running", numberOfAllBmh)
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  int(numberOfAllBmh),
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	By("Get the IPPools in the cluster")
	baremetalv4Pool, provisioningPool = GetIPPools(ctx, managementClusterClient, input.ClusterName, input.Namespace)
	Expect(baremetalv4Pool).To(HaveLen(1))
	Expect(provisioningPool).To(HaveLen(1))

	By("Check if same IP addresses are reused for nodes")
	Logf("baremetalv4Pool[0].Spec.PreAllocations: %v", baremetalv4Pool[0].Spec.PreAllocations)
	Logf("baremetalv4Pool[0].Status.Allocations: %v", baremetalv4Pool[0].Status.Allocations)
	bmv4equal := reflect.DeepEqual(baremetalv4Pool[0].Spec.PreAllocations, baremetalv4Pool[0].Status.Allocations)
	Expect(bmv4equal).To(BeTrue(), "The same IP addreesses from baremetal IPPool were not reused for nodes")
	Logf("provisioningPool[0].Spec.PreAllocations: %v", provisioningPool[0].Spec.PreAllocations)
	Logf("provisioningPool[0].Status.Allocations: %v", provisioningPool[0].Status.Allocations)
	provequal := reflect.DeepEqual(provisioningPool[0].Spec.PreAllocations, provisioningPool[0].Status.Allocations)
	Expect(provequal).To(BeTrue(), "The same IP addreesses from provisioning IPPool were not reused for nodes")

	By("Check that every provisioned BareMetalHost is consumed and its reused IPs resolve through the IP chain")
	VerifyBmhsReuseIPs(ctx, managementClusterClient, input.Namespace, baremetalv4Pool[0], provisioningPool[0])

	By("Check that each reused baremetal IP is actually used as the InternalIP of its Node")
	VerifyNodesReuseIPs(ctx, managementClusterClient, targetClusterClient, input.Namespace, baremetalv4Pool[0])

	By("IP REUSE TESTS PASSED!")
}

// VerifyBmhsReuseIPs verifies, for every Machine, that the
// BareMetalHost backing it is provisioned and consumed, and that the IPs
// allocated to the Machine from both IPPools are recorded in the pool allocations.
func VerifyBmhsReuseIPs(ctx context.Context, c client.Client, namespace string, baremetalPool, provisioningPool ipamv1.IPPool) {
	machines := clusterv1.MachineList{}
	Expect(c.List(ctx, &machines, client.InNamespace(namespace))).To(Succeed())
	Expect(machines.Items).NotTo(BeEmpty(), "Expected at least one Machine in namespace %s", namespace)

	bmhs, err := GetAllBmhs(ctx, c, namespace)
	Expect(err).NotTo(HaveOccurred(), "Failed to list BareMetalHosts in namespace %s", namespace)
	bmhByName := make(map[string]*bmov1alpha1.BareMetalHost, len(bmhs))
	for i := range bmhs {
		bmhByName[bmhs[i].Name] = &bmhs[i]
	}

	controlplaneM3Machines, workerM3Machines := GetMetal3Machines(ctx, c, "", namespace)
	m3Machines := append(controlplaneM3Machines, workerM3Machines...)

	for i := range machines.Items {
		machine := &machines.Items[i]

		m3ms := FilterMetal3MachinesByName(m3Machines, machine.Spec.InfrastructureRef.Name)
		Expect(m3ms).To(HaveLen(1), "expected exactly one Metal3Machine named %s for Machine %s", machine.Spec.InfrastructureRef.Name, machine.Name)
		m3machine := m3ms[0]

		bmhName := Metal3MachineToBmhName(m3machine)
		bmh := bmhByName[bmhName]
		Expect(bmh).NotTo(BeNil(), "BareMetalHost %s for Machine %s not found", bmhName, machine.Name)
		Expect(bmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateProvisioned), "BareMetalHost %s is not provisioned", bmhName)
		Expect(bmh.Spec.ConsumerRef).NotTo(BeNil(), "provisioned BareMetalHost %s has no ConsumerRef", bmhName)
		Expect(bmh.Spec.ConsumerRef.Name).To(Equal(m3machine.Name), "BareMetalHost %s is not consumed by Metal3Machine %s", bmhName, m3machine.Name)

		bmIP := machineIPInPool(ctx, c, machine, baremetalPool)
		provIP := machineIPInPool(ctx, c, machine, provisioningPool)
		Logf("BareMetalHost %s (consumed by %s) uses baremetal IP %s and provisioning IP %s", bmhName, m3machine.Name, bmIP, provIP)
	}
}

// VerifyNodesReuseIPs verifies that the baremetal IP appears as a NodeInternalIP of
// the Node referenced by the Machine.
func VerifyNodesReuseIPs(ctx context.Context, mgmtClient, targetClient client.Client, namespace string, baremetalPool ipamv1.IPPool) {
	machines := clusterv1.MachineList{}
	Expect(mgmtClient.List(ctx, &machines, client.InNamespace(namespace))).To(Succeed())
	Expect(machines.Items).NotTo(BeEmpty(), "Expected at least one Machine in namespace %s", namespace)

	for i := range machines.Items {
		machine := &machines.Items[i]
		Expect(machine.Status.NodeRef.IsDefined()).To(BeTrue(), "Machine %s has no NodeRef", machine.Name)

		bmIP := machineIPInPool(ctx, mgmtClient, machine, baremetalPool)

		node := corev1.Node{}
		Expect(targetClient.Get(ctx, client.ObjectKey{Name: machine.Status.NodeRef.Name}, &node)).To(Succeed(),
			"failed to get Node %s for Machine %s", machine.Status.NodeRef.Name, machine.Name)

		nodeInternalIPs := make([]string, 0, len(node.Status.Addresses))
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				nodeInternalIPs = append(nodeInternalIPs, addr.Address)
			}
		}
		Expect(nodeInternalIPs).To(ContainElement(bmIP),
			"baremetal IP %s is not used as an InternalIP of Node %s (addresses: %v)", bmIP, node.Name, nodeInternalIPs)

		Logf("Node %s uses reused baremetal IP %s", node.Name, bmIP)
	}
}

// machineIPInPool resolves the IP allocated to the Machine from the given IPPool
// and asserts that it is present in the pool's allocations.
func machineIPInPool(ctx context.Context, c client.Client, machine *clusterv1.Machine, pool ipamv1.IPPool) string {
	ip, err := MachineToIPAddress(ctx, c, machine, pool)
	Expect(err).NotTo(HaveOccurred(), "failed to resolve IP for Machine %s from pool %s", machine.Name, pool.Name)
	allocated := false
	for _, a := range pool.Status.Allocations {
		if string(a) == ip {
			allocated = true
			break
		}
	}
	Expect(allocated).To(BeTrue(), "IP %s for Machine %s is not part of IPPool %s allocations", ip, machine.Name, pool.Name)
	return ip
}
