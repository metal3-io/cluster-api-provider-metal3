package e2e

import (
	"context"
	"reflect"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	deprecatedpatch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
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
	fromK8sVersion := input.E2EConfig.MustGetVariable("FROM_K8S_VERSION")
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
	helper, err := deprecatedpatch.NewHelper(kcpObj, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.MachineTemplate.InfrastructureRef.Name = KCPNewM3MachineTemplateName
	kcpObj.Spec.Version = toK8sVersion
	kcpObj.Spec.RolloutStrategy.RollingUpdate.MaxSurge.IntVal = 0

	Expect(helper.Patch(ctx, kcpObj)).To(Succeed())

	Byf("Wait until %d Control Plane machines become running and updated with the new %s k8s version", numberOfControlplane, toK8sVersion)
	runningAndUpgraded := func(machine clusterv1beta1.Machine) bool {
		running := machine.Status.GetTypedPhase() == clusterv1beta1.MachinePhaseRunning
		upgraded := *machine.Spec.Version == toK8sVersion
		return (running && upgraded)
	}
	WaitForNumMachines(ctx, runningAndUpgraded, WaitForNumInput{
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
	bmv4helper, err := deprecatedpatch.NewHelper(&baremetalv4Pool[0], managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	baremetalv4Pool[0].Spec.PreAllocations = bmv4PoolPreallocations
	Expect(bmv4helper.Patch(ctx, &baremetalv4Pool[0])).To(Succeed())

	By("Patch provisioning IPPool with new Preallocations field and values")
	Expect(managementClusterClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: provisioningPoolName}, &provisioningPool[0])).To(Succeed())
	provhelper, err := deprecatedpatch.NewHelper(&provisioningPool[0], managementClusterClient)
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
	helper, err = deprecatedpatch.NewHelper(md, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	md.Spec.Template.Spec.InfrastructureRef.Name = newM3MachineTemplateName
	md.Spec.Template.Spec.Version = &toK8sVersion
	md.Spec.Strategy.RollingUpdate.MaxSurge.IntVal = 0
	md.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal = numberOfWorkers
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
	WaitForNumMachinesInState(ctx, clusterv1beta1.MachinePhaseRunning, WaitForNumInput{
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

	By("IP REUSE TESTS PASSED!")
}
