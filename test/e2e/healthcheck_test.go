package e2e

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

/*
 * Healthcheck Test:
 * - For both worker and controlplane machines:
 * - Create and deploy machinehealthcheck.
 * - Stop kubelet on the machine.
 * - Wait for the healthcheck to notice the unhealthy machine.
 * - Wait for the remediation request to be created.
 * - Wait for the machine to appear as healthy again.
 * - Wait for the remediation request to be deleted.
 **/

var _ = Describe("When testing healthcheck [healthcheck]", func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	It("Should remediate unhealthy machines", func() {
		By("Fetching cluster configuration")
		k8sVersion := e2eConfig.GetVariable("KUBERNETES_VERSION")
		By("Provision Workload cluster")
		targetCluster, _ = createTargetCluster(k8sVersion)

		healthcheck(ctx, func() HealthCheckInput {
			return HealthCheckInput{
				BootstrapClusterProxy: bootstrapClusterProxy,
				ClusterName:           clusterName,
				Namespace:             namespace,
			}
		})
	})

	AfterEach(func() {
		DumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})
})
