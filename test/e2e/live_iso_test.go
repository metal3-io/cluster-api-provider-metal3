package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("When testing live iso [live-iso] [features]", func() {
	liveIsoTest()
})

func liveIsoTest() {
	BeforeEach(func() {
		validateGlobals(specName)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})
	It("Should update the BMH with live ISO", func() {
		liveISOImageURL := e2eConfig.GetVariable("LIVE_ISO_IMAGE")
		Logf("Starting live ISO test")
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
			DiskFormat:   pointer.String("live-iso"),
		}
		Expect(bootstrapClient.Update(ctx, &isoBmh)).NotTo(HaveOccurred())
		isoBmhName := isoBmh.Name

		By("Waiting for live ISO image booted host to be in provisioned state")
		Eventually(func(g Gomega) {
			g.Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: isoBmhName}, &isoBmh)).To(Succeed())
			g.Expect(isoBmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateProvisioned), fmt.Sprintf("BMH %s is not in provisioned state", isoBmh.Name))
			Logf("BMH %s is in %s state", isoBmh.Name, isoBmh.Status.Provisioning.State)
		}, e2eConfig.GetIntervals(specName, "wait-bmh-provisioned")...).Should(Succeed())
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))

		vmName := BmhToVMName(isoBmh)
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
		// Abort the test in case of failure and keepTestEnv is true during keep VM trigger
		if CurrentSpecReport().Failed() {
			if keepTestEnv {
				AbortSuite("e2e test aborted and skip cleaning the VM", 4)
			}
		}
		By("Deprovisioning live ISO image booted BMH")

		bootstrapClient := bootstrapClusterProxy.GetClient()

		bmhs, err := GetAllBmhs(ctx, bootstrapClient, namespace)
		Expect(err).NotTo(HaveOccurred(), "Error getting all BMHs")

		for _, bmh := range bmhs {
			bmh := bmh // for gosec G601
			Logf("Checking BMH %s", bmh.Name)
			if bmh.Status.Provisioning.State == bmov1alpha1.StateProvisioned {
				Logf("live ISO image booted BMH found %s", bmh.Name)
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
