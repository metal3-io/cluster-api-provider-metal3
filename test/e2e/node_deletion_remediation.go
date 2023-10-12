package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeDeletionRemediation struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
}

/*
 * Node Deletion Remediation Test
 *
 * This test evaluates node deletion in reboot remediation feature added to CAPM3 Remediation Controller.
 * issue #392: Reboot remediation is incomplete
 * PR #668: Fix reboot remediation by adding node deletion
 * This test evaluates the reboot remediation strategy with an enhancement of node deletion in the CAPM3 (Cluster API Provider for Metal3) Remediation Controller.
 *
 * Tested Feature:
 * - Delete Node in Reboot Remediation
 *
 * Workflow:
 * 1. Retrieve the Metal3Machines associated with the worker nodes in the cluster.
 * 2. Identify the target worker machine node its associated BMH object corresponding to the Metal3Machine.
 * 3. Create a Metal3Remediation resource, specifying the remediation strategy as "Reboot" with a retry limit and timeout.
 * 4. Wait for the VM (Virtual Machine) associated with target BMH to power off.
 * 5. Wait for the target worker node to be deleted from the cluster.
 * 6. Wait for the VMs to power on.
 * 7. Verify that the target worker node becomes ready.
 * 8. Verify that the Metal3Remediation resource is successfully delete
 *
 * Metal3Remediation test ensures that Metal3 Remediation Controller can effectively remediate worker nodes by orchestrating
 * the reboot process and validating the successful recovery of the nodes. It helps ensure the stability and
 * resiliency of the cluster by allowing workloads to be seamlessly migrated from unhealthy nodes to healthy node
 */

func nodeDeletionRemediation(ctx context.Context, inputGetter func() NodeDeletionRemediation) {
	Logf("Starting node deletion remediation tests")
	input := inputGetter()
	bootstrapClient := input.BootstrapClusterProxy.GetClient()
	targetClient := input.TargetCluster.GetClient()

	_, workerM3Machines := GetMetal3Machines(ctx, bootstrapClient, input.ClusterName, input.Namespace)
	Expect(len(workerM3Machines)).To(BeNumerically(">", 0))

	getBmhFromM3Machine := func(m3Machine infrav1.Metal3Machine) (result bmov1alpha1.BareMetalHost) {
		Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: Metal3MachineToBmhName(m3Machine)}, &result)).To(Succeed())
		return result
	}

	workerM3Machine := workerM3Machines[0]
	workerBmh := getBmhFromM3Machine(workerM3Machine)

	workerMachineName, err := Metal3MachineToMachineName(workerM3Machine)
	Expect(err).ToNot(HaveOccurred())
	workerMachine := GetMachine(ctx, bootstrapClient, client.ObjectKey{Namespace: input.Namespace, Name: workerMachineName})
	workerNodeName := workerMachineName
	vmName := BmhToVMName(workerBmh)

	By("Creating a Metal3Remediation resource")
	timeout := metav1.Duration{Duration: 30 * time.Minute}
	m3Remediation := &infrav1.Metal3Remediation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workerNodeName,
			Namespace: input.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         clusterv1.GroupVersion.String(),
					Kind:               "Machine",
					Name:               workerMachineName,
					UID:                workerMachine.GetUID(),
					Controller:         nil,
					BlockOwnerDeletion: nil,
				},
			},
		},
		Spec: infrav1.Metal3RemediationSpec{
			Strategy: &infrav1.RemediationStrategy{
				Type:       "Reboot",
				RetryLimit: 1,
				Timeout:    &timeout,
			},
		},
		Status: infrav1.Metal3RemediationStatus{},
	}
	Expect(bootstrapClient.Create(ctx, m3Remediation)).To(Succeed(), "should create Metal3Remediation CR")

	By("Waiting for VM power off")
	waitForVmsState([]string{vmName}, shutoff, input.SpecName, input.E2EConfig.GetIntervals(input.SpecName, "wait-vm-state")...)

	By("Waiting for node deletion")
	interval := input.E2EConfig.GetIntervals(input.SpecName, "wait-vm-state")
	waitForNodeDeletion(ctx, targetClient, workerNodeName, interval...)

	By("Waiting for VM power on")
	waitForVmsState([]string{vmName}, running, input.SpecName, input.E2EConfig.GetIntervals(input.SpecName, "wait-vm-state")...)

	By("Waiting for node ready")
	waitForNodeStatus(ctx, targetClient, client.ObjectKey{Name: workerNodeName}, corev1.ConditionTrue, input.SpecName, input.E2EConfig.GetIntervals(input.SpecName, "wait-vm-state")...)

	By("Deleting Metal3Remediation CR")
	Expect(bootstrapClient.Delete(ctx, m3Remediation)).To(Succeed(), "should delete Metal3Remediation CR")

	By("Make sure Metal3Remediation CR was actually deleted (finalizer is removed)")
	Eventually(func() bool {
		err = bootstrapClient.Get(ctx, client.ObjectKeyFromObject(m3Remediation), m3Remediation)
		return apierrors.IsNotFound(err)
	}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "Metal3Remediation should have been deleted")

	By("NODE DELETION TESTS PASSED!")
}

func waitForNodeDeletion(ctx context.Context, cl client.Client, name string, intervals ...interface{}) {
	Byf("Waiting for Node '%s' to be removed", name)
	Eventually(
		func() bool {
			node := &corev1.Node{}
			err := cl.Get(ctx, client.ObjectKey{Name: name}, node)
			return apierrors.IsNotFound(err)
		}, intervals...).Should(BeTrue())
}
