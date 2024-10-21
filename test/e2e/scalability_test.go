package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
 * This test apply cluster templates on fake environments to test scalability
 * apply scaled number of BMHs per batches and wait for the them to become available
 * When all the BMHs become available it apply the cluster templates
 */

var _ = Describe("When testing scalability with fakeIPA and FKAS [scalability][basic]", Label("scalability", "basic"), func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
		specName = "scale"
		namespace = "scale"
		numberOfWorkers = int(*e2eConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
		numberOfControlplane = int(*e2eConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
		createFKASResources()
	})

	// Create the cluster
	capi_e2e.ScaleSpec(ctx, func() capi_e2e.ScaleSpecInput {
		return capi_e2e.ScaleSpecInput{
			E2EConfig:                         e2eConfig,
			ClusterctlConfigPath:              clusterctlConfigPath,
			InfrastructureProvider:            ptr.To("metal3"),
			BootstrapClusterProxy:             bootstrapClusterProxy,
			ArtifactFolder:                    artifactFolder,
			SkipCleanup:                       skipCleanup,
			ClusterCount:                      ptr.To[int64](5),
			Concurrency:                       ptr.To[int64](2),
			Flavor:                            ptr.To(fmt.Sprintf("%s-fake", osType)),
			ControlPlaneMachineCount:          ptr.To[int64](int64(numberOfControlplane)),
			MachineDeploymentCount:            ptr.To[int64](1),
			WorkerMachineCount:                ptr.To[int64](int64(numberOfWorkers)),
			PostScaleClusterNamespaceCreated:  postScaleClusterNamespaceCreated,
			DeployClusterInSeparateNamespaces: true,
		}
	})

	AfterEach(func() {
		FKASKustomization := e2eConfig.GetVariable("FKAS_RELEASE_LATEST")
		By(fmt.Sprintf("Removing FKAS from kustomization %s from the bootsrap cluster", FKASKustomization))
		BuildAndRemoveKustomization(ctx, FKASKustomization, bootstrapClusterProxy)
		DumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})
})

func registerFKASCluster(cn string, ns string) (string, int) {
	/*
	* this function reads the certificate from the cluster secrets
	 */
	type FKASCluster struct {
		Cluster   string `json:"cluster"`
		Namespace string `json:"namespace"`
	}
	type Endpoint struct {
		Host string
		Port int
	}

	fkasCluster := FKASCluster{
		Cluster:   cn,
		Namespace: ns,
	}
	marshalled, err := json.Marshal(fkasCluster)
	if err != nil {
		Logf("impossible to marshall fkasCluster: %s", err)
	}
	// send the request
	cluster_endpoints, err := http.Post("http://172.22.0.2:3333/register", "application/json", bytes.NewReader(marshalled))
	Expect(err).NotTo(HaveOccurred())
	defer cluster_endpoints.Body.Close()
	body, err := ioutil.ReadAll(cluster_endpoints.Body)
	Expect(err).NotTo(HaveOccurred())
	var response Endpoint
	json.Unmarshal(body, &response)
	return response.Host, response.Port
}

func createFKASResources() {
	FKASDeployLogFolder := filepath.Join(os.TempDir(), "fkas-deploy-logs", bootstrapClusterProxy.GetName())
	FKASKustomization := e2eConfig.GetVariable("FKAS_RELEASE_LATEST")
	By(fmt.Sprintf("Installing FKAS from kustomization %s on the bootsrap cluster", FKASKustomization))
	err := BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       FKASKustomization,
		ClusterProxy:        bootstrapClusterProxy,
		WaitForDeployment:   true,
		WatchDeploymentLogs: true,
		LogPath:             FKASDeployLogFolder,
		DeploymentName:      "metal3-fkas-system",
		DeploymentNamespace: "fkas-system",
		WaitIntervals:       e2eConfig.GetIntervals("default", "wait-deployment"),
	})
	Expect(err).NotTo(HaveOccurred())
}

func LogToFile(logFile string, data []byte) {
	err := ioutil.WriteFile(filepath.Clean(logFile), data, 0644)
	Expect(err).ToNot(HaveOccurred(), "Cannot log to file")
}

// the function to create the bmh needed in the namesace and call fkas to prepare a fakecluster and update templates accordingly
// PostNamespaceCreated
// (managementClusterProxy framework.ClusterProxy, namespace, clusterName string, clusterTemplateYAML []byte){.
func postScaleClusterNamespaceCreated(clusterProxy framework.ClusterProxy, clusterNamespace string, clusterName string, baseClusterTemplateYAML []byte) (template []byte) {
	c := clusterProxy.GetClient()

	getClusterId := func(clusterName string) (index int) {
		re := regexp.MustCompile("[0-9]+$")
		index, _ = strconv.Atoi(string((re.Find([]byte(clusterName)))))
		return
	}

	getBmhsCountNeeded := func() (sum int) {
		numberOfWorkers = int(*e2eConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
		numberOfControlplane = int(*e2eConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
		sum = numberOfWorkers + numberOfControlplane
		return
	}

	getBmhsFromToIndex := func(clusterIndex int, bmhCount int) (from int, to int) {
		to = clusterIndex*bmhCount - 1
		from = to - bmhCount + 1
		return
	}

	applyBmhsByBatch := func(batchSize int, from int, to int) {
		applyBatchBmh := func(from int, to int) {
			bmhsNameList := make([]string, 0, to-from+1)
			Logf("Apply BMH batch from node_%d to node_%d", from, to)
			for i := from; i < to+1; i++ {
				bmhsNameList = append(bmhsNameList, fmt.Sprintf("node-%d", i))
				resource, err := os.ReadFile(filepath.Join(workDir, fmt.Sprintf("bmhs/node_%d.yaml", i)))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(CreateOrUpdateWithNamespace(ctx, clusterProxy, resource, clusterNamespace)).ShouldNot(HaveOccurred())
			}
			// I need to get a list of bmh names we are getting bmhlist to avoid http request one by one
			// TODO (mboukhalfa) if the clusters are using the same namespace then might pickup the bmh from the list
			// that we are waiting to become available aby another cluster then the bmh number avialble will never reached
			Logf("Waiting for BMHs from node_%d to node_%d to become available", from, to)
			Eventually(func(g Gomega) {
				bmhList := bmov1alpha1.BareMetalHostList{}
				g.Expect(c.List(ctx, &bmhList, []client.ListOption{client.InNamespace(clusterNamespace)}...)).To(Succeed())
				g.Expect(FilterAvialableBmhsName(bmhList.Items, bmhsNameList, bmov1alpha1.StateAvailable)).To(HaveLen(to - from + 1))
			}, e2eConfig.GetIntervals(specName, "wait-bmh-available")...).Should(Succeed())
			ListBareMetalHosts(ctx, c, []client.ListOption{client.InNamespace(clusterNamespace)}...)
		}
		for i := from; i <= to; i += batchSize {
			if i+batchSize > to {
				applyBatchBmh(i, to)
				break
			}
			applyBatchBmh(i, i+batchSize-1)
		}
	}
	index := getClusterId(clusterName)
	cn := getBmhsCountNeeded()
	f, t := getBmhsFromToIndex(index, cn)
	batch, _ := strconv.Atoi(e2eConfig.GetVariable("BMH_BATCH_SIZE"))
	applyBmhsByBatch(batch, f, t)

	h, p := registerFKASCluster(clusterName, clusterNamespace)
	clusterTemplateYAML := bytes.Replace(baseClusterTemplateYAML, []byte("CLUSTER_APIENDPOINT_HOST_HOLDER"), []byte(h), -1)
	clusterTemplateYAML = bytes.Replace(clusterTemplateYAML, []byte("CLUSTER_APIENDPOINT_PORT_HOLDER"), []byte(strconv.Itoa(p)), -1)

	return clusterTemplateYAML
}

// FilterAvialableBmhsName returns a filtered list of BaremetalHost objects in certain provisioning state.
func FilterAvialableBmhsName(bmhs []bmov1alpha1.BareMetalHost, bmhsNameList []string, state bmov1alpha1.ProvisioningState) (result []bmov1alpha1.BareMetalHost) {
	for _, bmh := range bmhs {
		for _, name := range bmhsNameList {
			if bmh.Name == name && bmh.Status.Provisioning.State == state {
				result = append(result, bmh)
			}
		}
	}
	return
}
