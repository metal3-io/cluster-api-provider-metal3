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

	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

var (
	ctx                      = context.TODO()
	specName                 = "metal3"
	namespace                = "metal3"
	flavorSuffix             string
	cluster                  *capi.Cluster
	clusterName              = "test1"
	clusterctlLogFolder      string
	cniFile                  string
	targetCluster            framework.ClusterProxy
	controlPlaneMachineCount int64
	workerMachineCount       int64
)
var _ = Describe("Workload cluster creation", func() {

	BeforeEach(func() {
		cniFile = "/tmp/calico.yaml"
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		flavorSuffix = "-m3-dev-env"

		if osType == "centos" {
			updateCalico(cniFile, "eth1")
		} else {
			updateCalico(cniFile, "enp2s0")
		}
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, cluster, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})

	Context("Creating a highly available control-plane cluster", func() {
		It("Should create a cluster with 3 control-plane and 1 worker nodes", func() {
			By("Creating a high available cluster")
			controlPlaneMachineCount = int64(*e2eConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
			workerMachineCount = int64(*e2eConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
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
					ControlPlaneMachineCount: &controlPlaneMachineCount,
					WorkerMachineCount:       &workerMachineCount,
				},
				CNIManifestPath:              cniFile,
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
			cluster = result.Cluster
			targetCluster = bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
			test_remediation()
			pivoting()
			cert_rotation()
			// node_reuse()
		})
	})
})

func updateCalico(calicoYaml, calicoInterface string) {
	err := downloadFile(calicoYaml, "https://docs.projectcalico.org/manifests/calico.yaml")
	Expect(err).To(BeNil(), "Unable to download Calico manifest")
	cniYaml, err := os.ReadFile(calicoYaml)
	Expect(err).To(BeNil(), "Unable to read Calico manifest")
	podCIDR := os.Getenv("POD_CIDR")
	cniYaml = []byte(strings.Replace(string(cniYaml), "192.168.0.0/16", podCIDR, -1))

	yamlDocuments, err := splitYAML(cniYaml)
	Expect(err).To(BeNil(), "Cannot unmarshal the calico yaml elements to golang objects")
	calicoNodes, err := yamlContainKeyValue(yamlDocuments, "calico-node", "metadata", "labels", "k8s-app")
	Expect(err).To(BeNil())
	for _, calicoNode := range calicoNodes {
		calicoNodeSpecTemplateSpec, err := yamlFindByValue(calicoNode, "spec", "template", "spec", "containers")
		Expect(err).To(BeNil())
		calicoNodeContainers, err := yamlContainKeyValue(calicoNodeSpecTemplateSpec.Content, "calico-node", "name")
		Expect(err).To(BeNil())
		// Since we find the container by name, we expect to get only one container.
		Expect(len(calicoNodeContainers) == 1).To(BeTrue(), "Found 0 or more than 1 container with name `calico-node`")
		calicoNodeContainer := calicoNodeContainers[0]
		calicoNodeContainerEnvs, err := yamlFindByValue(calicoNodeContainer, "env")
		Expect(err).To(BeNil())
		addItem := &yaml.Node{}
		err = copier.CopyWithOption(addItem, calicoNodeContainerEnvs.Content[0], copier.Option{IgnoreEmpty: true, DeepCopy: true})
		Expect(err).To(BeNil(), "Cannot copy this object")
		addItem.Content[1].SetString("IP_AUTODETECTION_METHOD")
		addItem.Content[3].SetString("interface=" + calicoInterface)
		addItem.HeadComment = "Start section modified by CAPM3 e2e test framework"
		addItem.FootComment = "End section modified by CAPM3 e2e test framework"
		calicoNodeContainerEnvs.Content = append(calicoNodeContainerEnvs.Content, addItem)
	}

	yamlOut, err := printYaml(yamlDocuments)
	Expect(err).To(BeNil())
	err = os.WriteFile(calicoYaml, yamlOut, 0664)
	Expect(err).To(BeNil(), "Cannot print out the update to the file")
}
