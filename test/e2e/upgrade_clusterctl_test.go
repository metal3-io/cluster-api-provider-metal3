package e2e

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	framework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	capiContract  = "v1beta2"
	capm3Contract = "v1beta2"
)

var (
	clusterctlDownloadURL = "https://github.com/kubernetes-sigs/cluster-api/releases/download/v%s/clusterctl-{OS}-{ARCH}"
	providerCAPIPrefix    = "cluster-api:v%s"
	providerKubeadmPrefix = "kubeadm:v%s"
	providerMetal3Prefix  = "metal3:v%s"
	ironicGoproxy         = "https://proxy.golang.org/github.com/metal3-io/ironic-image/@v/list"
	bmoGoproxy            = "https://proxy.golang.org/github.com/metal3-io/baremetal-operator/@v/list"

	k8sVersion                 string
	managementClusterNamespace string
	workloadClusterProxy       framework.ClusterProxy
)

// Ironic 35.0 -> latest image tag.
var _ = Describe("When testing cluster upgrade from releases (v1.13=>current)", Label("clusterctl-upgrade"), func() {
	minorVersion := "1.13"
	bmoFromRelease := "0.13"
	ironicFromRelease := "35.0"
	bmoToRelease := "main"
	ironicToRelease := "main"
	testName := "v1-13-to-current"
	// Use the .99 versions available in the local artifact repository (built from
	// release branch kustomize overlays in e2e_conf.yaml). The old clusterctl binary
	// resolves provider components from the local repo, which only has .99 versions —
	// not the actual released patch versions. Core CAPI providers work because
	// clusterctl has built-in GitHub URLs for them.
	capm3InitVersion := minorVersion + ".99"
	ipamInitVersion := minorVersion + ".99"
	var capiStableRelease string

	BeforeEach(func() {
		k8sVersion = "v1.36.2"
		validateGlobals(specName)
		imageURL, imageChecksum := EnsureImage(k8sVersion)
		os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
		os.Setenv("IMAGE_RAW_URL", imageURL)
		clusterctlLogFolder = filepath.Join(artifactFolder, bootstrapClusterProxy.GetName())

		var err error
		capiStableRelease, err = capi_e2e.GetStableReleaseOfMinor(ctx, minorVersion)
		Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for CAPI minor release : %s", minorVersion)
	})

	AfterEach(func() {
		By("Collecting logs from the bootstrap cluster after clusterctl upgrade test (v1.13=>current)")
		err := FetchClusterLogs(bootstrapClusterProxy, filepath.Join(artifactFolder, testName, "afterEach-logs"))
		if err != nil {
			Logf("Error fetching logs: %v", err)
		}
		err = FetchManifests(bootstrapClusterProxy, filepath.Join(artifactFolder, testName, "afterEach-manifests"))
		if err != nil {
			Logf("Error fetching manifests: %v", err)
		}

		By("Collecting logs from the management cluster")
		if workloadClusterProxy != nil {
			err = FetchClusterLogs(workloadClusterProxy, filepath.Join(artifactFolder, testName, "afterEach-logs"))
			if err != nil {
				Logf("Error fetching management cluster logs: %v", err)
			}
			err = FetchManifests(workloadClusterProxy, filepath.Join(artifactFolder, testName, "afterEach-manifests"))
			if err != nil {
				Logf("Error fetching management cluster manifests: %v", err)
			}
		}
	})

	capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
		return capi_e2e.ClusterctlUpgradeSpecInput{
			E2EConfig:                       e2eConfig,
			ClusterctlConfigPath:            clusterctlConfigPath,
			BootstrapClusterProxy:           bootstrapClusterProxy,
			ArtifactFolder:                  artifactFolder,
			SkipCleanup:                     skipCleanup,
			InitWithCoreProvider:            fmt.Sprintf(providerCAPIPrefix, capiStableRelease),
			InitWithBootstrapProviders:      []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
			InitWithControlPlaneProviders:   []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
			InitWithInfrastructureProviders: []string{fmt.Sprintf(providerMetal3Prefix, capm3InitVersion)},
			InitWithIPAMProviders:           []string{fmt.Sprintf(providerMetal3Prefix, ipamInitVersion)},
			InitWithKubernetesVersion:       k8sVersion,
			WorkloadKubernetesVersion:       k8sVersion,
			InitWithBinary:                  fmt.Sprintf(clusterctlDownloadURL, capiStableRelease),
			PreInit: func(clusterProxy framework.ClusterProxy) {
				preInitFunc(clusterProxy, bmoFromRelease, ironicFromRelease, testName)
				// Override capi/capm3 versions exported in preInit
				os.Setenv("CAPI_VERSION", capiContract)
				os.Setenv("CAPM3_VERSION", capm3Contract)
				os.Setenv("KUBECONFIG_BOOTSTRAP", bootstrapClusterProxy.GetKubeconfigPath())
			},
			PostNamespaceCreated: func(clusterProxy framework.ClusterProxy, clusterNamespace string) {
				postClusterctlUpgradeNamespaceCreated(clusterProxy, clusterNamespace, testName)
			},
			PreUpgrade: func(clusterProxy framework.ClusterProxy) {
				preUpgrade(clusterProxy, bmoToRelease, ironicToRelease, testName)
			},
			PreCleanupManagementCluster: func(clusterProxy framework.ClusterProxy) {
				preCleanupManagementCluster(clusterProxy, testName)
			},
			MgmtFlavor:     osType,
			WorkloadFlavor: osType,
		}
	})
})

// Ironic 33.0 -> latest image tag.
var _ = Describe("When testing cluster upgrade from releases (v1.12=>current)", Label("clusterctl-upgrade"), func() {
	minorVersion := "1.12"
	bmoFromRelease := "0.12"
	ironicFromRelease := "33.0"
	bmoToRelease := "main"
	ironicToRelease := "main"
	testName := "v1-12-to-current"

	// Use the .99 versions available in the local artifact repository (built from
	// release branch kustomize overlays in e2e_conf.yaml). The old clusterctl binary
	// resolves provider components from the local repo, which only has .99 versions —
	// not the actual released patch versions. Core CAPI providers work because
	// clusterctl has built-in GitHub URLs for them.
	capm3InitVersion := minorVersion + ".99"
	ipamInitVersion := minorVersion + ".99"
	var capiStableRelease string

	BeforeEach(func() {
		k8sVersion = "v1.36.2"
		validateGlobals(specName)
		imageURL, imageChecksum := EnsureImage(k8sVersion)
		os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
		os.Setenv("IMAGE_RAW_URL", imageURL)
		clusterctlLogFolder = filepath.Join(artifactFolder, bootstrapClusterProxy.GetName())

		var err error
		capiStableRelease, err = capi_e2e.GetStableReleaseOfMinor(ctx, minorVersion)
		Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for CAPI minor release : %s", minorVersion)
	})

	AfterEach(func() {
		By("Collecting logs from the bootstrap cluster after clusterctl upgrade test (v1.12=>current)")
		err := FetchClusterLogs(bootstrapClusterProxy, filepath.Join(artifactFolder, testName, "afterEach-logs"))
		if err != nil {
			Logf("Error fetching logs: %v", err)
		}
		err = FetchManifests(bootstrapClusterProxy, filepath.Join(artifactFolder, testName, "afterEach-manifests"))
		if err != nil {
			Logf("Error fetching manifests: %v", err)
		}

		By("Collecting logs from the management cluster")
		if workloadClusterProxy != nil {
			err = FetchClusterLogs(workloadClusterProxy, filepath.Join(artifactFolder, testName, "afterEach-logs"))
			if err != nil {
				Logf("Error fetching management cluster logs: %v", err)
			}
			err = FetchManifests(workloadClusterProxy, filepath.Join(artifactFolder, testName, "afterEach-manifests"))
			if err != nil {
				Logf("Error fetching management cluster manifests: %v", err)
			}
		}
	})

	capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
		return capi_e2e.ClusterctlUpgradeSpecInput{
			E2EConfig:                       e2eConfig,
			ClusterctlConfigPath:            clusterctlConfigPath,
			BootstrapClusterProxy:           bootstrapClusterProxy,
			ArtifactFolder:                  artifactFolder,
			SkipCleanup:                     skipCleanup,
			InitWithCoreProvider:            fmt.Sprintf(providerCAPIPrefix, capiStableRelease),
			InitWithBootstrapProviders:      []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
			InitWithControlPlaneProviders:   []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
			InitWithInfrastructureProviders: []string{fmt.Sprintf(providerMetal3Prefix, capm3InitVersion)},
			InitWithIPAMProviders:           []string{fmt.Sprintf(providerMetal3Prefix, ipamInitVersion)},
			InitWithKubernetesVersion:       k8sVersion,
			WorkloadKubernetesVersion:       k8sVersion,
			InitWithBinary:                  fmt.Sprintf(clusterctlDownloadURL, capiStableRelease),
			PreInit: func(clusterProxy framework.ClusterProxy) {
				preInitFunc(clusterProxy, bmoFromRelease, ironicFromRelease, testName)
				// Override capi/capm3 versions exported in preInit
				os.Setenv("CAPI_VERSION", capiContract)
				os.Setenv("CAPM3_VERSION", capm3Contract)
				os.Setenv("KUBECONFIG_BOOTSTRAP", bootstrapClusterProxy.GetKubeconfigPath())
			},
			PostNamespaceCreated: func(clusterProxy framework.ClusterProxy, clusterNamespace string) {
				postClusterctlUpgradeNamespaceCreated(clusterProxy, clusterNamespace, testName)
			},
			PreUpgrade: func(clusterProxy framework.ClusterProxy) {
				preUpgrade(clusterProxy, bmoToRelease, ironicToRelease, testName)
			},
			PreCleanupManagementCluster: func(clusterProxy framework.ClusterProxy) {
				preCleanupManagementCluster(clusterProxy, testName)
			},
			MgmtFlavor:     osType,
			WorkloadFlavor: osType,
		}
	})
})

// postClusterctlUpgradeNamespaceCreated is a hook function that should be called from ClusterctlUpgradeSpec after creating
// the namespace, it creates the needed bmhs in namespace hosting the cluster.
func postClusterctlUpgradeNamespaceCreated(clusterProxy framework.ClusterProxy, clusterNamespace string, testName string) {
	// Check which from which cluster creation this call is coming
	// if isBootstrapProxy==true then this call when creating the management else we are creating the workload.
	isBootstrapProxy := !strings.HasPrefix(clusterProxy.GetName(), "clusterctl-upgrade")
	Expect(len(vmInfos)).To(BeNumerically(">=", 5), "clusterctl-upgrade requires at least 5 VMs in bmcs config (node-0..node-4), got %d", len(vmInfos))

	By("Fetch manifest for cluster")
	err := FetchManifests(clusterProxy, filepath.Join(artifactFolder, testName, "postNamespaceCreated-manifest"))
	if err != nil {
		Logf("Error fetching manifests for cluster : %v", err)
	}

	By("Fetch logs for cluster")
	err = FetchClusterLogs(clusterProxy, filepath.Join(artifactFolder, testName, "postNamespaceCreated-logs"))
	if err != nil {
		Logf("Error fetching logs for cluster: %v", err)
	}

	if isBootstrapProxy {
		managementClusterNamespace = clusterNamespace
		// Power off workload VMs so they don't get accidentally PXE-booted by
		// bootstrap Ironic's dnsmasq while we inspect only node-0 and node-1.
		By("Powering off workload VMs (node-2..4) to prevent accidental PXE boot")
		PowerOffVMs(vmInfos[2:5])

		// Apply BMHs only for [node-0 and node-1] to host the target management cluster.
		ApplyBMHs(ctx, clusterProxy, vmInfos[:2], clusterNamespace)
		WaitForBMHsAvailable(ctx, clusterProxy, clusterNamespace, 2, e2eConfig.GetIntervals(specName, "wait-bmh-available"))
	} else {
		// Apply BMHs for [node-2, node-3 and node-4] in the management cluster to host workload cluster.
		// These VMs were powered off earlier and will be booted fresh by Ironic during inspection.
		ApplyBMHs(ctx, clusterProxy, vmInfos[2:5], clusterNamespace)
		WaitForBMHsAvailable(ctx, clusterProxy, clusterNamespace, 3, e2eConfig.GetIntervals(specName, "wait-bmh-available"))
	}
	ListBareMetalHosts(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(clusterNamespace))
}

// preInitFunc hook function that should be called from ClusterctlUpgradeSpec before init the management cluster
// it installs certManager, BMO and Ironic and overrides the default IPs for the workload cluster.
func preInitFunc(clusterProxy framework.ClusterProxy, bmoRelease string, ironicRelease string, testName string) {
	workloadClusterProxy = clusterProxy
	installCertManager := func(clusterProxy framework.ClusterProxy) {
		certManagerLink := fmt.Sprintf("https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml", e2eConfig.MustGetVariable("CERT_MANAGER_RELEASE"))
		err := DownloadFile("/tmp/certManager.yaml", certManagerLink)
		Expect(err).ToNot(HaveOccurred(), "Unable to download certmanager manifest")
		certManagerYaml, err := os.ReadFile("/tmp/certManager.yaml")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(clusterProxy.CreateOrUpdate(ctx, certManagerYaml)).To(Succeed())

		By("Wait for cert-manager pods to be available")
		deploymentNameList := make([]string, 0, 3)
		deploymentNameList = append(deploymentNameList, "cert-manager", "cert-manager-cainjector", "cert-manager-webhook")
		clientSet := clusterProxy.GetClientSet()
		for _, name := range deploymentNameList {
			deployment, err := clientSet.AppsV1().Deployments("cert-manager").Get(ctx, name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred(), "Unable to get the deployment %s in namespace %s \n error message: %s", name, "cert-manager", err)
			framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
				Getter:     clusterProxy.GetClient(),
				Deployment: deployment,
			}, e2eConfig.GetIntervals(specName, "wait-all-pod-to-be-running-on-target-cluster")...)
		}
		By("Checking that cert-manager is functioning, by creating a self-signed certificate")
		// Create an issuer and certificate to ensure that cert-manager is ready.
		Eventually(func() error {
			return BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
				ClusterProxy:  clusterProxy,
				Kustomization: "data/cert-manager-test",
			})
		}, e2eConfig.GetIntervals(specName, "wait-controllers")...).Should(Succeed())
		// Wait for and check that the certificate becomes ready.
		certKey := client.ObjectKey{
			Name:      "my-selfsigned-cert",
			Namespace: "test",
		}
		testCert := new(unstructured.Unstructured)
		testCert.SetAPIVersion("cert-manager.io/v1")
		testCert.SetKind("Certificate")
		Eventually(func() error {
			if err := clusterProxy.GetClient().Get(ctx, certKey, testCert); err != nil {
				return err
			}
			conditions, found, err := unstructured.NestedSlice(testCert.Object, "status", "conditions")
			if err != nil {
				return err
			}
			if !found {
				return errors.New("certificate doesn't have status.conditions (yet)")
			}
			// There is only one condition (Ready) on certificates.
			condType, ok := conditions[0].(map[string]any)["type"]
			if !ok {
				return errors.New("unexpected condition type")
			}
			condStatus, ok := conditions[0].(map[string]any)["status"]
			if !ok {
				return errors.New("unexpected condition status")
			}
			if condType == "Ready" && condStatus == "True" {
				return nil
			}
			condMessage, ok := conditions[0].(map[string]any)["message"]
			if !ok {
				return errors.New("unexpected condition message")
			}
			return fmt.Errorf("certificate is not ready, type: %s, status: %s, message: %s", condType, condStatus, condMessage)
		}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())
		// Delete test namespace
		Expect(clusterProxy.GetClientSet().CoreV1().Namespaces().Delete(ctx, "test", metav1.DeleteOptions{})).To(Succeed())
	}

	By("Fetch manifest for workload cluster")
	err := FetchManifests(clusterProxy, filepath.Join(artifactFolder, testName, "preInit-manifest"))
	if err != nil {
		Logf("Error fetching manifests for workload cluster: %v", err)
	}

	By("Fetch logs for workload cluster")
	err = FetchClusterLogs(clusterProxy, filepath.Join(artifactFolder, testName, "preInit-logs"))
	if err != nil {
		Logf("Error fetching logs for workload cluster: %v", err)
	}

	By("Fetch manifest for bootstrap cluster")
	err = FetchManifests(bootstrapClusterProxy, filepath.Join(artifactFolder, testName, "preInit-manifest"))
	if err != nil {
		Logf("Error fetching manifests for bootstrap cluster: %v", err)
	}
	By("Fetch logs for bootstrap cluster")
	err = FetchClusterLogs(bootstrapClusterProxy, filepath.Join(artifactFolder, testName, "preInit-logs"))
	if err != nil {
		Logf("Error fetching logs for bootstrap cluster: %v", err)
	}
	// install certmanager
	installCertManager(clusterProxy)
	// Remove ironic
	Byf("Remove Ironic from cluster : %s", bootstrapClusterProxy.GetName())
	removeIronic(ctx, func() RemoveIronicInput {
		return RemoveIronicInput{
			ClusterProxy: bootstrapClusterProxy,
			E2EConfig:    e2eConfig,
		}
	})

	// Remove the provisioning interface from the bootstrap kind node to avoid conflict
	// with the management cluster's keepalived VIP.
	By("Removing provisioning network from bootstrap cluster")
	provisioningInterface := e2eConfig.MustGetVariable("BARE_METAL_PROVISIONER_INTERFACE")
	RemoveProvisioningNetwork(ctx, e2eConfig.ManagementClusterName, provisioningInterface)

	// Power off workload VMs to ensure they boot fresh when Ironic inspects them.
	// Without this, VMs may still be running with a stale IPA from the bootstrap
	// cluster's inspection attempt, causing them to never call back to the new Ironic.
	By("Powering off workload VMs before installing Ironic on management cluster")
	PowerOffVMs(vmInfos[2:5])

	// install ironic
	Byf("Install IRSO with ironic version %s in the target management cluster: %s", ironicRelease, clusterProxy.GetName())
	ironicKustomization := e2eConfig.MustGetVariable("IRSO_IRONIC_" + ironicRelease)
	irsoOperatorVersion := "LATEST"
	if ironicRelease < "32.0" {
		irsoOperatorVersion = "0.8.0"
	}
	irsoKustomizePath := e2eConfig.MustGetVariable("IRSO_OPERATOR_" + irsoOperatorVersion)
	irsoDeployLogFolder := filepath.Join(artifactFolder, testName, clusterProxy.GetName(), "ironic-deploy-logs-preinit")
	err = InstallIRSO(ctx, InstallIRSOInput{
		E2EConfig:             e2eConfig,
		ClusterProxy:          clusterProxy,
		IronicNamespace:       e2eConfig.MustGetVariable(ironicNamespace),
		ClusterName:           clusterProxy.GetName(),
		IrsoOperatorKustomize: irsoKustomizePath,
		IronicKustomize:       ironicKustomization,
		LogPath:               irsoDeployLogFolder,
	})
	Expect(err).NotTo(HaveOccurred())

	// install bmo
	Byf("Install BMO version %s in the target management cluster: %s", bmoRelease, clusterProxy.GetName())
	bmoDeployLogFolder := filepath.Join(artifactFolder, testName, clusterProxy.GetName(), "bmo-deploy-logs")
	bmoKustomizePath := "BMO_RELEASE_" + bmoRelease
	initBMOKustomization := e2eConfig.MustGetVariable(bmoKustomizePath)
	By(fmt.Sprintf("Installing BMO from kustomization %s on the upgrade cluster", initBMOKustomization))
	err = InstallBMO(ctx, InstallBMOInput{
		E2EConfig:        e2eConfig,
		ClusterProxy:     clusterProxy,
		Namespace:        e2eConfig.MustGetVariable(ironicNamespace),
		BmoKustomization: initBMOKustomization,
		LogFolder:        bmoDeployLogFolder,
		WatchLogs:        true,
	})
	Expect(err).NotTo(HaveOccurred())

	By("Fetch manifest for workload cluster")
	err = FetchManifests(clusterProxy, filepath.Join(artifactFolder, testName, "postInit-manifest"))
	if err != nil {
		Logf("Error fetching manifests for workload cluster: %v", err)
	}

	By("Fetch logs for workload cluster")
	err = FetchClusterLogs(clusterProxy, filepath.Join(artifactFolder, testName, "postInit-logs"))
	if err != nil {
		Logf("Error fetching logs for workload cluster: %v", err)
	}
	// Export capi/capm3 versions
	os.Setenv("CAPI_VERSION", capiContract)
	os.Setenv("CAPM3_VERSION", capm3Contract)

	// These exports bellow we need them after applying the management cluster template and before
	// applying the workload. if exported before it will break creating the management because it uses v1beta1 templates and default IPs.
	// override default IPs for the workload cluster
	os.Setenv("CLUSTER_APIENDPOINT_HOST", "192.168.111.250")
	os.Setenv("IPAM_EXTERNALV4_POOL_RANGE_START", "192.168.111.201")
	os.Setenv("IPAM_EXTERNALV4_POOL_RANGE_END", "192.168.111.240")
	os.Setenv("IPAM_PROVISIONING_POOL_RANGE_START", "172.22.0.201")
	os.Setenv("IPAM_PROVISIONING_POOL_RANGE_END", "172.22.0.240")
}

// preUpgrade hook should be called from ClusterctlUpgradeSpec before upgrading the management cluster
// it upgrades Ironic and BMO before upgrading the providers.
func preUpgrade(clusterProxy framework.ClusterProxy, bmoUpgradeToRelease string, ironicUpgradeToRelease string, testName string) {
	err := FetchManifests(clusterProxy, filepath.Join(artifactFolder, testName, "preUpgrade-manifest"))
	if err != nil {
		Logf("Error fetching manifests for bootstrap cluster: %v", err)
	}

	err = FetchClusterLogs(clusterProxy, filepath.Join(artifactFolder, testName, "preUpgrade-logs"))
	if err != nil {
		Logf("Error fetching logs for workload cluster: %v", err)
	}

	ironicTag, err := GetLatestPatchRelease(ironicGoproxy, ironicUpgradeToRelease)
	Expect(err).ToNot(HaveOccurred(), "Failed to fetch ironic version for release %s", ironicUpgradeToRelease)
	Logf("Ironic Tag %s\n", ironicTag)

	bmoTag, err := GetLatestPatchRelease(bmoGoproxy, bmoUpgradeToRelease)
	Expect(err).ToNot(HaveOccurred(), "Failed to fetch bmo version for release %s", bmoUpgradeToRelease)
	Logf("Bmo Tag %s\n", bmoTag)

	Byf("Upgrade IRSO with ironic version %s in the target management cluster: %s", ironicTag, clusterProxy.GetName())
	ironicKustomization := e2eConfig.MustGetVariable("IRSO_IRONIC_" + ironicTag)
	irsoOperatorVersion := "LATEST"
	if ironicTag < "32.0" {
		irsoOperatorVersion = "0.8.0"
	}
	irsoKustomizePath := e2eConfig.MustGetVariable("IRSO_OPERATOR_" + irsoOperatorVersion)
	irsoDeployLogFolder := filepath.Join(artifactFolder, testName, clusterProxy.GetName(), "ironic-deploy-logs-preupgrade")
	err = InstallIRSO(ctx, InstallIRSOInput{
		E2EConfig:             e2eConfig,
		ClusterProxy:          clusterProxy,
		IronicNamespace:       e2eConfig.MustGetVariable(ironicNamespace),
		ClusterName:           clusterProxy.GetName(),
		IrsoOperatorKustomize: irsoKustomizePath,
		IronicKustomize:       ironicKustomization,
		LogPath:               irsoDeployLogFolder,
	})
	Expect(err).NotTo(HaveOccurred())

	// install bmo
	Byf("Upgrade BMO with version %s in the target management cluster: %s", bmoTag, clusterProxy.GetName())
	bmoDeployLogFolder := filepath.Join(artifactFolder, testName, clusterProxy.GetName(), "bmo-deploy-logs")
	bmoKustomizePath := "BMO_RELEASE_" + bmoTag
	initBMOKustomization := e2eConfig.MustGetVariable(bmoKustomizePath)
	By(fmt.Sprintf("Upgrading BMO from kustomization %s on the upgrade cluster", initBMOKustomization))
	err = InstallBMO(ctx, InstallBMOInput{
		E2EConfig:        e2eConfig,
		ClusterProxy:     clusterProxy,
		Namespace:        e2eConfig.MustGetVariable(ironicNamespace),
		BmoKustomization: initBMOKustomization,
		LogFolder:        bmoDeployLogFolder,
		WatchLogs:        true,
	})
	Expect(err).NotTo(HaveOccurred())

	err = FetchManifests(clusterProxy, filepath.Join(artifactFolder, testName, "postUpgrade-manifest"))
	if err != nil {
		Logf("Error fetching manifests for bootstrap cluster: %v", err)
	}

	err = FetchClusterLogs(clusterProxy, filepath.Join(artifactFolder, testName, "postUpgrade-logs"))
	if err != nil {
		Logf("Error fetching logs for workload cluster: %v", err)
	}
}

// preCleanupManagementCluster hook should be called from ClusterctlUpgradeSpec before cleaning the target management cluster
// it moves back Ironic to the bootstrap cluster.
func preCleanupManagementCluster(clusterProxy framework.ClusterProxy, testName string) {
	By("Fetch logs from target cluster")
	err := FetchClusterLogs(clusterProxy, filepath.Join(artifactFolder, testName, "preCleanup-logs"))
	if err != nil {
		Logf("Error: %v", err)
	}

	err = FetchManifests(clusterProxy, filepath.Join(artifactFolder, testName, "preCleanup-manifest"))
	if err != nil {
		Logf("Error fetching manifests for bootstrap cluster: %v", err)
	}
	os.Unsetenv("KUBECONFIG_WORKLOAD")
	os.Unsetenv("KUBECONFIG_BOOTSTRAP")
	bmoIronicNamespace := e2eConfig.MustGetVariable(ironicNamespace)
	// Reinstall ironic
	reInstallIronic := func() {
		By("ReInstall IRSO in the bootstrap cluster")
		irsoDeployLogFolder := filepath.Join(artifactFolder, testName, bootstrapClusterProxy.GetName(), "ironic-deploy-logs-reinstall")
		err := InstallIRSO(ctx, InstallIRSOInput{
			E2EConfig:             e2eConfig,
			ClusterProxy:          bootstrapClusterProxy,
			IronicNamespace:       bmoIronicNamespace,
			ClusterName:           bootstrapClusterProxy.GetName(),
			IrsoOperatorKustomize: e2eConfig.MustGetVariable("IRSO_OPERATOR_LATEST"),
			IronicKustomize:       e2eConfig.MustGetVariable("IRSO_IRONIC_PR_TEST"),
			LogPath:               irsoDeployLogFolder,
		})
		Expect(err).NotTo(HaveOccurred())
	}

	removeIronic(ctx, func() RemoveIronicInput {
		return RemoveIronicInput{
			ClusterProxy: clusterProxy,
			E2EConfig:    e2eConfig,
		}
	})

	// Re-add the provisioning IP to the bootstrap kind node before reinstalling Ironic.
	By("Reconfiguring provisioning network on the bootstrap cluster")
	provisioningIP := os.Getenv("CLUSTER_PROVISIONING_IP")
	if provisioningIP == "" {
		provisioningIP = "172.22.0.2"
	}
	provisioningInterface := e2eConfig.MustGetVariable("BARE_METAL_PROVISIONER_INTERFACE")
	ConfigureProvisioningNetwork(ctx, e2eConfig.ManagementClusterName, provisioningIP, provisioningInterface)

	reInstallIronic()

	// Restart BMO so it picks up the new Ironic TLS certificates. BMO has been running
	// since initial setup and its cached TLS connection is stale after Ironic redeployment.
	By("Restarting BMO on the bootstrap cluster to refresh Ironic connection")
	bmoPods := &corev1.PodList{}
	Expect(bootstrapClusterProxy.GetClient().List(ctx, bmoPods,
		client.InNamespace(bmoIronicNamespace),
		client.MatchingLabels{"control-plane": "controller-manager"},
	)).To(Succeed())
	for i := range bmoPods.Items {
		Expect(bootstrapClusterProxy.GetClient().Delete(ctx, &bmoPods.Items[i])).To(Succeed())
		Logf("Deleted BMO pod %s", bmoPods.Items[i].Name)
	}
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: bootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "baremetal-operator-controller-manager",
				Namespace: bmoIronicNamespace,
			},
		},
	}, e2eConfig.GetIntervals("default", "wait-deployment")...)

	// Clean env variables set for management upgrade, defaults are set in e2e config file
	// Capi/capm3 versions
	os.Unsetenv("CAPI_VERSION")
	os.Unsetenv("CAPM3_VERSION")
	// The provider id format
	os.Unsetenv("PROVIDER_ID_FORMAT")
	// IPs
	os.Unsetenv("CLUSTER_APIENDPOINT_HOST")
	os.Unsetenv("IPAM_EXTERNALV4_POOL_RANGE_START")
	os.Unsetenv("IPAM_EXTERNALV4_POOL_RANGE_END")
	os.Unsetenv("IPAM_PROVISIONING_POOL_RANGE_START")
	os.Unsetenv("IPAM_PROVISIONING_POOL_RANGE_END")
}
