package e2e

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	bmo_e2e "github.com/metal3-io/baremetal-operator/test/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	framework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const workDir = "/opt/metal3-dev-env/"

var (
	clusterctlDownloadURL = "https://github.com/kubernetes-sigs/cluster-api/releases/download/v%s/clusterctl-{OS}-{ARCH}"
	providerCAPIPrefix    = "cluster-api:v%s"
	providerKubeadmPrefix = "kubeadm:v%s"
	providerMetal3Prefix  = "metal3:v%s"
)

var _ = Describe("When testing cluster upgrade from releases (v1.7=>current) [clusterctl-upgrade]", func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
		imageURL, imageChecksum := EnsureImage("v1.30.0")
		os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
		os.Setenv("IMAGE_RAW_URL", imageURL)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	minorVersion := "1.7"
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
			InitWithKubernetesVersion:       "v1.30.0",
			WorkloadKubernetesVersion:       "v1.30.0",
			InitWithBinary:                  fmt.Sprintf(clusterctlDownloadURL, capiStableRelease),
			PreInit: func(clusterProxy framework.ClusterProxy) {
				preInitFunc(clusterProxy)
				// Override capi/capm3 versions exported in preInit
				os.Setenv("CAPI_VERSION", "v1beta1")
				os.Setenv("CAPM3_VERSION", "v1beta1")
				os.Setenv("KUBECONFIG_BOOTSTRAP", bootstrapClusterProxy.GetKubeconfigPath())
			},
			PostNamespaceCreated:        postNamespaceCreated,
			PreUpgrade:                  preUpgrade,
			PreCleanupManagementCluster: preCleanupManagementCluster,
			MgmtFlavor:                  osType,
			WorkloadFlavor:              osType,
		}
	})
	AfterEach(func() {
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
			Replicas:  5,
			Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
		})
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))
	})
})

var _ = Describe("When testing cluster upgrade from releases (v1.6=>current) [clusterctl-upgrade]", func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
		imageURL, imageChecksum := EnsureImage("v1.29.0")
		os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
		os.Setenv("IMAGE_RAW_URL", imageURL)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	minorVersion := "1.6"
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
			InitWithKubernetesVersion:       "v1.29.0",
			WorkloadKubernetesVersion:       "v1.29.0",
			InitWithBinary:                  fmt.Sprintf(clusterctlDownloadURL, capiStableRelease),
			PreInit: func(clusterProxy framework.ClusterProxy) {
				preInitFunc(clusterProxy)
				// Override capi/capm3 versions exported in preInit
				os.Setenv("CAPI_VERSION", "v1beta1")
				os.Setenv("CAPM3_VERSION", "v1beta1")
				os.Setenv("KUBECONFIG_BOOTSTRAP", bootstrapClusterProxy.GetKubeconfigPath())
			},
			PostNamespaceCreated:        postNamespaceCreated,
			PreUpgrade:                  preUpgrade,
			PreCleanupManagementCluster: preCleanupManagementCluster,
			MgmtFlavor:                  osType,
			WorkloadFlavor:              osType,
		}
	})
	AfterEach(func() {
		// Recreate bmh that was used in capi namespace in metal3
		//#nosec G204 -- We need to pass in the file name here.
		cmd := exec.Command("bash", "-c", "kubectl apply -f bmhosts_crs.yaml  -n metal3")
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		bmo_e2e.Logf("Applying bmh to metal3 namespace : \n %v", string(output))
		Expect(err).ToNot(HaveOccurred())
		// wait for all bmh to become available
		bootstrapClient := bootstrapClusterProxy.GetClient()
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))
		WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
			Client:    bootstrapClient,
			Options:   []client.ListOption{client.InNamespace(namespace)},
			Replicas:  5,
			Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
		})
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))
	})
})

var _ = Describe("When testing cluster upgrade from releases (v1.5=>current) [clusterctl-upgrade]", func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
		imageURL, imageChecksum := EnsureImage("v1.28.1")
		os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
		os.Setenv("IMAGE_RAW_URL", imageURL)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	minorVersion := "1.5"
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
			InitWithKubernetesVersion:       "v1.28.1",
			WorkloadKubernetesVersion:       "v1.28.1",
			InitWithBinary:                  fmt.Sprintf(clusterctlDownloadURL, capiStableRelease),
			PreInit: func(clusterProxy framework.ClusterProxy) {
				preInitFunc(clusterProxy)
				// Override capi/capm3 versions exported in preInit
				os.Setenv("CAPI_VERSION", "v1beta1")
				os.Setenv("CAPM3_VERSION", "v1beta1")
				os.Setenv("KUBECONFIG_BOOTSTRAP", bootstrapClusterProxy.GetKubeconfigPath())
			},
			PostNamespaceCreated:        postNamespaceCreated,
			PreUpgrade:                  preUpgrade,
			PreCleanupManagementCluster: preCleanupManagementCluster,
			MgmtFlavor:                  osType,
			WorkloadFlavor:              osType,
		}
	})
	AfterEach(func() {
		// Recreate bmh that was used in capi namespace in metal3
		//#nosec G204 -- We need to pass in the file name here.
		cmd := exec.Command("bash", "-c", "kubectl apply -f bmhosts_crs.yaml  -n metal3")
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.Logf("Applying bmh to metal3 namespace : \n %v", string(output))
		Expect(err).ToNot(HaveOccurred())
		// wait for all bmh to become available
		bootstrapClient := bootstrapClusterProxy.GetClient()
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))
		WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
			Client:    bootstrapClient,
			Options:   []client.ListOption{client.InNamespace(namespace)},
			Replicas:  5,
			Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
		})
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))
	})
})

// postNamespaceCreated is a hook function that should be called from ClusterctlUpgradeSpec after creating
// the namespace, it creates the needed bmhs in namespace hosting the cluster.
func postNamespaceCreated(clusterProxy framework.ClusterProxy, clusterNamespace string) {
	// Check which from which cluster creation this call is coming
	// if isBootstrapProxy==true then this call when creating the management else we are creating the workload.
	isBootstrapProxy := !strings.HasPrefix(clusterProxy.GetName(), "clusterctl-upgrade")
	if isBootstrapProxy {
		// remove existing bmh from source and apply first 2 in target
		bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.Logf("remove existing bmh from source")
		cmd := exec.Command("bash", "-c", "kubectl delete -f bmhosts_crs.yaml  -n metal3")
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.Logf("Remove existing bmhs:\n %v", string(output))
		Expect(err).ToNot(HaveOccurred())

		// Apply secrets and bmhs for [node_0 and node_1] in the management cluster to host the target management cluster
		for i := 0; i < 2; i++ {
			resource, err := os.ReadFile(filepath.Join(workDir, fmt.Sprintf("bmhs/node_%d.yaml", i)))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(clusterProxy.Apply(ctx, resource, []string{"-n", clusterNamespace}...)).ShouldNot(HaveOccurred())
		}
	} else {
		// Apply secrets and bmhs for [node_2, node_3 and node_4] in the management cluster to host workload cluster
		for i := 2; i < 5; i++ {
			resource, err := os.ReadFile(filepath.Join(workDir, fmt.Sprintf("bmhs/node_%d.yaml", i)))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(clusterProxy.Apply(ctx, resource, []string{"-n", clusterNamespace}...)).ShouldNot(HaveOccurred())
		}
	}
}

// preInitFunc hook function that should be called from ClusterctlUpgradeSpec before init the management cluster
// it installs certManager, BMO and Ironic and overrides the default IPs for the workload cluster.
func preInitFunc(clusterProxy framework.ClusterProxy) {
	installCertManager := func(clusterProxy framework.ClusterProxy) {
		certManagerLink := fmt.Sprintf("https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml", config.CertManagerDefaultVersion)
		err := DownloadFile("/tmp/certManager.yaml", certManagerLink)
		Expect(err).ToNot(HaveOccurred(), "Unable to download certmanager manifest")
		certManagerYaml, err := os.ReadFile("/tmp/certManager.yaml")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(clusterProxy.Apply(ctx, certManagerYaml)).ShouldNot(HaveOccurred())

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
			}, e2eConfig.GetIntervals(specName, "wait-deployment")...)
		}
	}

	By("Fetch manifest for bootstrap cluster")
	path := filepath.Join(os.Getenv("CAPM3PATH"), "scripts")
	cmd := exec.Command("./fetch_manifests.sh") // #nosec G204:gosec
	cmd.Dir = path
	_ = cmd.Run()

	By("Fetch target cluster kubeconfig for target cluster log collection")
	kconfigPathWorkload := clusterProxy.GetKubeconfigPath()
	os.Setenv("KUBECONFIG_WORKLOAD", kconfigPathWorkload)
	bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.Logf("Save kubeconfig in temp folder for project-infra target log collection")
	kubeconfigPathTemp := "/tmp/kubeconfig-test1.yaml"
	cmd = exec.Command("cp", kconfigPathWorkload, kubeconfigPathTemp) // #nosec G204:gosec
	stdoutStderr, er := cmd.CombinedOutput()
	bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.Logf("%s\n", stdoutStderr)
	Expect(er).ToNot(HaveOccurred(), "Cannot fetch target cluster kubeconfig")
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

	// install ironic
	By("Install Ironic in the target cluster")
	installIronicBMO(ctx, func() installIronicBMOInput {
		return installIronicBMOInput{
			ManagementCluster:          clusterProxy,
			BMOPath:                    e2eConfig.GetVariable(bmoPath),
			deployIronic:               true,
			deployBMO:                  false,
			deployIronicTLSSetup:       getBool(e2eConfig.GetVariable(ironicTLSSetup)),
			deployIronicBasicAuth:      getBool(e2eConfig.GetVariable(ironicBasicAuth)),
			deployIronicKeepalived:     getBool(e2eConfig.GetVariable(ironicKeepalived)),
			deployIronicMariadb:        getBool(e2eConfig.GetVariable(ironicMariadb)),
			Namespace:                  e2eConfig.GetVariable(ironicNamespace),
			NamePrefix:                 e2eConfig.GetVariable(NamePrefix),
			RestartContainerCertUpdate: getBool(e2eConfig.GetVariable(restartContainerCertUpdate)),
			E2EConfig:                  e2eConfig,
			SpecName:                   specName,
		}
	})

	// install bmo
	By("Install BMO")
	installIronicBMO(ctx, func() installIronicBMOInput {
		return installIronicBMOInput{
			ManagementCluster:          clusterProxy,
			BMOPath:                    e2eConfig.GetVariable(bmoPath),
			deployIronic:               false,
			deployBMO:                  true,
			deployIronicTLSSetup:       getBool(e2eConfig.GetVariable(ironicTLSSetup)),
			deployIronicBasicAuth:      getBool(e2eConfig.GetVariable(ironicBasicAuth)),
			deployIronicKeepalived:     getBool(e2eConfig.GetVariable(ironicKeepalived)),
			deployIronicMariadb:        getBool(e2eConfig.GetVariable(ironicMariadb)),
			Namespace:                  e2eConfig.GetVariable(ironicNamespace),
			NamePrefix:                 e2eConfig.GetVariable(NamePrefix),
			RestartContainerCertUpdate: getBool(e2eConfig.GetVariable(restartContainerCertUpdate)),
			E2EConfig:                  e2eConfig,
			SpecName:                   specName,
		}
	})

	// Export capi/capm3 versions
	os.Setenv("CAPI_VERSION", "v1beta1")
	os.Setenv("CAPM3_VERSION", "v1beta1")

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
	if CurrentSpecReport().Failed() {
		// Fetch logs in case of failure in management cluster
		By("Fetch logs from management cluster")
		path := filepath.Join(os.Getenv("CAPM3PATH"), "scripts")
		cmd := exec.Command("./fetch_target_logs.sh") // #nosec G204:gosec
		cmd.Dir = path
		errorPipe, _ := cmd.StderrPipe()
		_ = cmd.Start()
		errorData, _ := io.ReadAll(errorPipe)
		if len(errorData) > 0 {
			bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.Logf("Error of the shell: %v\n", string(errorData))
		}
	}
	// Fetch logs from management cluster
	By("Fetch logs from management cluster")
	path := filepath.Join(os.Getenv("CAPM3PATH"), "scripts")
	cmd := exec.Command("./fetch_target_logs.sh") //#nosec G204:gosec
	cmd.Dir = path
	errorPipe, _ := cmd.StderrPipe()
	_ = cmd.Start()
	errorData, _ := io.ReadAll(errorPipe)
	if len(errorData) > 0 {
		bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.bmo_e2e.Logf("Error of the shell: %v\n", string(errorData))
	}
	os.Unsetenv("KUBECONFIG_WORKLOAD")
	os.Unsetenv("KUBECONFIG_BOOTSTRAP")

	// Reinstall ironic
	reInstallIronic := func() {
		By("Reinstate Ironic containers and BMH")
		ephemeralCluster := os.Getenv("EPHEMERAL_CLUSTER")
		if ephemeralCluster == Kind {
			By("Install Ironic in the source cluster as containers")
			bmoPath := e2eConfig.GetVariable("BMOPATH")
			ironicCommand := bmoPath + "/tools/run_local_ironic.sh"
			//#nosec G204 -- We take the BMOPATH from a variable.
			cmd := exec.Command("sh", "-c", "export CONTAINER_RUNTIME=docker; "+ironicCommand)
			stdoutStderr, err := cmd.CombinedOutput()
			fmt.Printf("%s\n", stdoutStderr)
			Expect(err).ToNot(HaveOccurred(), "Cannot run local ironic")
		} else {
			By("Install Ironic in the source cluster as deployments")
			installIronicBMO(ctx, func() installIronicBMOInput {
				return installIronicBMOInput{
					ManagementCluster:          bootstrapClusterProxy,
					BMOPath:                    e2eConfig.GetVariable(bmoPath),
					deployIronic:               true,
					deployBMO:                  false,
					deployIronicTLSSetup:       getBool(e2eConfig.GetVariable(ironicTLSSetup)),
					deployIronicBasicAuth:      getBool(e2eConfig.GetVariable(ironicBasicAuth)),
					deployIronicKeepalived:     getBool(e2eConfig.GetVariable(ironicKeepalived)),
					deployIronicMariadb:        getBool(e2eConfig.GetVariable(ironicMariadb)),
					Namespace:                  e2eConfig.GetVariable(ironicNamespace),
					NamePrefix:                 e2eConfig.GetVariable(NamePrefix),
					RestartContainerCertUpdate: getBool(e2eConfig.GetVariable(restartContainerCertUpdate)),
					E2EConfig:                  e2eConfig,
					SpecName:                   specName,
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
