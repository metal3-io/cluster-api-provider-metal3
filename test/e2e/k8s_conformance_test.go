package e2e

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
)

var _ = Describe("When testing K8S conformance [Conformance]", Label("Conformance"), func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "target_cluster_logs", bootstrapClusterProxy.GetName())

		Logf("Removing existing BMHs from source")
		bmhData, err := os.ReadFile(filepath.Join(workDir, bmhCrsFile))
		Expect(err).ToNot(HaveOccurred(), "BMH CRs file not found")
		kubeConfigPath := bootstrapClusterProxy.GetKubeconfigPath()
		err = KubectlDelete(ctx, kubeConfigPath, bmhData, "-n", "metal3")
		Expect(err).ToNot(HaveOccurred(), "Failed to delete existing BMHs")
		Logf("BMHs are removed")
	})
	// Note: This installs a cluster based on KUBERNETES_VERSION and runs conformance tests.
	capi_e2e.K8SConformanceSpec(ctx, func() capi_e2e.K8SConformanceSpecInput {
		return capi_e2e.K8SConformanceSpecInput{
			E2EConfig:             e2eConfig,
			ClusterctlConfigPath:  clusterctlConfigPath,
			BootstrapClusterProxy: bootstrapClusterProxy,
			ArtifactFolder:        artifactFolder,
			SkipCleanup:           skipCleanup,
			PostNamespaceCreated:  postNamespaceCreated,
			Flavor:                osType,
		}
	})
})
