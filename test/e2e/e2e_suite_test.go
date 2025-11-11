package e2e

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/jinzhu/copier"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	irsov1alpha1 "github.com/metal3-io/ironic-standalone-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KubernetesVersion = "KUBERNETES_VERSION"
)

// Test suite flags.
var (
	// configPath is the path to the e2e config file.
	configPath string

	// useExistingCluster instructs the test to use the current cluster instead of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// artifactFolder is the folder to store e2e test artifacts.
	artifactFolder string

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool

	// upgradeTest triggers only e2e upgrade test if true.
	upgradeTest bool
)

// Test suite global vars.
var (
	// e2eConfig to be used for this test, read from configPath.
	e2eConfig *clusterctl.E2EConfig

	// clusterctlConfigPath to be used for this test, created by generating a clusterctl local repository
	// with the providers specified in the configPath.
	clusterctlConfigPath string

	// bootstrapClusterProvider manages provisioning of the bootstrap cluster to be used for the e2e tests.
	// Please note that provisioning will be skipped if e2e.use-existing-cluster is provided.
	bootstrapClusterProvider bootstrap.ClusterProvider

	// bootstrapClusterProxy allows to interact with the bootstrap cluster to be used for the e2e tests.
	bootstrapClusterProxy framework.ClusterProxy

	osType string

	kubeconfigPath string
	e2eTestsPath   string

	numberOfControlplane int
	numberOfWorkers      int
	numberOfAllBmh       int
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "", "path to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "", "folder where e2e test artifact should be stored")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.BoolVar(&upgradeTest, "e2e.trigger-upgrade-test", false, "if true, the e2e upgrade test will be triggered and other tests will be skipped")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", true, "if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
	flag.StringVar(&kubeconfigPath, "e2e.kubeconfig-path", os.Getenv("HOME")+"/.kube/config", "if e2e.use-existing-cluster is true, path to the kubeconfig file")
	e2eTestsPath = getE2eTestsPath()

	osType = strings.ToLower(os.Getenv("OS"))
}

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}

// Using a SynchronizedBeforeSuite for controlling how to create resources shared across ParallelNodes (~ginkgo threads).
// The local clusterctl repository & the bootstrap cluster are created once and shared across all the tests.
var _ = SynchronizedBeforeSuite(func() []byte {
	// Before all ParallelNodes.

	Expect(configPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.RemoveAll(artifactFolder)).To(Succeed())
	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder)

	By("Initializing a runtime.Scheme with all the GVK relevant for this test")
	scheme := initScheme()
	ctrl.SetLogger(klog.Background())

	By(fmt.Sprintf("Loading the e2e test configuration from %q", configPath))
	e2eConfig = loadE2EConfig(configPath)
	numberOfControlplane = int(*e2eConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
	numberOfWorkers = int(*e2eConfig.MustGetInt32PtrVariable("WORKER_MACHINE_COUNT"))
	numberOfAllBmh = numberOfControlplane + numberOfWorkers

	By(fmt.Sprintf("Creating a clusterctl local repository into %q", artifactFolder))
	clusterctlConfigPath = CreateClusterctlLocalRepository(e2eConfig, filepath.Join(artifactFolder, "repository"))

	By("Setting up the bootstrap cluster")
	bootstrapClusterProvider, bootstrapClusterProxy = SetupBootstrapCluster(e2eConfig, scheme, useExistingCluster)

	By("Initializing the bootstrap cluster")
	InitBootstrapCluster(bootstrapClusterProxy, e2eConfig, clusterctlConfigPath, artifactFolder)

	return []byte(
		strings.Join([]string{
			artifactFolder,
			configPath,
			clusterctlConfigPath,
			bootstrapClusterProxy.GetKubeconfigPath(),
		}, ","),
	)
}, func(data []byte) {
	// Before each ParallelNode.
	parts := strings.Split(string(data), ",")
	Expect(parts).To(HaveLen(4))

	artifactFolder = parts[0]
	configPath = parts[1]
	clusterctlConfigPath = parts[2]
	kubeconfigPath = parts[3]

	e2eConfig = loadE2EConfig(configPath)
	withMetal3LogCollectorOpt := framework.WithMachineLogCollector(Metal3LogCollector{})
	bootstrapClusterProxy = framework.NewClusterProxy("bootstrap", kubeconfigPath, initScheme(), withMetal3LogCollectorOpt)
})

// Using a SynchronizedAfterSuite for controlling how to delete resources shared across ParallelNodes (~ginkgo threads).
// The bootstrap cluster is shared across all the tests, so it should be deleted only after all ParallelNodes completes.
// The local clusterctl repository is preserved like everything else created into the artifact folder.
var _ = SynchronizedAfterSuite(func() {
	// After each ParallelNode.
}, func() {
	// After all ParallelNodes.
	By("Tearing down the management cluster")
	if !skipCleanup {
		TearDown(bootstrapClusterProvider, bootstrapClusterProxy)
	}
})

func initScheme() *runtime.Scheme {
	sc := runtime.NewScheme()
	framework.TryAddDefaultSchemes(sc)
	Expect(bmov1alpha1.AddToScheme(sc)).To(Succeed())
	Expect(infrav1.AddToScheme(sc)).To(Succeed())
	Expect(ipamv1.AddToScheme(sc)).To(Succeed())
	Expect(irsov1alpha1.AddToScheme(sc)).To(Succeed())
	Expect(clusterv1beta1.AddToScheme(sc)).To(Succeed())
	Expect(clusterv1.AddToScheme(sc)).To(Succeed())

	return sc
}

func loadE2EConfig(configPath string) *clusterctl.E2EConfig {
	config := clusterctl.LoadE2EConfig(context.TODO(), clusterctl.LoadE2EConfigInput{ConfigPath: configPath})
	Expect(config).ToNot(BeNil(), "Failed to load E2E config from %s", configPath)

	return config
}

func CreateClusterctlLocalRepository(config *clusterctl.E2EConfig, repositoryFolder string) string {
	createRepositoryInput := clusterctl.CreateRepositoryInput{
		E2EConfig:        config,
		RepositoryFolder: repositoryFolder,
	}

	// Ensuring a CNI file is defined in the config and register a FileTransformation to inject the referenced file as in place of the CNI_RESOURCES envSubst variable.
	Expect(config.Variables).To(HaveKey(capi_e2e.CNIPath), "Missing %s variable in the config", capi_e2e.CNIPath)
	cniPath := config.MustGetVariable(capi_e2e.CNIPath)

	cniProvider := config.MustGetVariable("CNI_PROVIDER")

	cniInterface := "enp2s0"
	if osType == osTypeLeap {
		cniInterface = "eth1"
	}
	switch cniProvider {
	case "cilium":
		updateCilium(config, cniPath)
	case "calico":
		updateCalico(config, cniPath, cniInterface)
	default:
		Expect(cniProvider).To(Or(Equal("calico"), Equal("cilium")), "Invalid CNI type %q, only 'cilium' and 'calico' are supported", cniProvider)
	}
	Expect(cniPath).To(BeAnExistingFile(), "The %s variable should resolve to an existing file", capi_e2e.CNIPath)
	createRepositoryInput.RegisterClusterResourceSetConfigMapTransformation(cniPath, capi_e2e.CNIResources)

	clusterctlConfig := clusterctl.CreateRepository(context.TODO(), createRepositoryInput)
	Expect(clusterctlConfig).To(BeAnExistingFile(), "The clusterctl config file does not exists in the local repository %s", repositoryFolder)

	return clusterctlConfig
}

func SetupBootstrapCluster(config *clusterctl.E2EConfig, scheme *runtime.Scheme, useExistingCluster bool) (bootstrap.ClusterProvider, framework.ClusterProxy) {
	var clusterProvider bootstrap.ClusterProvider
	if !useExistingCluster {
		clusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(context.TODO(), bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               config.ManagementClusterName,
			RequiresDockerSock: config.HasDockerProvider(),
			Images:             config.Images,
			LogFolder:          filepath.Join(artifactFolder, "kind", bootstrapClusterProxy.GetName()),
		})
		Expect(clusterProvider).ToNot(BeNil(), "Failed to create a bootstrap cluster")

		kubeconfigPath = clusterProvider.GetKubeconfigPath()
		Expect(kubeconfigPath).To(BeAnExistingFile(), "Failed to get the kubeconfig file for the bootstrap cluster")
	}

	clusterProxy := framework.NewClusterProxy("bootstrap", kubeconfigPath, scheme)
	Expect(clusterProxy).ToNot(BeNil(), "Failed to get a bootstrap cluster proxy")

	return clusterProvider, clusterProxy
}

func InitBootstrapCluster(bootstrapClusterProxy framework.ClusterProxy, config *clusterctl.E2EConfig, clusterctlConfig, artifactFolder string) {
	clusterctl.InitManagementClusterAndWatchControllerLogs(context.TODO(), clusterctl.InitManagementClusterAndWatchControllerLogsInput{
		ClusterProxy:            bootstrapClusterProxy,
		ClusterctlConfigPath:    clusterctlConfig,
		InfrastructureProviders: config.InfrastructureProviders(),
		IPAMProviders:           config.IPAMProviders(),
		LogFolder:               filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
	}, config.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
}

func TearDown(bootstrapClusterProvider bootstrap.ClusterProvider, bootstrapClusterProxy framework.ClusterProxy) {
	if bootstrapClusterProxy != nil {
		bootstrapClusterProxy.Dispose(context.TODO())
	}
	if bootstrapClusterProvider != nil {
		bootstrapClusterProvider.Dispose(context.TODO())
	}
}

func getE2eTestsPath() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func validateGlobals(specName string) {
	Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
	Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))
	Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
	Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(osType).ToNot(Equal(""))
	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)
}

func updateCalico(config *clusterctl.E2EConfig, calicoYaml, calicoInterface string) {
	calicoManifestURL := fmt.Sprintf("https://raw.githubusercontent.com/projectcalico/calico/%s/manifests/calico.yaml", config.MustGetVariable("CALICO_VERSION"))
	err := DownloadFile(calicoYaml, calicoManifestURL)
	Expect(err).ToNot(HaveOccurred(), "Unable to download Calico manifest")
	cniYaml, err := os.ReadFile(calicoYaml)
	Expect(err).ToNot(HaveOccurred(), "Unable to read Calico manifest")

	Logf("Replace the default CIDR with the one set in $POD_CIDR")
	podCIDR := config.MustGetVariable("POD_CIDR")
	calicoContainerRegistry := config.MustGetVariable("DOCKER_HUB_PROXY")
	// Uncomment the CALICO_IPV4POOL_CIDR environment variable
	cniYaml = []byte(strings.ReplaceAll(string(cniYaml), "# - name: CALICO_IPV4POOL_CIDR", "- name: CALICO_IPV4POOL_CIDR"))
	cniYaml = []byte(strings.ReplaceAll(string(cniYaml), "#   value: \"192.168.0.0/16\"", "  value: \""+podCIDR+"\""))
	cniYaml = []byte(strings.ReplaceAll(string(cniYaml), "docker.io", calicoContainerRegistry))

	yamlDocuments, err := splitYAML(cniYaml)
	Expect(err).ToNot(HaveOccurred(), "Cannot unmarshal the calico yaml elements to golang objects")
	calicoNodes, err := yamlContainKeyValue(yamlDocuments, "calico-node", "metadata", "labels", "k8s-app")
	Expect(err).ToNot(HaveOccurred())
	for _, calicoNode := range calicoNodes {
		var calicoNodeSpecTemplateSpec, calicoNodeContainerEnvs *yaml.Node
		var calicoNodeContainers []*yaml.Node

		calicoNodeSpecTemplateSpec, err = yamlFindByValue(calicoNode, "spec", "template", "spec", "containers")
		Expect(err).ToNot(HaveOccurred())
		calicoNodeContainers, err = yamlContainKeyValue(calicoNodeSpecTemplateSpec.Content, "calico-node", "name")
		Expect(err).ToNot(HaveOccurred())
		// Since we find the container by name, we expect to get only one container.
		Expect(calicoNodeContainers).To(HaveLen(1), "Found 0 or more than 1 container with name `calico-node`")
		calicoNodeContainer := calicoNodeContainers[0]
		calicoNodeContainerEnvs, err = yamlFindByValue(calicoNodeContainer, "env")
		Expect(err).ToNot(HaveOccurred())
		addItem := &yaml.Node{}
		err = copier.CopyWithOption(addItem, calicoNodeContainerEnvs.Content[0], copier.Option{IgnoreEmpty: true, DeepCopy: true})
		Expect(err).ToNot(HaveOccurred())
		addItem.Content[1].SetString("IP_AUTODETECTION_METHOD")
		addItem.Content[3].SetString("interface=" + calicoInterface)
		addItem.HeadComment = "Start section modified by CAPM3 e2e test framework"
		addItem.FootComment = "End section modified by CAPM3 e2e test framework"
		calicoNodeContainerEnvs.Content = append(calicoNodeContainerEnvs.Content, addItem)
	}

	yamlOut, err := printYaml(yamlDocuments)
	Expect(err).ToNot(HaveOccurred())
	err = os.WriteFile(calicoYaml, yamlOut, 0600)
	Expect(err).ToNot(HaveOccurred(), "Cannot print out the update to the file")
}

// updateCilium generates and writes a Cilium CNI manifest to the CNI path specified in e2e config.
// It retrieves the Cilium version from e2e configuration, downloads the corresponding Helm chart, generates a manifest from the chart template, and writes the manifest to the CNI path.
func updateCilium(config *clusterctl.E2EConfig, cniPath string) {
	ctx = context.Background()
	ciliumVersion := config.MustGetVariable("CILIUM_VERSION")
	if ciliumVersion[0] == 'v' {
		ciliumVersion = ciliumVersion[1:]
	}
	settings := cli.New()
	settings.SetNamespace("kube-system")
	helmDriver := os.Getenv("HELM_DRIVER")
	opts := HelmOpts{
		Logger:         log.Default(),
		Settings:       settings,
		ReleaseName:    "cilium",
		ChartRef:       fmt.Sprintf("https://helm.cilium.io/cilium-%s.tgz", ciliumVersion),
		ChartLocation:  fmt.Sprintf("/tmp/cilium-%s.tgz", ciliumVersion),
		ReleaseVersion: semver.MustParse(ciliumVersion),
		Driver:         helmDriver,
	}

	manifestOverwriteValues := map[string]interface{}{
		"operator": map[string]interface{}{
			"replicas": 1,
			"updateStrategy": map[string]interface{}{
				"rollingUpdate": map[string]interface{}{
					"maxUnavailable": "100%",
				},
			},
		},
	}

	manifest, err := generateTemplateFromHelmChart(ctx, opts, manifestOverwriteValues, e2eConfig)
	Expect(err).ToNot(HaveOccurred(), "failed to generate template: %v", err)

	// Replace ${BIN_PATH} with /opt/cni/bin. This is done to prevent
	// framework.RegisterClusterResourceSetConfigMapTransformation from throwing
	// an error due to unresolvable "envsubst" variable.
	manifest = strings.ReplaceAll(manifest, "${BIN_PATH}", "/opt/cni/bin")

	containerRegistry := config.MustGetVariable("CONTAINER_REGISTRY")
	manifest = strings.ReplaceAll(manifest, "quay.io", containerRegistry)

	err = os.WriteFile(cniPath, []byte(manifest), 0600)
	Expect(err).ToNot(HaveOccurred(), "Failed to write Cilium manifest to file: %v", err)
}

// createBMHsInNamespace is a hook function that can be called after creating
// a namespace, it creates the needed bmhs in the namespace hosting the cluster.
func createBMHsInNamespace(clusterProxy framework.ClusterProxy, clusterNamespace string) {
	// Apply secrets and bmhs for all nodes in the cluster to host the target cluster
	nodes := int(*e2eConfig.MustGetInt32PtrVariable("NUM_NODES"))
	for i := range nodes {
		resource, err := os.ReadFile(filepath.Join(workDir, fmt.Sprintf("bmhs/node_%d.yaml", i)))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(CreateOrUpdateWithNamespace(ctx, clusterProxy, resource, clusterNamespace)).ShouldNot(HaveOccurred())
	}
	clusterClient := clusterProxy.GetClient()
	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(clusterNamespace))
	WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(clusterNamespace)},
		Replicas:  nodes,
		Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
	})
	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(clusterNamespace))

	ListBareMetalHosts(ctx, bootstrapClusterProxy.GetClient(), client.InNamespace(clusterNamespace))
}
