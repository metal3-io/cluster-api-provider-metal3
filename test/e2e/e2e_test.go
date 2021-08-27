package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/jinzhu/copier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"k8s.io/utils/pointer"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

var (
	ctx                 = context.TODO()
	specName            = "metal3"
	namespace           = "metal3"
	flavorSuffix        string
	cluster             *clusterv1.Cluster
	clusterName         = "test1"
	clusterctlLogFolder string
	cniFile             string
)
var _ = Describe("Workload cluster creation", func() {

	BeforeEach(func() {
		cniFile = "/tmp/calico.yaml"
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))

		if osType == "centos" {
			flavorSuffix = "-centos"
			updateCalico(cniFile, "eth1")
		} else {
			flavorSuffix = ""
			updateCalico(cniFile, "enp2s0")
		}

		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)

		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, cluster, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})

	Context("Creating a highly available control-plane cluster", func() {
		It("Should create a cluster with 3 control-plane and 1 worker nodes", func() {
			By("Creating a high available cluster")
			result := &clusterctl.ApplyClusterTemplateAndWaitResult{}
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ha" + flavorSuffix,
					Namespace:                namespace,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(3),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				CNIManifestPath:              e2eTestsPath + cniFile,
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
			cluster = result.Cluster
			pivoting()
		})
	})
})

func updateCalico(calicoYaml, calicoInterface string) {
	// cniYaml, err := os.ReadFile("/tmp/calico.yaml.bk")\
	err := downloadFile(calicoYaml, "https://docs.projectcalico.org/manifests/calico.yaml")
	Expect(err).To(BeNil(), "Unable to download Calico manifest")
	cniYaml, err := os.ReadFile(calicoYaml)
	Expect(err).To(BeNil(), "Unable to read Calico manifest")
	// TODO: Delete below
	// podCIDR := "192.168.0.0/18"
	podCIDR := os.Getenv("POD_CIDR")
	cniYaml = []byte(strings.Replace(string(cniYaml), "192.168.0.0/16", podCIDR, -1))

	yamlDocuments, err := splitYAML(cniYaml)
	Expect(err).To(BeNil(), "Cannot unmarshal the calico yaml elements to golang objects")
	// err = yaml.Unmarshal([]byte(cniYaml), yamlNode)

	//TODO: check error
	calicoNodes, err := yamlContainKeyValue(yamlDocuments, "calico-node", "metadata", "labels", "k8s-app")
	Expect(err).To(BeNil())
	// In this test, there should be only one document node that needs to be updated
	calicoNode := calicoNodes[0]

	calicoNodeSpecTemplateSpec, err := yamlFindByValue(calicoNode, "spec", "template", "spec", "containers")
	Expect(err).To(BeNil())
	// debugYaml, err2 := yaml.Marshal(calicoNodeSpecTemplateSpec)
	// if err2 != nil {
	// 	panic(err2)
	// }
	// fmt.Println(string(debugYaml))

	calicoNodeContainers, err := yamlContainKeyValue(calicoNodeSpecTemplateSpec.Content, "calico-node", "name")
	Expect(err).To(BeNil())

	// In this test, there should be only one container in the yaml node that needs to be updated
	calicoNodeContainer := calicoNodeContainers[0]
	// debugYaml, err2 := yaml.Marshal(calicoNodeContainer)
	// if err2 != nil {
	// 	panic(err2)
	// }
	// fmt.Println(string(debugYaml))
	calicoNodeContainerEnvs, err := yamlFindByValue(calicoNodeContainer, "env")
	Expect(err).To(BeNil())
	// debugYaml, err2 := yaml.Marshal(calicoNodeContainerEnvs)
	// if err2 != nil {
	// 	panic(err2)
	// }
	// fmt.Println(string(debugYaml))
	addItem := &yaml.Node{}
	copier.CopyWithOption(addItem, calicoNodeContainerEnvs.Content[0], copier.Option{IgnoreEmpty: true, DeepCopy: true})
	addItem.Content[1].SetString("IP_AUTODETECTION_METHOD")
	addItem.Content[3].SetString("interface=" + calicoInterface)
	addItem.HeadComment = "Start section modified by CAPM3 e2e test framework"
	addItem.FootComment = "End section modified by CAPM3 e2e test framework"
	calicoNodeContainerEnvs.Content = append(calicoNodeContainerEnvs.Content, addItem)
	yamlOut, err := printYaml(yamlDocuments)
	Expect(err).To(BeNil())
	// fmt.Println(string(yamlOut))

	err = os.WriteFile(calicoYaml, yamlOut, 0664)
	Expect(err).To(BeNil(), "Cannot print out the update to the file")
}
