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
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	bmhCrsFile = "bmhosts_crs.yaml"
)

var _ = Describe("When testing MachineDeployment remediation [healthcheck] [remediation] [features]", Label("healthcheck", "remediation", "features"), func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
		k8sVersion := e2eConfig.MustGetVariable("KUBERNETES_VERSION")
		imageURL, imageChecksum := EnsureImage(k8sVersion)
		os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
		os.Setenv("IMAGE_RAW_URL", imageURL)
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
	capi_e2e.MachineDeploymentRemediationSpec(ctx, func() capi_e2e.MachineDeploymentRemediationSpecInput {
		return capi_e2e.MachineDeploymentRemediationSpecInput{
			E2EConfig:             e2eConfig,
			ClusterctlConfigPath:  clusterctlConfigPath,
			BootstrapClusterProxy: bootstrapClusterProxy,
			ArtifactFolder:        artifactFolder,
			SkipCleanup:           skipCleanup,
			PostNamespaceCreated:  postNamespaceCreated,
			Flavor:                ptr.To(fmt.Sprintf("%s-md-remediation", osType)),
		}
	})
	AfterEach(func() {
		ListBareMetalHosts(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		ListMetal3Machines(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		ListMachines(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(namespace))
		// Recreate bmh that was used in capi namespace in metal3
		//#nosec G204 -- We need to pass in the file name here.
		cmd := exec.Command("bash", "-c", "kubectl apply -f bmhosts_crs.yaml  -n metal3")
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		Logf("Applying bmh to metal3 namespace : \n %v", string(output))
		Expect(err).ToNot(HaveOccurred())
		// wait for all bmh to become available
		bootstrapClient := bootstrapClusterProxy.GetClient()
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))
		WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
			Client:    bootstrapClient,
			Options:   []client.ListOption{client.InNamespace(namespace)},
			Replicas:  4,
			Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
		})
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))
	})
})
