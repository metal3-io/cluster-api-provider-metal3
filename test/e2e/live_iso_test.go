package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	bmo_e2e "github.com/metal3-io/baremetal-operator/test/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
 * The purpose of the live-iso feature in Metal3 is to allow booting a BareMetalHost with a live ISO image instead of deploying an image to the local disk using the IPA deploy ramdisk. This feature is useful in scenarios where reducing boot time for ephemeral workloads is desired, or when integrating with third-party installers distributed as a CD image.
 *
 * This test demonstrates the usage of the live-iso feature. It performs the following steps:
 *
 * 	The live ISO image URL is retrieved from the test configuration.
 * 	The list of bare metal hosts (BMHs) in the namespace is displayed.
 * 	It waits for all BMHs to be in the "Available" state.
 * 	It retrieves all BMHs and selects the first available BMH that supports the "redfish-virtualmedia" mechanism for provisioning the live image.
 * 	The selected BMH is updated with the live ISO image URL and marked as online.
 * 	It waits for the BMH to transition to the "Provisioned" state, indicating successful booting from the live ISO image.
 * 	The list of BMHs in the namespace is displayed.
 * 	Serial logs are read to verify that the node was booted from the live ISO image.
 * 	The test is considered passed.
 */

var _ = Describe("When testing live iso [live-iso] [features]", Label("live-iso", "features"), func() {
	liveIsoTest()
})

// Live iso tests provision live-iso image on host
// it lists all the bmh and selects the one supporting redfish-virtualmedia for provisioning the live image.
func liveIsoTest() {
	BeforeEach(func() {
		validateGlobals(specName)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})
	It("Should update the BMH with live ISO", func() {
		liveISOImageURL := e2eConfig.GetVariable("LIVE_ISO_IMAGE")
		bmo_e2e.Logf("Starting live ISO test")
		bootstrapClient := bootstrapClusterProxy.GetClient()
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))

		WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
			Client:    bootstrapClient,
			Options:   []client.ListOption{client.InNamespace(namespace)},
			Replicas:  numberOfAllBmh,
			Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
		})

		bmhs, err := GetAllBmhs(ctx, bootstrapClient, namespace)
		Expect(err).NotTo(HaveOccurred(), "Error getting BMHs")
		var isoBmh bmov1alpha1.BareMetalHost
		for _, bmh := range bmhs {
			bmo_e2e.Logf("Checking BMH %s", bmh.Name)
			// Pick the first BMH that is available and uses redfish-virtualmedia (ipmi and redfish does not support live-iso)
			if bmh.Status.Provisioning.State == bmov1alpha1.StateAvailable &&
				strings.HasPrefix(bmh.Spec.BMC.Address, "redfish-virtualmedia") {
				isoBmh = bmh
				bmo_e2e.Logf("BMH %s is in %s state", bmh.Name, bmh.Status.Provisioning.State)
				break
			}
		}

		isoBmh.Spec.Online = true
		isoBmh.Spec.Image = &bmov1alpha1.Image{
			URL:          liveISOImageURL,
			Checksum:     "",
			ChecksumType: "",
			DiskFormat:   ptr.To("live-iso"),
		}
		Expect(bootstrapClient.Update(ctx, &isoBmh)).NotTo(HaveOccurred())
		isoBmhName := isoBmh.Name

		By("Waiting for live ISO image booted host to be in provisioned state")
		Eventually(func(g Gomega) {
			g.Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: isoBmhName}, &isoBmh)).To(Succeed())
			g.Expect(isoBmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateProvisioned), fmt.Sprintf("BMH %s is not in provisioned state", isoBmh.Name))
			bmo_e2e.Logf("BMH %s is in %s state", isoBmh.Name, isoBmh.Status.Provisioning.State)
		}, e2eConfig.GetIntervals(specName, "wait-bmh-provisioned")...).Should(Succeed())
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))

		vmName := BmhToVMName(isoBmh)
		serialLogFile := fmt.Sprintf("/var/log/libvirt/qemu/%s-serial0.log", vmName)

		By("Reading serial logs to verify the node was booted from live ISO image")
		Eventually(func(g Gomega) {
			cmd := fmt.Sprintf("sudo cat %s | grep '#  Welcome'", serialLogFile)
			output, err := exec.Command("/bin/sh", "-c", cmd).Output()
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(output).ToNot(BeNil(), fmt.Sprintf("Failed to read serial logs from %s", serialLogFile))
		}, e2eConfig.GetIntervals(specName, "wait-job")...).Should(Succeed())
		By("LIVE ISO TEST PASSED!")
	})

	AfterEach(func() {
		By("Deprovisioning live ISO image booted BMH")

		bootstrapClient := bootstrapClusterProxy.GetClient()

		bmhs, err := GetAllBmhs(ctx, bootstrapClient, namespace)
		Expect(err).NotTo(HaveOccurred(), "Error getting all BMHs")

		for _, bmh := range bmhs {
			bmh := bmh // for gosec G601
			bmo_e2e.Logf("Checking BMH %s", bmh.Name)
			if bmh.Status.Provisioning.State == bmov1alpha1.StateProvisioned {
				bmo_e2e.Logf("live ISO image booted BMH found %s", bmh.Name)
				bmh.Spec.Online = false
				bmh.Spec.Image = nil
				Expect(bootstrapClient.Update(ctx, &bmh)).NotTo(HaveOccurred())
			}
		}
		By("Waiting for deprovisioned live ISO image booted BMH to be available")
		WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
			Client:    bootstrapClient,
			Options:   []client.ListOption{client.InNamespace(namespace)},
			Replicas:  numberOfAllBmh,
			Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
		})
		By("live ISO image booted BMH deprovisioned successfully")
	})
}
