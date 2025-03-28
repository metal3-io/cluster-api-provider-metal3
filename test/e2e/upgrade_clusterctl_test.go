package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	framework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	workDir  = "/opt/metal3-dev-env/"
	contract = "v1beta1"
)

var (
	clusterctlDownloadURL = "https://github.com/kubernetes-sigs/cluster-api/releases/download/v%s/clusterctl-{OS}-{ARCH}"
	providerCAPIPrefix    = "cluster-api:v%s"
	providerKubeadmPrefix = "kubeadm:v%s"
	providerMetal3Prefix  = "metal3:v%s"
	ironicGoproxy         = "https://proxy.golang.org/github.com/metal3-io/ironic-image/@v/list"
	bmoGoproxy            = "https://proxy.golang.org/github.com/metal3-io/baremetal-operator/@v/list"

	k8sVersion string
)

var _ = Describe("When testing cluster upgrade from releases (v1.9=>current) [clusterctl-upgrade]", func() {
	BeforeEach(func() {
		k8sVersion = "v1.32.1"
		validateGlobals(specName)
		imageURL, imageChecksum := EnsureImage(k8sVersion)
		os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
		os.Setenv("IMAGE_RAW_URL", imageURL)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "target_cluster_logs", bootstrapClusterProxy.GetName())
	})

	minorVersion := "1.9"
	bmoFromRelease := "0.9"
	ironicFromRelease := "27.0"
	bmoToRelease := "latest"
	ironicToRelease := "latest"
	capiStableRelease, err := capi_e2e.GetStableReleaseOfMinor(ctx, minorVersion)
	Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for CAPI minor release : %s", minorVersion)
	capm3StableRelease, err := GetCAPM3StableReleaseOfMinor(ctx, minorVersion)
	Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for CAPM3 minor release : %s", minorVersion)

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
			InitWithInfrastructureProviders: []string{fmt.Sprintf(providerMetal3Prefix, capm3StableRelease)},
			InitWithIPAMProviders:           []string{""}, // Explicitly set to empty since we use the IPAM bundled with CAPM3.
			InitWithKubernetesVersion:       k8sVersion,
			WorkloadKubernetesVersion:       k8sVersion,
			InitWithBinary:                  fmt.Sprintf(clusterctlDownloadURL, capiStableRelease),
			PreInit: func(clusterProxy framework.ClusterProxy) {
				preInitFunc(clusterProxy, bmoFromRelease, ironicFromRelease)
				// Override capi/capm3 versions exported in preInit
				os.Setenv("CAPI_VERSION", contract)
				os.Setenv("CAPM3_VERSION", contract)
				os.Setenv("KUBECONFIG_BOOTSTRAP", bootstrapClusterProxy.GetKubeconfigPath())
			},
			PostNamespaceCreated: postNamespaceCreated,
			PreUpgrade: func(clusterProxy framework.ClusterProxy) {
				preUpgrade(clusterProxy, bmoToRelease, ironicToRelease)
			},
			PostUpgrade: func(clusterProxy framework.ClusterProxy, _ string, _ string) {
				postUpgrade(ctx, clusterProxy)
			},
			PreCleanupManagementCluster: func(clusterProxy framework.ClusterProxy) {
				preCleanupManagementCluster(clusterProxy, ironicToRelease)
			},
			MgmtFlavor:     osType,
			WorkloadFlavor: osType,
		}
	})
})

var _ = Describe("When testing cluster upgrade from releases (v1.8=>current) [clusterctl-upgrade]", func() {
	BeforeEach(func() {
		k8sVersion = "v1.31.2"
		validateGlobals(specName)
		imageURL, imageChecksum := EnsureImage(k8sVersion)
		os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
		os.Setenv("IMAGE_RAW_URL", imageURL)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "target_cluster_logs", bootstrapClusterProxy.GetName())
	})

	minorVersion := "1.8"
	bmoFromRelease := "0.8"
	ironicFromRelease := "26.0"
	bmoToRelease := "latest"
	ironicToRelease := "latest"
	capiStableRelease, err := capi_e2e.GetStableReleaseOfMinor(ctx, minorVersion)
	Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for CAPI minor release : %s", minorVersion)
	capm3StableRelease, err := GetCAPM3StableReleaseOfMinor(ctx, minorVersion)
	Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for CAPM3 minor release : %s", minorVersion)

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
			InitWithInfrastructureProviders: []string{fmt.Sprintf(providerMetal3Prefix, capm3StableRelease)},
			InitWithIPAMProviders:           []string{""}, // Explicitly set to empty since we use the IPAM bundled with CAPM3.
			InitWithKubernetesVersion:       k8sVersion,
			WorkloadKubernetesVersion:       k8sVersion,
			InitWithBinary:                  fmt.Sprintf(clusterctlDownloadURL, capiStableRelease),
			PreInit: func(clusterProxy framework.ClusterProxy) {
				preInitFunc(clusterProxy, bmoFromRelease, ironicFromRelease)
				// Override capi/capm3 versions exported in preInit
				os.Setenv("CAPI_VERSION", contract)
				os.Setenv("CAPM3_VERSION", contract)
				os.Setenv("KUBECONFIG_BOOTSTRAP", bootstrapClusterProxy.GetKubeconfigPath())
			},
			PostNamespaceCreated: postNamespaceCreated,
			PreUpgrade: func(clusterProxy framework.ClusterProxy) {
				preUpgrade(clusterProxy, bmoToRelease, ironicToRelease)
			},
			PostUpgrade: func(clusterProxy framework.ClusterProxy, _ string, _ string) {
				postUpgrade(ctx, clusterProxy)
			},
			PreCleanupManagementCluster: func(clusterProxy framework.ClusterProxy) {
				preCleanupManagementCluster(clusterProxy, ironicToRelease)
			},
			MgmtFlavor:     osType,
			WorkloadFlavor: osType,
		}
	})
})

// postNamespaceCreated is a hook function that should be called from ClusterctlUpgradeSpec after creating
// the namespace, it creates the needed bmhs in namespace hosting the cluster.
func postNamespaceCreated(clusterProxy framework.ClusterProxy, clusterNamespace string) {
	// Check which from which cluster creation this call is coming
	// if isBootstrapProxy==true then this call when creating the management else we are creating the workload.
	isBootstrapProxy := !strings.HasPrefix(clusterProxy.GetName(), "clusterctl-upgrade")
	if isBootstrapProxy {
		// Apply secrets and bmhs for [node_0 and node_1] in the management cluster to host the target management cluster
		for i := 0; i < 2; i++ {
			resource, err := os.ReadFile(filepath.Join(workDir, fmt.Sprintf("bmhs/node_%d.yaml", i)))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(CreateOrUpdateWithNamespace(ctx, clusterProxy, resource, clusterNamespace)).ShouldNot(HaveOccurred())
		}
		clusterClient := clusterProxy.GetClient()
		ListBareMetalHosts(ctx, clusterClient, client.InNamespace(clusterNamespace))
		WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
			Client:    clusterClient,
			Options:   []client.ListOption{client.InNamespace(clusterNamespace)},
			Replicas:  2,
			Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
		})
		ListBareMetalHosts(ctx, clusterClient, client.InNamespace(clusterNamespace))
	} else {
		// Apply secrets and bmhs for [node_2, node_3 and node_4] in the management cluster to host workload cluster
		for i := 2; i < 5; i++ {
			resource, err := os.ReadFile(filepath.Join(workDir, fmt.Sprintf("bmhs/node_%d.yaml", i)))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(CreateOrUpdateWithNamespace(ctx, clusterProxy, resource, clusterNamespace)).ShouldNot(HaveOccurred())
		}
		clusterClient := clusterProxy.GetClient()
		ListBareMetalHosts(ctx, clusterClient, client.InNamespace(clusterNamespace))
		WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
			Client:    clusterClient,
			Options:   []client.ListOption{client.InNamespace(clusterNamespace)},
			Replicas:  3,
			Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
		})
		ListBareMetalHosts(ctx, clusterClient, client.InNamespace(clusterNamespace))
	}
	ListBareMetalHosts(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(clusterNamespace))
}

// preInitFunc hook function that should be called from ClusterctlUpgradeSpec before init the management cluster
// it installs certManager, BMO and Ironic and overrides the default IPs for the workload cluster.
func preInitFunc(clusterProxy framework.ClusterProxy, bmoRelease string, ironicRelease string) {
	installCertManager := func(clusterProxy framework.ClusterProxy) {
		certManagerLink := fmt.Sprintf("https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml", config.CertManagerDefaultVersion)
		err := DownloadFile("/tmp/certManager.yaml", certManagerLink)
		Expect(err).ToNot(HaveOccurred(), "Unable to download certmanager manifest")
		certManagerYaml, err := os.ReadFile("/tmp/certManager.yaml")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(clusterProxy.CreateOrUpdate(ctx, certManagerYaml)).To(Succeed())

		By("Wait for cert-manager pods to be available")
		deploymentNameList := []string{}
		deploymentNameList = append(deploymentNameList, "cert-manager", "cert-manager-cainjector", "cert-manager-webhook")
		clientSet := clusterProxy.GetClientSet()
		for _, name := range deploymentNameList {
			deployment, err := clientSet.AppsV1().Deployments("cert-manager").Get(ctx, name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred(), "Unable to get the deployment %s in namespace %s \n error message: %s", name, "cert-manager", err)
			framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
				Getter:     clusterProxy.GetClient(),
				Deployment: deployment,
			}, e2eConfig.GetIntervals(specName, "wait-controllers")...)
		}
		By("Checking that cert-manager is functioning, by creating a self-signed certificate")
		// Create an issuer and certificate to ensure that cert-manager is ready.
		Eventually(func() error {
			// TODO(lentzi90): Bug in cert-manager. The webhook can be stuck refusing connections
			// and still appear Running. We need to restart the pod to get it working.
			// This is a workaround until the bug is fixed.
			err := clusterProxy.GetClientSet().CoreV1().Pods("cert-manager").DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: "app.kubernetes.io/name=webhook",
			})
			Expect(err).NotTo(HaveOccurred(), "Unable to delete webhook pod")
			// Get the cert-manager-webhook deployment and wait for it to be available again.
			deployment, err := clientSet.AppsV1().Deployments("cert-manager").Get(ctx, "cert-manager-webhook", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred(), "Unable to get the cert-manager-webhook deployment\nerror message: %s", err)
			framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
				Getter:     clusterProxy.GetClient(),
				Deployment: deployment,
			}, e2eConfig.GetIntervals(specName, "wait-controllers")...)

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
				return fmt.Errorf("certificate doesn't have status.conditions (yet)")
			}
			// There is only one condition (Ready) on certificates.
			condType := conditions[0].(map[string]any)["type"]
			condStatus := conditions[0].(map[string]any)["status"]
			if condType == "Ready" && condStatus == "True" {
				return nil
			}
			return fmt.Errorf("certificate is not ready, type: %s, status: %s, message: %s", condType, condStatus, conditions[0].(map[string]any)["message"])
		}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())
		// Delete test namespace
		Expect(clusterProxy.GetClientSet().CoreV1().Namespaces().Delete(ctx, "test", metav1.DeleteOptions{})).To(Succeed())
	}

	By("Fetch manifest for bootstrap cluster")
	err := FetchManifests(clusterProxy, "/tmp/manifests/")
	if err != nil {
		Logf("Error fetching manifests for bootstrap cluster: %v", err)
	}

	By("Fetch target cluster kubeconfig for target cluster log collection")
	kconfigPathWorkload := clusterProxy.GetKubeconfigPath()
	os.Setenv("KUBECONFIG_WORKLOAD", kconfigPathWorkload)
	Logf("Save kubeconfig in temp folder for project-infra target log collection")
	kubeconfigPathTemp := "/tmp/kubeconfig-test1.yaml"
	cmd := exec.Command("cp", kconfigPathWorkload, kubeconfigPathTemp) // #nosec G204:gosec
	stdoutStderr, er := cmd.CombinedOutput()
	Expect(er).ToNot(HaveOccurred(), "Cannot fetch target cluster kubeconfig: %s", stdoutStderr)
	// install certmanager
	installCertManager(clusterProxy)
	// Remove ironic
	By("Remove Ironic containers from the source cluster")
	ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
	ironicDeploymentType := IronicDeploymentTypeBMO
	if ephemeralCluster == Kind {
		ironicDeploymentType = IronicDeploymentTypeLocal
	} else if GetBoolVariable(e2eConfig, "USE_IRSO") {
		ironicDeploymentType = IronicDeploymentTypeIrSO
	}
	removeIronic(ctx, func() RemoveIronicInput {
		return RemoveIronicInput{
			ManagementCluster: bootstrapClusterProxy,
			DeploymentType:    ironicDeploymentType,
			Namespace:         e2eConfig.MustGetVariable(ironicNamespace),
			NamePrefix:        e2eConfig.MustGetVariable(NamePrefix),
		}
	})
	bmoIronicNamespace := e2eConfig.MustGetVariable(ironicNamespace)
	// install ironic
	By("Install Ironic in the target cluster")
	ironicDeployLogFolder := filepath.Join(os.TempDir(), "target_cluster_logs", "ironic-deploy-logs", clusterProxy.GetName())
	ironicKustomizePath := fmt.Sprintf("IRONIC_RELEASE_%s", ironicRelease)
	initIronicKustomization := e2eConfig.MustGetVariable(ironicKustomizePath)
	By(fmt.Sprintf("Installing Ironic from kustomization %s on the upgrade cluster", initIronicKustomization))
	err = BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       initIronicKustomization,
		ClusterProxy:        clusterProxy,
		WaitForDeployment:   true,
		WatchDeploymentLogs: true,
		LogPath:             ironicDeployLogFolder,
		DeploymentName:      "baremetal-operator-ironic",
		DeploymentNamespace: bmoIronicNamespace,
		WaitIntervals:       e2eConfig.GetIntervals("default", "wait-deployment"),
	})
	Expect(err).NotTo(HaveOccurred(), "Failed to install Ironic on target cluster %v", err)

	// install bmo
	By("Install BMO in the target cluster")
	bmoDeployLogFolder := filepath.Join(os.TempDir(), "target_cluster_logs", "bmo-deploy-logs", clusterProxy.GetName())
	bmoKustomizePath := fmt.Sprintf("BMO_RELEASE_%s", bmoRelease)
	initBMOKustomization := e2eConfig.MustGetVariable(bmoKustomizePath)
	By(fmt.Sprintf("Installing BMO from kustomization %s on the upgrade cluster", initBMOKustomization))
	err = BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       initBMOKustomization,
		ClusterProxy:        clusterProxy,
		WaitForDeployment:   true,
		WatchDeploymentLogs: true,
		LogPath:             bmoDeployLogFolder,
		DeploymentName:      "baremetal-operator-controller-manager",
		DeploymentNamespace: bmoIronicNamespace,
		WaitIntervals:       e2eConfig.GetIntervals("default", "wait-deployment"),
	})
	Expect(err).NotTo(HaveOccurred(), "Failed to install BMO on target cluster %v", err)

	// Export capi/capm3 versions
	os.Setenv("CAPI_VERSION", contract)
	os.Setenv("CAPM3_VERSION", contract)

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
func preUpgrade(clusterProxy framework.ClusterProxy, bmoUpgradeToRelease string, ironicUpgradeToRelease string) {
	ironicTag, err := GetLatestPatchRelease(ironicGoproxy, ironicUpgradeToRelease)
	Expect(err).ToNot(HaveOccurred(), "Failed to fetch ironic version for release %s", ironicUpgradeToRelease)
	Logf("Ironic Tag %s\n", ironicTag)

	bmoTag, err := GetLatestPatchRelease(bmoGoproxy, bmoUpgradeToRelease)
	Expect(err).ToNot(HaveOccurred(), "Failed to fetch bmo version for release %s", bmoUpgradeToRelease)
	Logf("Bmo Tag %s\n", bmoTag)

	bmoIronicNamespace := e2eConfig.MustGetVariable(ironicNamespace)
	By("Upgrade Ironic in the target cluster")
	ironicDeployLogFolder := filepath.Join(os.TempDir(), "target_cluster_logs", "ironic-deploy-logs", clusterProxy.GetName())
	ironicKustomizePath := fmt.Sprintf("IRONIC_RELEASE_%s", ironicTag)
	initIronicKustomization := e2eConfig.MustGetVariable(ironicKustomizePath)
	By(fmt.Sprintf("Upgrading Ironic from kustomization %s on the upgrade cluster", initIronicKustomization))
	err = BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       initIronicKustomization,
		ClusterProxy:        clusterProxy,
		WaitForDeployment:   true,
		WatchDeploymentLogs: true,
		LogPath:             ironicDeployLogFolder,
		DeploymentName:      "baremetal-operator-ironic",
		DeploymentNamespace: bmoIronicNamespace,
		WaitIntervals:       e2eConfig.GetIntervals("default", "wait-deployment"),
	})
	Expect(err).NotTo(HaveOccurred())

	// install bmo
	By("Upgrade BMO in the target cluster")
	bmoDeployLogFolder := filepath.Join(os.TempDir(), "target_cluster_logs", "bmo-deploy-logs", clusterProxy.GetName())
	bmoKustomizePath := fmt.Sprintf("BMO_RELEASE_%s", bmoTag)
	initBMOKustomization := e2eConfig.MustGetVariable(bmoKustomizePath)
	By(fmt.Sprintf("Upgrading BMO from kustomization %s on the upgrade cluster", initBMOKustomization))
	err = BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       initBMOKustomization,
		ClusterProxy:        clusterProxy,
		WaitForDeployment:   true,
		WatchDeploymentLogs: true,
		LogPath:             bmoDeployLogFolder,
		DeploymentName:      "baremetal-operator-controller-manager",
		DeploymentNamespace: bmoIronicNamespace,
		WaitIntervals:       e2eConfig.GetIntervals("default", "wait-deployment"),
	})
	Expect(err).NotTo(HaveOccurred())
}

// postUpgrade hook is for installing the new Metal3 IPAM provider
// when upgrading from CAPM3 bundled IPAM.
func postUpgrade(ctx context.Context, clusterProxy framework.ClusterProxy) {
	By("Installing Metal3 IPAM provider")
	ipamDeployLogFolder := filepath.Join(os.TempDir(), "target_cluster_logs", "ipam-deploy-logs", clusterProxy.GetName())
	ipamVersions := e2eConfig.GetProviderLatestVersionsByContract(contract, e2eConfig.IPAMProviders()...)
	Expect(ipamVersions).To(HaveLen(1), "Failed to get the latest version for the IPAM provider")
	input := clusterctl.InitInput{
		ClusterctlConfigPath: clusterctlConfigPath,
		KubeconfigPath:       clusterProxy.GetKubeconfigPath(),
		LogFolder:            ipamDeployLogFolder,
		IPAMProviders:        []string{ipamVersions[0]},
	}
	clusterctl.Init(ctx, input)
}

// preCleanupManagementCluster hook should be called from ClusterctlUpgradeSpec before cleaning the target management cluster
// it moves back Ironic to the bootstrap cluster.
func preCleanupManagementCluster(clusterProxy framework.ClusterProxy, ironicRelease string) {
	By("Fetch logs from target cluster")
	err := FetchClusterLogs(clusterProxy, clusterLogCollectionBasePath)
	if err != nil {
		Logf("Error: %v", err)
	}
	os.Unsetenv("KUBECONFIG_WORKLOAD")
	os.Unsetenv("KUBECONFIG_BOOTSTRAP")
	bmoIronicNamespace := e2eConfig.MustGetVariable(ironicNamespace)
	// Remove BMO to stop log watchers. They will otherwise spam errors as the cluster is deleted.
	RemoveDeployment(ctx, func() RemoveDeploymentInput {
		return RemoveDeploymentInput{
			ManagementCluster: clusterProxy,
			Namespace:         bmoIronicNamespace,
			Name:              e2eConfig.MustGetVariable(NamePrefix) + "-controller-manager",
		}
	})
	// Reinstall ironic
	reInstallIronic := func() {
		By("Reinstate Ironic containers and BMH")
		ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
		if ephemeralCluster == Kind {
			By("Install Ironic in the source cluster as containers")
			bmoPath := e2eConfig.MustGetVariable("BMOPATH")
			ironicCommand := bmoPath + "/tools/run_local_ironic.sh"
			//#nosec G204 -- We take the BMOPATH from a variable.
			cmd := exec.Command("sh", "-c", "export CONTAINER_RUNTIME=docker; "+ironicCommand)
			stdoutStderr, err := cmd.CombinedOutput()
			Logf("Output: %s", stdoutStderr)
			Expect(err).ToNot(HaveOccurred(), "Cannot run local ironic")
		} else {
			By("Install Ironic in the source cluster as deployments")
			ironicDeployLogFolder := filepath.Join(os.TempDir(), "target_cluster_logs", "ironic-deploy-logs", bootstrapClusterProxy.GetName())
			ironicKustomizePath := fmt.Sprintf("IRONIC_RELEASE_%s", ironicRelease)
			initIronicKustomization := e2eConfig.MustGetVariable(ironicKustomizePath)
			namePrefix := e2eConfig.MustGetVariable("NAMEPREFIX")
			ironicDeployName := namePrefix + ironicSuffix
			By(fmt.Sprintf("Installing Ironic from kustomization %s on the upgrade cluster", initIronicKustomization))
			err := BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
				Kustomization:       initIronicKustomization,
				ClusterProxy:        bootstrapClusterProxy,
				WaitForDeployment:   true,
				WatchDeploymentLogs: true,
				LogPath:             ironicDeployLogFolder,
				DeploymentName:      ironicDeployName,
				DeploymentNamespace: bmoIronicNamespace,
				WaitIntervals:       e2eConfig.GetIntervals("default", "wait-deployment"),
			})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	ironicDeploymentType := IronicDeploymentTypeBMO
	// TODO(dtantsur): support USE_IRSO in the target cluster
	removeIronic(ctx, func() RemoveIronicInput {
		return RemoveIronicInput{
			ManagementCluster: clusterProxy,
			DeploymentType:    ironicDeploymentType,
			Namespace:         e2eConfig.MustGetVariable(ironicNamespace),
			NamePrefix:        e2eConfig.MustGetVariable(NamePrefix),
		}
	})
	reInstallIronic()

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
