package e2e

import (
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	inspectAnnotation = "inspect.metal3.io"
)

func inspection() {
	Logf("Starting inspection tests")

	var (
		numberOfWorkers       = int(*e2eConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
		numberOfAvailableBMHs = 2 * numberOfWorkers
	)

	bootstrapClient := bootstrapClusterProxy.GetClient()

	Logf("Request inspection for all Available BMHs via API")
	availableBMHList := bmov1alpha1.BareMetalHostList{}
	Expect(bootstrapClient.List(ctx, &availableBMHList, client.InNamespace(namespace))).To(Succeed())
	Logf("Request inspection for all Available BMHs via API")
	for _, bmh := range availableBMHList.Items {
		if bmh.Status.Provisioning.State == bmov1alpha1.StateAvailable {
			annotateBmh(ctx, bootstrapClient, bmh, inspectAnnotation, pointer.String(""))
		}
	}

	Byf("Waiting for %d BMHs to be in Inspecting state", numberOfAvailableBMHs)
	Eventually(func(g Gomega) {
		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		g.Expect(err).NotTo(HaveOccurred())
		inspectingBMHs := filterBmhsByProvisioningState(bmhs, bmov1alpha1.StateInspecting)
		g.Expect(inspectingBMHs).To(HaveLen(numberOfAvailableBMHs))
	}, e2eConfig.GetIntervals(specName, "wait-bmh-inspecting")...).Should(Succeed())

	Byf("Waiting for %d BMHs to be in Available state", numberOfAvailableBMHs)
	Eventually(func(g Gomega) {
		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		g.Expect(err).NotTo(HaveOccurred())
		availableBMHs := filterBmhsByProvisioningState(bmhs, bmov1alpha1.StateAvailable)
		g.Expect(availableBMHs).To(HaveLen(numberOfAvailableBMHs))
	}, e2eConfig.GetIntervals(specName, "wait-bmh-available")...).Should(Succeed())

	By("INSPECTION TESTS PASSED!")
}
