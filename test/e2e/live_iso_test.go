package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("When testing live iso [live-iso]", func() {
	liveIsoTest()
})

func liveIsoTest() {
	Logf("Starting live ISO test")
	var (
		liveISOImageURL = e2eConfig.GetVariable("LIVE_ISO_IMAGE")
	)
	BeforeEach(func() {
		validateGlobals(specName)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})
	It("Should update the bmh with live iso", func() {
		bootstrapClient := bootstrapClusterProxy.GetClient()

		bmhs, err := getAllBmhs(ctx, bootstrapClient, namespace, specName)
		Expect(err).NotTo(HaveOccurred(), "Error getting BMHs")

		isoBmh := bmhs[0]
		Expect(isoBmh).ToNot(BeNil())

		isoBmh.Spec.Image = &bmov1alpha1.Image{
			URL:          liveISOImageURL,
			Checksum:     "",
			ChecksumType: "",
			DiskFormat:   pointer.StringPtr("live-iso"),
		}

		Expect(bootstrapClient.Update(ctx, &isoBmh)).NotTo(HaveOccurred())

		By("waiting for live ISO image booted host to be in provisioned state")
		Eventually(func(g Gomega) {
			g.Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: bmhs[0].Name}, &isoBmh)).To(Succeed())
			g.Expect(isoBmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateProvisioned))
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
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})
}
