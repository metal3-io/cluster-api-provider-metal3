package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("When testing live iso [live-iso] [features]", func() {
	liveIsoTest()
})

func liveIsoTest() {
	var (
		liveISOImageURL = e2eConfig.GetVariable("LIVE_ISO_IMAGE")
	)
	BeforeEach(func() {
		validateGlobals(specName)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})
	It("Should update the BMH with live ISO", func() {
		Logf("Starting live ISO test")
		bootstrapClient := bootstrapClusterProxy.GetClient()

		By("Waiting for all BMHs to be in Available state")
		Eventually(
			func(g Gomega) {
				bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
				Expect(err).NotTo(HaveOccurred(), "Error getting BMHs")
				filtered := filterBmhsByProvisioningState(bmhs, bmov1alpha1.StateAvailable)
				g.Expect(len(filtered)).To(Equal(len(bmhs)))
				Logf("One or more BMHs are not available yet")
			}, e2eConfig.GetIntervals(specName, "wait-bmh-available")...).Should(Succeed())

		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		Expect(err).NotTo(HaveOccurred(), "Error getting BMHs")
		var isoBmh bmov1alpha1.BareMetalHost
		for _, bmh := range bmhs {
			Logf("Checking BMH %s", bmh.Name)
			// Pick the first BMH that is available and uses redfish-virtualmedia (ipmi and redfish does not support live-iso)
			if bmh.Status.Provisioning.State == bmov1alpha1.StateAvailable &&
				strings.HasPrefix(bmh.Spec.BMC.Address, "redfish-virtualmedia") {
				isoBmh = bmh
				Logf("BMH %s is in %s state", bmh.Name, bmh.Status.Provisioning.State)
				break
			}
		}

		isoBmh.Spec.Online = true
		isoBmh.Spec.Image = &bmov1alpha1.Image{
			URL:          liveISOImageURL,
			Checksum:     "",
			ChecksumType: "",
			DiskFormat:   pointer.StringPtr("live-iso"),
		}
		Expect(bootstrapClient.Update(ctx, &isoBmh)).NotTo(HaveOccurred())
		isoBmhName := isoBmh.Name

		By("Waiting for live ISO image booted host to be in provisioned state")
		Eventually(func(g Gomega) {
			g.Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: isoBmhName}, &isoBmh)).To(Succeed())
			g.Expect(isoBmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateProvisioned), fmt.Sprintf("BMH %s is not in provisioned state", isoBmh.Name))
			Logf("BMH %s is in %s state", isoBmh.Name, isoBmh.Status.Provisioning.State)
		}, e2eConfig.GetIntervals(specName, "wait-bmh-provisioned")...).Should(Succeed())

		vmName := bmhToVMName(isoBmh)
		serialLogFile := fmt.Sprintf("/var/log/libvirt/qemu/%s-serial0.log", vmName)

		By("Reading serial logs to verify the node was booted from live ISO image")
		Eventually(func(g Gomega) {
			cmd := fmt.Sprintf("sudo cat %s | grep '#  Welcome'", serialLogFile)
			output, err := exec.Command("/bin/sh", "-c", cmd).Output()
			g.Expect(err).To(BeNil())
			g.Expect(output).ToNot(BeNil(), fmt.Sprintf("Failed to read serial logs from %s", serialLogFile))
		}, e2eConfig.GetIntervals(specName, "wait-job")...).Should(Succeed())
		By("LIVE ISO TEST PASSED!")
	})

	AfterEach(func() {
		By("Deprovisioning live ISO image booted BMH")

		bootstrapClient := bootstrapClusterProxy.GetClient()

		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		Expect(err).NotTo(HaveOccurred(), "Error getting all BMHs")
		bmhsLen := len(bmhs)

		for _, bmh := range bmhs {
			Logf("Checking BMH %s", bmh.Name)
			if bmh.Status.Provisioning.State == bmov1alpha1.StateProvisioned {
				Logf("live ISO image booted BMH found %s", bmh.Name)
				bmh.Spec.Online = false
				bmh.Spec.Image = nil
				Expect(bootstrapClient.Update(ctx, &bmh)).NotTo(HaveOccurred())
			}
		}
		By("Waiting for deprovisioned live ISO image booted BMH to be available")
		Eventually(
			func(g Gomega) {
				bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
				Expect(err).NotTo(HaveOccurred(), "Error getting BMHs")
				filtered := filterBmhsByProvisioningState(bmhs, bmov1alpha1.StateAvailable)
				g.Expect(len(filtered)).To(Equal(bmhsLen))
			}, e2eConfig.GetIntervals(specName, "wait-bmh-available")...).Should(Succeed())
		By("live ISO image booted BMH deprovisioned successfully")
	})
}
