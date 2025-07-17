package e2e

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
)

const (
	bmhCrsFile = "bmhosts_crs.yaml"
)

var _ = Describe("When testing MachineDeployment remediation [healthcheck] [remediation] [features]", Label("healthcheck", "remediation", "features"), func() {

	BeforeEach(func() {
		osType = strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "target_cluster_logs", bootstrapClusterProxy.GetName())

		By("Check if there are existing BMHs")
		clusterClient := bootstrapClusterProxy.GetClient()
		bmhList, err := GetAllBmhs(ctx, clusterClient, namespace)
		Expect(err).ToNot(HaveOccurred(), "Could not list BMHs")

		if len(bmhList) > 0 {
			By("Removing existing BMHs from source")
			bmhData, err := os.ReadFile(filepath.Join(workDir, bmhCrsFile))
			Expect(err).ToNot(HaveOccurred(), "BMH CRs file not found")
			kubeConfigPath := bootstrapClusterProxy.GetKubeconfigPath()
			err = KubectlDelete(ctx, kubeConfigPath, bmhData, "-n", "metal3", "--ignore-not-found")
			Expect(err).ToNot(HaveOccurred(), "Could not delete BMHs")

			By("Waiting for all the BMHs deleted")
			Eventually(func(g Gomega) {
				bmhList, err := GetAllBmhs(ctx, clusterClient, namespace)
				Expect(err).ToNot(HaveOccurred(), "Could not list BMHs")
				g.Expect(bmhList).To(BeEmpty())
			}, e2eConfig.GetIntervals(specName, "wait-bmh-deleting")...).Should(Succeed())
			By("All BMHs are deleted")
		}

	})
	capi_e2e.MachineDeploymentRemediationSpec(ctx, func() capi_e2e.MachineDeploymentRemediationSpecInput {
		return capi_e2e.MachineDeploymentRemediationSpecInput{
			E2EConfig:             e2eConfig,
			ClusterctlConfigPath:  clusterctlConfigPath,
			BootstrapClusterProxy: bootstrapClusterProxy,
			ArtifactFolder:        artifactFolder,
			SkipCleanup:           skipCleanup,
			PostNamespaceCreated:  createBMHsInNamespace,
			Flavor:                ptr.To(osType + "-md-remediation"),
		}
	})
})
