package e2e

import (
	"context"
	"fmt"
	"reflect"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
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
	E2EConfig         *clusterctl.E2EConfig
	ManagementCluster framework.ClusterProxy
	TargetCluster     framework.ClusterProxy
	SpecName          string
	ClusterName       string
	Namespace         string
}

func IPReuse(ctx context.Context, inputGetter func() IPReuseInput) {
	Logf("Starting IP reuse tests")
	input := inputGetter()
	targetClusterClient := input.TargetCluster.GetClient()
	managementClusterClient := input.ManagementCluster.GetClient()
	kubernetesVersion := input.E2EConfig.GetVariable("KUBERNETES_VERSION")
	upgradedK8sVersion := input.E2EConfig.GetVariable("PATCH_KUBERNETES_VERSION")

	// scale down KCP to 1
	By("Scale the controlplane down to 1")
	ScaleKubeadmControlPlane(ctx, managementClusterClient, client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName}, 1)

	// scale up MD to 3
	By("Scale the worker up to 3")
	ScaleMachineDeployment(ctx, managementClusterClient, input.ClusterName, input.Namespace, 3)

	// GetIPPools in the cluster
	baremetalv4Pool, provisioningPool := GetIPPools(ctx, targetClusterClient, input.ClusterName, input.Namespace)
	Expect(baremetalv4Pool).To(HaveLen(1))
	Expect(provisioningPool).To(HaveLen(1))
	baremetalv4PoolName := baremetalv4Pool[0].Name
	provisioningPoolName := provisioningPool[0].Name

	// Get the status indexes of the IPPools
	baremetalv4PoolNameIndexes, err := GetIPPoolIndexes(ctx, baremetalv4Pool[0], baremetalv4PoolName, targetClusterClient)
	Expect(err).NotTo(HaveOccurred())
	provisioningPoolNameIndexes, err := GetIPPoolIndexes(ctx, provisioningPool[0], provisioningPoolName, targetClusterClient)
	Expect(err).NotTo(HaveOccurred())

	// patch baremetal ippool with new allocations
	By("Patch baremetal IPPool with new allocations using Preallocations field")
	Expect(managementClusterClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName}, &baremetalv4Pool[0])).To(Succeed())
	bmv4helper, err := patch.NewHelper(&baremetalv4Pool[0], managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	baremetalv4Pool[0].Spec.PreAllocations = baremetalv4PoolNameIndexes
	Expect(bmv4helper.Patch(ctx, &baremetalv4Pool[0])).To(Succeed())

	// patch provisioning ippool with new allocations
	By("Patch provisioning IPPool with new allocations using Preallocations field")
	Expect(managementClusterClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName}, &provisioningPool[0])).To(Succeed())
	provhelper, err := patch.NewHelper(&provisioningPool[0], managementClusterClient)
	Expect(err).NotTo(HaveOccurred())
	baremetalv4Pool[0].Spec.PreAllocations = provisioningPoolNameIndexes
	Expect(provhelper.Patch(ctx, &provisioningPool[0])).To(Succeed())

	// patch MD maxUnavailable and k8s version to start a parallel upgrade
	By("Get MachineDeployment")
	machineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      managementClusterClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace,
	})
	Expect(len(machineDeployments)).To(Equal(1), "Expected exactly 1 MachineDeployment")
	md := machineDeployments[0]

	Byf("Update MachineDeployment maxUnavailable to number of replicas and k8s version from %s to %s", kubernetesVersion, upgradedK8sVersion)
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
	}`, upgradedK8sVersion))
	err = managementClusterClient.Patch(ctx, md, client.RawPatch(types.MergePatchType, patch))
	Expect(err).To(BeNil(), "Failed to patch MachineDeployment")

	// wait until 3 BMHs are in deprovisioning state
	Byf("Wait until %d BMH(s) in deprovisioning state", 3)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateDeprovisioning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  3,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-bmh-deprovisioning"),
	})

	// wait until all 4 machines are in running state
	Byf("Wait until all %d machine(s) become(s) running", 4)
	WaitForNumMachinesInState(ctx, clusterv1.MachinePhaseRunning, WaitForNumInput{
		Client:    managementClusterClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  4,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running"),
	})

	// check if same IP addresses are reused for nodes(ippool.Preallocations == ippool.Status.Allocations)
	By("Check if same IP addresses are reused for nodes")
	bmv4equal := reflect.DeepEqual(baremetalv4Pool[0].Spec.PreAllocations, baremetalv4Pool[0].Status.Allocations)
	Expect(bmv4equal).To(BeTrue(), "The same IP addreesses from baremetal IPPool were not reused for nodes")
	provequal := reflect.DeepEqual(provisioningPool[0].Spec.PreAllocations, provisioningPool[0].Status.Allocations)
	Expect(provequal).To(BeTrue(), "The same IP addreesses from provisioning IPPool were not reused for nodes")

	By("IP REUSE TESTS PASSED!")
}
