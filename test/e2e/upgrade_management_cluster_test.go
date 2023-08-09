package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	framework "sigs.k8s.io/cluster-api/test/framework"
)

const workDir = "/opt/metal3-dev-env/"

var _ = Describe("When testing cluster upgrade v1alpha5 > current [upgrade]", func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
		imageURL, imageChecksum := EnsureImage(e2eConfig.GetVariable("INIT_WITH_KUBERNETES_VERSION"))
		os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
		os.Setenv("IMAGE_RAW_URL", imageURL)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	isPreRelease := strings.Contains(os.Getenv("CAPI_TO_RELEASE"), "-") || strings.Contains(os.Getenv("CAPM3_TO_RELEASE"), "-")

	if isPreRelease {
		capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
			return capi_e2e.ClusterctlUpgradeSpecInput{
				E2EConfig:                   e2eConfig,
				ClusterctlConfigPath:        clusterctlConfigPath,
				BootstrapClusterProxy:       bootstrapClusterProxy,
				ArtifactFolder:              artifactFolder,
				SkipCleanup:                 skipCleanup,
				InitWithProvidersContract:   "v1alpha4",
				CoreProvider:                fmt.Sprintf("capi-system/cluster-api:%s", os.Getenv("CAPI_TO_RELEASE")),
				BootstrapProviders:          []string{fmt.Sprintf("capi-kubeadm-bootstrap-system/kubeadm:%s", os.Getenv("CAPI_TO_RELEASE"))},
				ControlPlaneProviders:       []string{fmt.Sprintf("capi-kubeadm-control-plane-system/kubeadm:%s", os.Getenv("CAPI_TO_RELEASE"))},
				InfrastructureProviders:     []string{fmt.Sprintf("capm3-system/metal3:%s", os.Getenv("CAPM3_TO_RELEASE"))},
				InitWithBinary:              e2eConfig.GetVariable("INIT_WITH_BINARY"),
				PreInit:                     preInitFunc,
				PreWaitForCluster:           preWaitForCluster,
				PreUpgrade:                  preUpgrade,
				PreCleanupManagementCluster: preCleanupManagementCluster,
				MgmtFlavor:                  osType,
				WorkloadFlavor:              osType,
			}
		})
	} else {
		capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
			return capi_e2e.ClusterctlUpgradeSpecInput{
				E2EConfig:                   e2eConfig,
				ClusterctlConfigPath:        clusterctlConfigPath,
				BootstrapClusterProxy:       bootstrapClusterProxy,
				ArtifactFolder:              artifactFolder,
				SkipCleanup:                 skipCleanup,
				InitWithProvidersContract:   "v1alpha4",
				InitWithBinary:              e2eConfig.GetVariable("INIT_WITH_BINARY"),
				PreInit:                     preInitFunc,
				PreWaitForCluster:           preWaitForCluster,
				PreUpgrade:                  preUpgrade,
				PreCleanupManagementCluster: preCleanupManagementCluster,
				MgmtFlavor:                  osType,
				WorkloadFlavor:              osType,
			}
		})
	}
})

// preWaitForCluster is a hook function that should be called from ClusterctlUpgradeSpec before waiting for the cluster to spin up
// it creates the needed bmhs in namespace hosting the cluster and export the providerID format for v1alpha5.
func preWaitForCluster(clusterProxy framework.ClusterProxy, clusterNamespace string, clusterName string) {
	// Install the split-yaml tool to split our bmhosts_crs.yaml file.
	installSplitYAML := func() {
		cmd := exec.Command("bash", "-c", "kubectl krew update; kubectl krew install split-yaml")
		output, err := cmd.CombinedOutput()
		Logf("Download split-yaml:\n %v", string(output))
		Expect(err).To(BeNil())
	}

	// Split the <fileName> file into <resourceName>.yaml files based on the name of each resource and return list of the filename created
	// so we can apply limited number on BMH in each namespace.
	splitYAMLFile := func(fileName string, filePath string) []string {
		//#nosec G204 -- This is just a test. We are not concerned with command injection
		cmd := exec.Command("bash", "-c", fmt.Sprintf("cat %s | kubectl split-yaml -t {{.name}}.yaml -p.", fileName))
		cmd.Dir = filePath
		output, err := cmd.CombinedOutput()
		Logf("splitting %s%s file into multiple files: \n %v", filePath, fileName, string(output))
		Expect(err).To(BeNil())
		return strings.Split(string(output), "\n")
	}

	// Create the BMHs needed in the hosting namespace.
	createBMH := func(clusterProxy framework.ClusterProxy, clusterNamespace string, clusterName string) {
		installSplitYAML()
		splitFiles := splitYAMLFile("bmhosts_crs.yaml", workDir)
		// Check which from which cluster creation this call is coming
		// if isBootstrapProxy==true then this call when creating the management else we are creating the workload.
		isBootstrapProxy := !strings.HasPrefix(clusterProxy.GetName(), "clusterctl-upgrade")
		if isBootstrapProxy {
			// remove existing bmh from source and apply first 2 in target
			Logf("remove existing bmh from source")
			cmd := exec.Command("bash", "-c", "kubectl delete -f bmhosts_crs.yaml  -n metal3")
			cmd.Dir = workDir
			output, err := cmd.CombinedOutput()
			Logf("Remove existing bmhs:\n %v", string(output))
			Expect(err).To(BeNil())

			// Apply secrets and bmhs for [node_0 and node_1] in the management cluster to host the target management cluster
			for i := 0; i < 4; i++ {
				resource, err := os.ReadFile(filepath.Join(workDir, splitFiles[i]))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(clusterProxy.Apply(ctx, resource, []string{"-n", clusterNamespace}...)).ShouldNot(HaveOccurred())
			}
		} else {
			// Apply secrets and bmhs for [node_2, node_3 and node_4] in the management cluster to host workload cluster
			for i := 4; i < 10; i++ {
				resource, err := os.ReadFile(filepath.Join(workDir, splitFiles[i]))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(clusterProxy.Apply(ctx, resource, []string{"-n", clusterNamespace}...)).ShouldNot(HaveOccurred())
			}
		}
	}

	// Create bmhs in the in the namespace that will host the new cluster
	createBMH(clusterProxy, clusterNamespace, clusterName)
}

// preInitFunc hook function that should be called from ClusterctlUpgradeSpec before init the management cluster
// it installs certManager, BMO and Ironic and overrides the default IPs for the workload cluster.
func preInitFunc(clusterProxy framework.ClusterProxy) {
	installCertManager := func(clusterProxy framework.ClusterProxy) {
		certManagerLink := fmt.Sprintf("https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml", e2eConfig.GetVariable("CERT_MANAGER_RELEASE"))
		err := DownloadFile("/tmp/certManager.yaml", certManagerLink)
		Expect(err).To(BeNil(), "Unable to download certmanager manifest")
		certManagerYaml, err := os.ReadFile("/tmp/certManager.yaml")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(clusterProxy.Apply(ctx, certManagerYaml)).ShouldNot(HaveOccurred())
	}

	// install certmanager
	installCertManager(clusterProxy)
	// Remove ironic
	By("Remove Ironic containers from the source cluster")
	ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
	isIronicDeployment := true
	if ephemeralCluster == Kind {
		isIronicDeployment = false
	}
	removeIronic(ctx, func() RemoveIronicInput {
		return RemoveIronicInput{
			ManagementCluster: bootstrapClusterProxy,
			IsDeployment:      isIronicDeployment,
			Namespace:         e2eConfig.GetVariable(ironicNamespace),
			NamePrefix:        e2eConfig.GetVariable(NamePrefix),
		}
	})
	// install bmo
	By("Install BMO")
	installIronicBMO(func() installIronicBMOInput {
		return installIronicBMOInput{
			ManagementCluster:          clusterProxy,
			BMOPath:                    e2eConfig.GetVariable(bmoPath),
			deployIronic:               false,
			deployBMO:                  true,
			deployIronicTLSSetup:       getBool(e2eConfig.GetVariable(ironicTLSSetup)),
			deployIronicBasicAuth:      getBool(e2eConfig.GetVariable(ironicBasicAuth)),
			deployIronicKeepalived:     getBool(e2eConfig.GetVariable(ironicKeepalived)),
			deployIronicMariadb:        getBool(e2eConfig.GetVariable(ironicMariadb)),
			NamePrefix:                 e2eConfig.GetVariable(NamePrefix),
			RestartContainerCertUpdate: getBool(e2eConfig.GetVariable(restartContainerCertUpdate)),
		}
	})

	// install ironic
	By("Install Ironic in the target cluster")
	installIronicBMO(func() installIronicBMOInput {
		return installIronicBMOInput{
			ManagementCluster:          clusterProxy,
			BMOPath:                    e2eConfig.GetVariable(bmoPath),
			deployIronic:               true,
			deployBMO:                  false,
			deployIronicTLSSetup:       getBool(e2eConfig.GetVariable(ironicTLSSetup)),
			deployIronicBasicAuth:      getBool(e2eConfig.GetVariable(ironicBasicAuth)),
			deployIronicKeepalived:     getBool(e2eConfig.GetVariable(ironicKeepalived)),
			deployIronicMariadb:        getBool(e2eConfig.GetVariable(ironicMariadb)),
			NamePrefix:                 e2eConfig.GetVariable(NamePrefix),
			RestartContainerCertUpdate: getBool(e2eConfig.GetVariable(restartContainerCertUpdate)),
		}
	})

	// Export capi/capm3 versions
	os.Setenv("CAPI_VERSION", "v1alpha4")
	os.Setenv("CAPM3_VERSION", "v1alpha5")

	// These exports bellow we need them after applying the management cluster template and before
	// applying the workload. if exported before it will break creating the management because it uses v1beta1 templates and default IPs.
	// override the provider id format
	os.Setenv("PROVIDER_ID_FORMAT", "metal3://{{ ds.meta_data.uuid }}")
	// override default IPs for the workload cluster
	os.Setenv("CLUSTER_APIENDPOINT_HOST", "192.168.111.250")
	os.Setenv("IPAM_EXTERNALV4_POOL_RANGE_START", "192.168.111.201")
	os.Setenv("IPAM_EXTERNALV4_POOL_RANGE_END", "192.168.111.240")
	os.Setenv("IPAM_PROVISIONING_POOL_RANGE_START", "172.22.0.201")
	os.Setenv("IPAM_PROVISIONING_POOL_RANGE_END", "172.22.0.240")
}

// preUpgrade hook should be called from ClusterctlUpgradeSpec before upgrading the management cluster
// it upgrades Ironic and BMO before upgrading the providers.
func preUpgrade(clusterProxy framework.ClusterProxy) {
	upgradeIronic(ctx, func() upgradeIronicInput {
		return upgradeIronicInput{
			E2EConfig:         e2eConfig,
			ManagementCluster: clusterProxy,
			SpecName:          specName,
		}
	})
	upgradeBMO(ctx, func() upgradeBMOInput {
		return upgradeBMOInput{
			E2EConfig:         e2eConfig,
			ManagementCluster: clusterProxy,
			SpecName:          specName,
		}
	})
}

// preCleanupManagementCluster hook should be called from ClusterctlUpgradeSpec before cleaning the target management cluster
// it moves back Ironic to the bootstrap cluster.
func preCleanupManagementCluster(clusterProxy framework.ClusterProxy) {
	// Reinstall ironic
	reInstallIronic := func() {
		By("Reinstate Ironic containers and BMH")
		ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
		if ephemeralCluster == Kind {
			By("Install Ironic in the source cluster as containers")
			bmoPath := e2eConfig.GetVariable("BMOPATH")
			ironicCommand := bmoPath + "/tools/run_local_ironic.sh"
			//#nosec G204 -- This is just a test. We are not concerned with command injection
			cmd := exec.Command("sh", "-c", "export CONTAINER_RUNTIME=docker; "+ironicCommand)
			stdoutStderr, err := cmd.CombinedOutput()
			fmt.Printf("%s\n", stdoutStderr)
			Expect(err).To(BeNil(), "Cannot run local ironic")
		} else {
			By("Install Ironic in the source cluster as deployments")
			installIronicBMO(func() installIronicBMOInput {
				return installIronicBMOInput{
					ManagementCluster:          bootstrapClusterProxy,
					BMOPath:                    e2eConfig.GetVariable(bmoPath),
					deployIronic:               true,
					deployBMO:                  false,
					deployIronicTLSSetup:       getBool(e2eConfig.GetVariable(ironicTLSSetup)),
					deployIronicBasicAuth:      getBool(e2eConfig.GetVariable(ironicBasicAuth)),
					deployIronicKeepalived:     getBool(e2eConfig.GetVariable(ironicKeepalived)),
					deployIronicMariadb:        getBool(e2eConfig.GetVariable(ironicMariadb)),
					NamePrefix:                 e2eConfig.GetVariable(NamePrefix),
					RestartContainerCertUpdate: getBool(e2eConfig.GetVariable(restartContainerCertUpdate)),
				}
			})
		}
	}
	removeIronic(ctx, func() RemoveIronicInput {
		return RemoveIronicInput{
			ManagementCluster: clusterProxy,
			IsDeployment:      true,
			Namespace:         e2eConfig.GetVariable(ironicNamespace),
			NamePrefix:        e2eConfig.GetVariable(NamePrefix),
		}
	})
	reInstallIronic()
}
