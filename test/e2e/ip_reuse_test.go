package e2e

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("When testing ip reuse [ip-reuse]", func() {

	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})
	It("Should create a cluster and upgrade workers then verify ip allocation reuse", func() {
		IPReuse(ctx, func() IPReuseInput {
			targetCluster, _ = createTargetCluster(e2eConfig.GetVariable("FROM_K8S_VERSION"))
			return IPReuseInput{
				E2EConfig:             e2eConfig,
				BootstrapClusterProxy: bootstrapClusterProxy,
				TargetCluster:         targetCluster,
				SpecName:              specName,
				ClusterName:           clusterName,
				Namespace:             namespace,
			}
		})
	})
})
