package e2e

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
)

var _ = Describe("When testing MachineDeployment rolling upgrades [capi-md-tests]", Label("capi-md-tests"), func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "target_cluster_logs", bootstrapClusterProxy.GetName())
	})
	capi_e2e.MachineDeploymentRolloutSpec(ctx, func() capi_e2e.MachineDeploymentRolloutSpecInput {
		return capi_e2e.MachineDeploymentRolloutSpecInput{
			E2EConfig:             e2eConfig,
			ClusterctlConfigPath:  clusterctlConfigPath,
			BootstrapClusterProxy: bootstrapClusterProxy,
			ArtifactFolder:        artifactFolder,
			SkipCleanup:           skipCleanup,
			Flavor:                osType,
			PostNamespaceCreated:  createBMHsInNamespace,
		}
	})
})
