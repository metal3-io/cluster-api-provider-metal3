package e2e

import (
	"context"
	"fmt"
	"reflect"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	bmo_e2e "github.com/metal3-io/baremetal-operator/test/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
	bmo_e2e.Logf("Starting IP reuse tests")
	input := inputGetter()
	targetClusterClient := input.TargetCluster.GetClient()
	managementClusterClient := input.BootstrapClusterProxy.GetClient()
	fromK8sVersion := input.E2EConfig.GetVariable("FROM_K8S_VERSION")
	kubernetesVersion := input.E2EConfig.GetVariable("KUBERNETES_VERSION")

	// scale down KCP to 1
	By("Scale the controlplane down to 1")
	ScaleKubeadmControlPlane(ctx, managementClusterClient, client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName}, 1)
	Byf("Wait until controlplane is scaled down and %d BMHs are Available", 2)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  2,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-cp-available"),
	})

	ListBareMetalHosts(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMetal3Machines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListMachines(ctx, managementClusterClient, client.InNamespace(input.Namespace))
	ListNodes(ctx, targetClusterClient)

	// scale up MD to 3
	By("Scale the worker up to 3")
	ScaleMachineDeployment(ctx, managementClusterClient, input.ClusterName, input.Namespace, 3)
	By("Waiting for one BMH to become provisioning")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  2,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	By("Waiting for all BMHs to become provisioned")
	WaitForNumBmhInState(ctx, bmov1alpha1.StateProvisioned, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  4,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	By("Waiting for all Machines to be Running")
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  4,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation"),
	})

	By("Get the IPPools in the cluster")
	baremetalv4Pool, provisioningPool := GetIPPools(ctx, managementClusterClient, input.ClusterName, input.Namespace)
	Expect(baremetalv4Pool).To(HaveLen(1))
	Expect(provisioningPool).To(HaveLen(1))
	baremetalv4PoolName := baremetalv4Pool[0].Name
	provisioningPoolName := provisioningPool[0].Name

	By("Get new values of spec.Preallocations field for baremetal IPPool")
	bmv4PoolPreallocations, err := GenerateIPPoolPreallocations(ctx, baremetalv4Pool[0], baremetalv4PoolName, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	bmo_e2e.Logf("new values of spec.Preallocations field for baremetal IPPool is: %s", bmv4PoolPreallocations)
	By("Get new values of spec.Preallocations field for provisioning IPPool")
	provPoolPreallocations, err := GenerateIPPoolPreallocations(ctx, provisioningPool[0], provisioningPoolName, managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	bmo_e2e.Logf("new values of spec.Preallocations field for provisioning IPPool is: %s", provPoolPreallocations)

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

	Byf("Update MachineDeployment maxUnavailable to number of workers and k8s version from %s to %s", fromK8sVersion, kubernetesVersion)
	patch := []byte(fmt.Sprintf(`{
		"spec": {
			"strategy": {
				"rollingUpdate": {
					"maxSurge": 0,
					"maxUnavailable": 3
				}
			},
			"template": {
				"spec": {
					"version": "%s"
				}
			}
		}
	}`, kubernetesVersion))
	err = managementClusterClient.Patch(ctx, md, client.RawPatch(types.MergePatchType, patch))
	Expect(err).ToNot(HaveOccurred(), "Failed to patch MachineDeployment")

	Byf("Wait until %d BMH(s) in deprovisioning state", 3)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateDeprovisioning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  3,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-deprovisioning"),
	})

	Byf("Wait until all %d machine(s) become(s) running", 4)
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  4,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	By("Get the IPPools in the cluster")
	baremetalv4Pool, provisioningPool = GetIPPools(ctx, managementClusterClient, input.ClusterName, input.Namespace)
	Expect(baremetalv4Pool).To(HaveLen(1))
	Expect(provisioningPool).To(HaveLen(1))

	By("Check if same IP addresses are reused for nodes")
	bmo_e2e.Logf("baremetalv4Pool[0].Spec.PreAllocations: %v", baremetalv4Pool[0].Spec.PreAllocations)
	bmo_e2e.Logf("baremetalv4Pool[0].Status.Allocations: %v", baremetalv4Pool[0].Status.Allocations)
	bmv4equal := reflect.DeepEqual(baremetalv4Pool[0].Spec.PreAllocations, baremetalv4Pool[0].Status.Allocations)
	Expect(bmv4equal).To(BeTrue(), "The same IP addreesses from baremetal IPPool were not reused for nodes")
	bmo_e2e.Logf("provisioningPool[0].Spec.PreAllocations: %v", provisioningPool[0].Spec.PreAllocations)
	bmo_e2e.Logf("provisioningPool[0].Status.Allocations: %v", provisioningPool[0].Status.Allocations)
	provequal := reflect.DeepEqual(provisioningPool[0].Spec.PreAllocations, provisioningPool[0].Status.Allocations)
	Expect(provequal).To(BeTrue(), "The same IP addreesses from provisioning IPPool were not reused for nodes")

	By("IP REUSE TESTS PASSED!")
}
