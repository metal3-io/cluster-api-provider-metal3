package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1beta1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
 * This test apply cluster templates on fake environments to test scalability
 * apply scaled number of BMHs per batches and wait for the them to become available
 * When all the BMHs become available it apply the cluster templates
 */

type ClusterRequest struct {
	ClusterName string `json:"cluster"`
	Namespace   string `json:"namespace"`
}

type ClusterAPIServer struct {
	Host string
	Port int
}

var (
	numberOfClusters     int64
	scaleSpecConcurrency int64
)

var _ = Describe("When testing scalability with fakeIPA and FKAS", Label("scalability"), func() {
	BeforeEach(func() {
		osType = strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
		specName = "scale"
		namespace = "scale"
		numberOfWorkers = int(*e2eConfig.MustGetInt32PtrVariable("WORKER_MACHINE_COUNT"))
		numberOfControlplane = int(*e2eConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
		numberOfClusters = int64(*e2eConfig.MustGetInt32PtrVariable("NUM_NODES"))
		scaleSpecConcurrency = int64(*e2eConfig.MustGetInt32PtrVariable("SCALE_SPEC_CONCURRENCY"))
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
		createFKASResources()
		imageURL, imageChecksum := EnsureImage("v1.34.1")
		os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
		os.Setenv("IMAGE_RAW_URL", imageURL)
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
			SkipUpgrade:                       true,
			ClusterCount:                      ptr.To[int64](numberOfClusters),
			Concurrency:                       ptr.To[int64](scaleSpecConcurrency),
			Flavor:                            ptr.To(osType + "-fake"),
			ControlPlaneMachineCount:          ptr.To[int64](int64(numberOfControlplane)),
			MachineDeploymentCount:            ptr.To[int64](0),
			WorkerPerMachineDeploymentCount:   ptr.To[int64](int64(numberOfWorkers)),
			PostScaleClusterNamespaceCreated:  postScaleClusterNamespaceCreated,
			DeployClusterInSeparateNamespaces: ptr.To(true),
		}
	})

	AfterEach(func() {
		FKASKustomization := e2eConfig.MustGetVariable("FKAS_RELEASE_LATEST")
		By(fmt.Sprintf("Removing FKAS from kustomization %s from the bootsrap cluster", FKASKustomization))
		err := BuildAndRemoveKustomization(ctx, FKASKustomization, bootstrapClusterProxy)
		Expect(err).NotTo(HaveOccurred())
	})
})

func registerFKASCluster(ctx context.Context, cn string, ns string) *ClusterAPIServer {
	/*
	* this function reads the certificate from the cluster secrets
	 */

	fkasCluster := ClusterRequest{
		ClusterName: cn,
		Namespace:   ns,
	}
	jsonData, err := json.Marshal(fkasCluster)
	Expect(err).NotTo(HaveOccurred())
	registerEndpoint := "http://172.22.0.2:3333/register"
	// Create the HTTP POST request with the context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerEndpoint, http.NoBody)
	Expect(err).NotTo(HaveOccurred())

	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytes.NewReader(jsonData))

	// Use the default HTTP client to send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	var endpoint ClusterAPIServer
	err = json.Unmarshal(body, &endpoint)
	Expect(err).NotTo(HaveOccurred())
	return &endpoint
}

func createFKASResources() {
	FKASDeployLogFolder := filepath.Join(os.TempDir(), "fkas-deploy-logs", bootstrapClusterProxy.GetName())
	FKASKustomization := e2eConfig.MustGetVariable("FKAS_RELEASE_LATEST")
	By(fmt.Sprintf("Installing FKAS with kustomization %s to the bootstrap cluster", FKASKustomization))
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
	err := ioutil.WriteFile(filepath.Clean(logFile), data, 0600)
	Expect(err).ToNot(HaveOccurred(), "Cannot log to file")
}

// the function to create the bmh needed in the namespace and call fkas to prepare a fakecluster and update templates accordingly.
func postScaleClusterNamespaceCreated(clusterProxy framework.ClusterProxy, clusterNamespace string, clusterName string, baseClusterClassYAML []byte, baseClusterTemplateYAML []byte) (clusterClassTemlate []byte, clusterTemplate []byte) {
	c := clusterProxy.GetClient()

	getClusterID := func(clusterName string) (index int) {
		re := regexp.MustCompile("[0-9]+$")
		index, _ = strconv.Atoi(string((re.Find([]byte(clusterName)))))
		return
	}

	getBmhsCountNeeded := func() (sum int) {
		numberOfWorkers = int(*e2eConfig.MustGetInt32PtrVariable("WORKER_MACHINE_COUNT"))
		numberOfControlplane = int(*e2eConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
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
	index := getClusterID(clusterName)
	cn := getBmhsCountNeeded()
	f, t := getBmhsFromToIndex(index, cn)
	batch, _ := strconv.Atoi(e2eConfig.MustGetVariable("BMH_BATCH_SIZE"))
	applyBmhsByBatch(batch, f, t)

	newClusterEndpoint := registerFKASCluster(ctx, clusterName, clusterNamespace)
	clusterTemplateYAML := bytes.ReplaceAll(baseClusterTemplateYAML, []byte("CLUSTER_APIENDPOINT_HOST_HOLDER"), []byte(newClusterEndpoint.Host))
	clusterTemplateYAML = bytes.ReplaceAll(clusterTemplateYAML, []byte("CLUSTER_APIENDPOINT_PORT_HOLDER"), []byte(strconv.Itoa(newClusterEndpoint.Port)))

	clusterClassYAML, clusterTemplateYAML := extractDataTemplateIppool(baseClusterClassYAML, clusterTemplateYAML)
	clusterTemplateYAML, err := RemoveEmptyWorkers(clusterTemplateYAML)
	Expect(err).ShouldNot(HaveOccurred())
	LogToFile(artifactFolder+"/"+clusterName+"-cluster.log", clusterTemplateYAML)
	LogToFile(artifactFolder+"/"+clusterName+"-clusterclass.log", clusterClassYAML)
	return clusterClassYAML, clusterTemplateYAML
}

func extractDataTemplateIppool(baseClusterClassYAML []byte, baseClusterTemplateYAML []byte) (clusterClassTemlate []byte, clusterTemplate []byte) {
	clusterClassObjs, err := yaml.ToUnstructured(baseClusterClassYAML)
	Expect(err).ToNot(HaveOccurred())
	newClusterClassObjs := []unstructured.Unstructured{}
	clusterObjs, err := yaml.ToUnstructured(baseClusterTemplateYAML)
	Expect(err).ToNot(HaveOccurred())
	for _, obj := range clusterClassObjs {
		if obj.GroupVersionKind().GroupKind() == ipamv1.GroupVersion.WithKind("IPPool").GroupKind() || obj.GroupVersionKind().GroupKind() == infrav1beta1.GroupVersion.WithKind("Metal3DataTemplate").GroupKind() {
			Logf("move %v cluster", obj.GroupVersionKind().GroupKind())
			clusterObjs = append(clusterObjs, obj)
		} else {
			newClusterClassObjs = append(newClusterClassObjs, obj)
		}
	}
	clusterYAML, err := yaml.FromUnstructured(clusterObjs)
	Expect(err).ToNot(HaveOccurred())
	clusterClassYAML, err := yaml.FromUnstructured(newClusterClassObjs)
	Expect(err).ToNot(HaveOccurred())
	return clusterClassYAML, clusterYAML
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

// RemoveEmptyWorkers removes the "workers" section from the input YAML if it is empty.
func RemoveEmptyWorkers(input []byte) ([]byte, error) {
	var ret []byte
	objs, err := yaml.ToUnstructured(input)
	if err == nil {
		// Iterate through the objects and remove empty "workers" sections
		for _, obj := range objs {
			if obj.GetKind() == "Cluster" {
				unstructured.RemoveNestedField(obj.Object, "spec", "topology", "workers")
			}
		}
		ret, err = yaml.FromUnstructured(objs)
		return ret, err
	}
	return nil, err
}
