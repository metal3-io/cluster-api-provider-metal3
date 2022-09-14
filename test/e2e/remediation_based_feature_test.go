package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type vmState string

const (
	running vmState = "running"
	paused  vmState = "paused"
	shutoff vmState = "shutoff"
	other   vmState = "other"
)

const (
	rebootAnnotation    = "reboot.metal3.io"
	poweroffAnnotation  = "reboot.metal3.io/poweroff"
	unhealthyAnnotation = "capi.metal3.io/unhealthy"
)

const defaultNamespace = "default"

var _ = Describe("Testing nodes remediation [remediation]", func() {

	var (
		ctx                 = context.TODO()
		specName            = "metal3"
		namespace           = "metal3"
		clusterName         = "test1"
		clusterctlLogFolder string
	)

	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	It("Should create a cluster and and run remediation based tests", func() {
		By("Creating target cluster")
		targetCluster = createTargetCluster()

		// Run Metal3Remediation test first, doesn't work after remediation...
		// By("Running Metal3Remediation tests")
		// metal3remediation()

		By("Running remediation tests")
		remediation()
	})

	AfterEach(func() {
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})

})
