package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func metal3remediation() {
	Logf("Starting metal3 remediation tests")

	bootstrapClient := bootstrapClusterProxy.GetClient()
	targetClient := targetCluster.GetClient()

	_, workerM3Machines := getMetal3Machines(ctx, bootstrapClient, clusterName, namespace)
	Expect(len(workerM3Machines)).To(BeNumerically(">", 0))

	getBmhFromM3Machine := func(m3Machine infrav1.Metal3Machine) (result bmov1alpha1.BareMetalHost) {
		Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: metal3MachineToBmhName(m3Machine)}, &result)).To(Succeed())
		return result
	}

	workerM3Machine := workerM3Machines[0]
	workerBmh := getBmhFromM3Machine(workerM3Machine)

	workerMachineName, err := metal3MachineToMachineName(workerM3Machine)
	Expect(err).ToNot(HaveOccurred())
	workerMachine := getMachine(ctx, bootstrapClient, client.ObjectKey{Namespace: namespace, Name: workerMachineName})
	workerNodeName := workerMachineName
	vmName := bmhToVMName(workerBmh)

	By("Creating a Metal3Remediation resource")
	timeout := metav1.Duration{Duration: 30 * time.Minute}
	m3Remediation := &infrav1.Metal3Remediation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workerNodeName,
			Namespace: namespace,
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
	waitForVmsState([]string{vmName}, shutoff, specName)

	By("Waiting for node deletion")
	waitForNodeDeletion(ctx, targetClient, workerNodeName)

	By("Waiting for VM power on")
	waitForVmsState([]string{vmName}, running, specName)

	By("Waiting for node ready")
	waitForNodeStatus(ctx, targetClient, client.ObjectKey{Name: workerNodeName}, corev1.ConditionTrue, specName)

	By("Deleting Metal3Remediation CR")
	Expect(bootstrapClient.Delete(ctx, m3Remediation)).To(Succeed(), "should delete Metal3Remediation CR")

	By("Make sure Metal3Remediation CR was actually deleted (finalizer is removed)")
	Eventually(func() bool {
		err = bootstrapClient.Get(ctx, client.ObjectKeyFromObject(m3Remediation), m3Remediation)
		return apierrors.IsNotFound(err)
	}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "Metal3Remediation should have been deleted")

	By("METAL3REMEDIATION TESTS PASSED!")
}

func waitForNodeDeletion(ctx context.Context, cl client.Client, name string) {
	Byf("Waiting for Node '%s' to be removed", name)
	Eventually(
		func() bool {
			node := &corev1.Node{}
			err := cl.Get(ctx, client.ObjectKey{Name: name}, node)
			return apierrors.IsNotFound(err)
		},
		e2eConfig.GetIntervals(specName, "wait-vm-state")...,
	).Should(BeTrue())
}
