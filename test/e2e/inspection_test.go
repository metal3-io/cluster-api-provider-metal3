package e2e

import (
	bmo "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
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
	availableBMHList := bmo.BareMetalHostList{}
	Expect(bootstrapClient.List(ctx, &availableBMHList, client.InNamespace(namespace))).To(Succeed())
	Logf("Request inspection for all Available BMHs via API")
	for _, bmh := range availableBMHList.Items {
		if bmh.Status.Provisioning.State == bmo.StateAvailable {
			annotateBmh(ctx, bootstrapClient, bmh, inspectAnnotation, pointer.String(""))
		}
	}

	Byf("Waiting for %d BMHs to be in Inspecting state", numberOfAvailableBMHs)
	Eventually(func(g Gomega) error {
		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		if err != nil {
			Logf("Error: %v", err)
			return err
		}
		inspectingBMHs := filterBmhsByProvisioningState(bmhs, bmo.StateInspecting)
		if len(inspectingBMHs) != numberOfAvailableBMHs {
			return errors.Errorf("Waiting for %v BMHs to be in Inspecting state, but got %v", numberOfAvailableBMHs, len(inspectingBMHs))
		}
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-bmh-inspecting")...).Should(Succeed())

	Byf("Waiting for %d BMHs to be in Available state", numberOfAvailableBMHs)
	Eventually(func(g Gomega) error {
		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		if err != nil {
			Logf("Error: %v", err)
			return err
		}
		availableBMHs := filterBmhsByProvisioningState(bmhs, bmo.StateAvailable)
		if len(availableBMHs) != numberOfAvailableBMHs {
			return errors.Errorf("Waiting for %v BMHs to be in Available state, but got %v", numberOfAvailableBMHs, len(availableBMHs))
		}
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-bmh-available")...).Should(Succeed())

	By("INSPECTION TESTS PASSED!")
}
