package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/blang/semver"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	testexec "sigs.k8s.io/cluster-api/test/framework/exec"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type vmState string

const (
	running        vmState = "running"
	paused         vmState = "paused"
	shutoff        vmState = "shutoff"
	other          vmState = "other"
	artifactoryURL         = "https://artifactory.nordix.org/artifactory/metal3/images/k8s"
	imagesURL              = "http://172.22.0.1/images"
	ironicImageDir         = "/opt/metal3-dev-env/ironic/html/images"
	osTypeCentos           = "centos"
	osTypeUbuntu           = "ubuntu"
	ironicSuffix           = "-ironic"
	// Out-of-service Taint test actions.
	oostAdded   = "added"
	oostRemoved = "removed"
)

var releaseMarkerPrefix = "go://github.com/metal3-io/cluster-api-provider-metal3@v%s"

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}

func Logf(format string, a ...interface{}) {
	fmt.Fprintf(GinkgoWriter, "INFO: "+format+"\n", a...)
}

func LogFromFile(logFile string) {
	data, err := os.ReadFile(filepath.Clean(logFile))
	Expect(err).ToNot(HaveOccurred(), "No log file found")
	Logf(string(data))
}

// return only the boolean value from ParseBool.
func getBool(s string) bool {
	b, err := strconv.ParseBool(s)
	Expect(err).ToNot(HaveOccurred())
	return b
}

// logTable print a formatted table into the e2e logs.
func logTable(title string, rows [][]string) {
	getRowFormatted := func(row []string) string {
		rowFormatted := ""
		for i := range row {
			rowFormatted = fmt.Sprintf("%s\t%s\t", rowFormatted, row[i])
		}
		return rowFormatted
	}
	w := tabwriter.NewWriter(GinkgoWriter, 0, 0, 2, ' ', tabwriter.TabIndent)
	fmt.Fprintln(w, title)
	for _, r := range rows {
		fmt.Fprintln(w, getRowFormatted(r))
	}
	fmt.Fprintln(w, "")
	w.Flush()
}

// getSha256Hash return sha256 hash of given file.
func getSha256Hash(filename string) ([]byte, error) {
	file, err := os.Open(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}
	defer func() {
		err := file.Close()
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Error closing file: %s", filename))
	}()
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

// TODO change this function to handle multiple workload(target) clusters.
func DumpSpecResourcesAndCleanup(ctx context.Context, specName string, bootstrapClusterProxy framework.ClusterProxy, targetClusterProxy framework.ClusterProxy, artifactFolder string, namespace string, intervalsGetter func(spec, key string) []interface{}, clusterName, clusterctlLogFolder string, skipCleanup bool) {
	Expect(os.RemoveAll(clusterctlLogFolder)).Should(Succeed())
	clusterClient := bootstrapClusterProxy.GetClient()

	bootstrapClusterProxy.CollectWorkloadClusterLogs(ctx, namespace, clusterName, artifactFolder)

	By("Fetch logs from target cluster")
	err := FetchClusterLogs(targetClusterProxy, clusterLogCollectionBasePath)
	if err != nil {
		Logf("Error: %v", err)
	}
	// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
	By(fmt.Sprintf("Dumping all the Cluster API resources in the %q namespace", namespace))
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    clusterClient,
		Namespace: namespace,
		LogPath:   filepath.Join(artifactFolder, bootstrapClusterProxy.GetName(), "resources"),
	})

	if !skipCleanup {
		By(fmt.Sprintf("Deleting cluster %s/%s", namespace, clusterName))
		// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
		// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
		// instead of DeleteClusterAndWait
		framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
			Client:    clusterClient,
			Namespace: namespace,
		}, intervalsGetter(specName, "wait-delete-cluster")...)

		// Waiting for Metal3Datas, Metal3DataTemplates and Metal3DataClaims, as these may take longer time to delete
		By("Checking leftover Metal3Datas, Metal3DataTemplates and Metal3DataClaims")
		Eventually(func(g Gomega) {
			opts := &client.ListOptions{}
			datas := infrav1.Metal3DataList{}
			dataTemplates := infrav1.Metal3DataTemplateList{}
			dataClaims := infrav1.Metal3DataClaimList{}
			g.Expect(clusterClient.List(ctx, &datas, opts)).To(Succeed())
			g.Expect(clusterClient.List(ctx, &dataTemplates, opts)).To(Succeed())
			g.Expect(clusterClient.List(ctx, &dataClaims, opts)).To(Succeed())
			for _, dataObject := range datas.Items {
				By(fmt.Sprintf("Data named: %s is not delete", dataObject.Name))
			}
			for _, dataObject := range dataTemplates.Items {
				By(fmt.Sprintf("Datatemplate named: %s is not deleted", dataObject.Name))
			}
			for _, dataObject := range dataClaims.Items {
				By(fmt.Sprintf("Dataclaim named: %s is not deleted", dataObject.Name))
			}
			g.Expect(datas.Items).To(BeEmpty())
			g.Expect(dataTemplates.Items).To(BeEmpty())
			g.Expect(dataClaims.Items).To(BeEmpty())
			Logf("Waiting for Metal3Datas, Metal3DataTemplates and Metal3DataClaims to be deleted")
		}, intervalsGetter(specName, "wait-delete-cluster")...).Should(Succeed())
		Logf("Metal3Datas, Metal3DataTemplates and Metal3DataClaims are deleted")
	}
}

func EnsureImage(k8sVersion string) (imageURL string, imageChecksum string) {
	osType := strings.ToLower(os.Getenv("OS"))
	Expect(osType).To(BeElementOf([]string{osTypeUbuntu, osTypeCentos}))
	imageNamePrefix := "CENTOS_9_NODE_IMAGE_K8S"
	if osType != osTypeCentos {
		imageNamePrefix = "UBUNTU_22.04_NODE_IMAGE_K8S"
	}
	imageName := fmt.Sprintf("%s_%s.qcow2", imageNamePrefix, k8sVersion)
	rawImageName := fmt.Sprintf("%s_%s-raw.img", imageNamePrefix, k8sVersion)
	imageLocation := fmt.Sprintf("%s_%s/", artifactoryURL, k8sVersion)
	imageURL = fmt.Sprintf("%s/%s", imagesURL, rawImageName)
	imageChecksum = fmt.Sprintf("%s/%s.sha256sum", imagesURL, rawImageName)

	// Check if node image with upgraded k8s version exist, if not download it
	imagePath := filepath.Join(ironicImageDir, imageName)
	rawImagePath := filepath.Join(ironicImageDir, rawImageName)
	if _, err := os.Stat(rawImagePath); err == nil {
		Logf("Local image %v already exists", rawImagePath)
	} else if os.IsNotExist(err) {
		Logf("Local image %v is not found \nDownloading..", rawImagePath)
		err = DownloadFile(imagePath, fmt.Sprintf("%s/%s", imageLocation, imageName))
		Expect(err).ToNot(HaveOccurred())
		cmd := exec.Command("qemu-img", "convert", "-O", "raw", imagePath, rawImagePath) // #nosec G204:gosec
		err = cmd.Run()
		Expect(err).ToNot(HaveOccurred())
		sha256sum, err := getSha256Hash(rawImagePath)
		Expect(err).ToNot(HaveOccurred())
		formattedSha256sum := fmt.Sprintf("%x", sha256sum)
		err = os.WriteFile(fmt.Sprintf("%s/%s.sha256sum", ironicImageDir, rawImageName), []byte(formattedSha256sum), 0544) //#nosec G306:gosec
		Expect(err).ToNot(HaveOccurred())
		Logf("Image: %v downloaded", rawImagePath)
	} else {
		fmt.Fprintf(GinkgoWriter, "ERROR: %v\n", err)
		os.Exit(1)
	}
	return imageURL, imageChecksum
}

// DownloadFile will download a url and store it in local filepath.
func DownloadFile(filePath string, url string) error {
	// TODO: Lets change the wget to use go's native http client when network
	// more resilient
	cmd := exec.Command("wget", "-O", filePath, url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wget failed: %v, output: %s", err, string(output))
	}
	return nil
}

// FilterBmhsByProvisioningState returns a filtered list of BaremetalHost objects in certain provisioning state.
func FilterBmhsByProvisioningState(bmhs []bmov1alpha1.BareMetalHost, state bmov1alpha1.ProvisioningState) (result []bmov1alpha1.BareMetalHost) {
	for _, bmh := range bmhs {
		if bmh.Status.Provisioning.State == state {
			result = append(result, bmh)
		}
	}
	return
}

// FilterMachinesByPhase returns a filtered list of CAPI machine objects in certain desired phase.
func FilterMachinesByPhase(machines []clusterv1.Machine, phase clusterv1.MachinePhase) (result []clusterv1.Machine) {
	accept := func(machine clusterv1.Machine) bool {
		return machine.Status.GetTypedPhase() == phase
	}
	return FilterMachines(machines, accept)
}

// FilterMachines returns a filtered list of Machines that were accepted by the accept function.
func FilterMachines(machines []clusterv1.Machine, accept func(clusterv1.Machine) bool) (result []clusterv1.Machine) {
	for _, machine := range machines {
		if accept(machine) {
			result = append(result, machine)
		}
	}
	return
}

// AnnotateBmh annotates BaremetalHost with a given key and value.
func AnnotateBmh(ctx context.Context, client client.Client, host bmov1alpha1.BareMetalHost, key string, value *string) {
	helper, err := patch.NewHelper(&host, client)
	Expect(err).NotTo(HaveOccurred())
	annotations := host.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if value == nil {
		delete(annotations, key)
	} else {
		annotations[key] = *value
	}
	host.SetAnnotations(annotations)
	Expect(helper.Patch(ctx, &host)).To(Succeed())
}

// DeleteNodeReuseLabelFromHost deletes nodeReuseLabelName from the host if it exists.
func DeleteNodeReuseLabelFromHost(ctx context.Context, client client.Client, host bmov1alpha1.BareMetalHost, nodeReuseLabelName string) {
	helper, err := patch.NewHelper(&host, client)
	Expect(err).NotTo(HaveOccurred())
	labels := host.GetLabels()
	if labels != nil {
		if _, ok := labels[nodeReuseLabelName]; ok {
			delete(host.Labels, nodeReuseLabelName)
		}
	}
	Expect(helper.Patch(ctx, &host)).To(Succeed())
}

// ScaleMachineDeployment scales up/down MachineDeployment object to desired replicas.
func ScaleMachineDeployment(ctx context.Context, clusterClient client.Client, clusterName, namespace string, newReplicas int) {
	machineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      clusterClient,
		ClusterName: clusterName,
		Namespace:   namespace,
	})
	Expect(machineDeployments).To(HaveLen(1), "Expected exactly 1 MachineDeployment")
	machineDeploy := machineDeployments[0]
	patch := []byte(fmt.Sprintf(`{"spec": {"replicas": %d}}`, newReplicas))
	err := clusterClient.Patch(ctx, machineDeploy, client.RawPatch(types.MergePatchType, patch))
	Expect(err).ToNot(HaveOccurred(), "Failed to patch workers MachineDeployment")
}

// ScaleKubeadmControlPlane scales up/down KubeadmControlPlane object to desired replicas.
func ScaleKubeadmControlPlane(ctx context.Context, c client.Client, name client.ObjectKey, newReplicaCount int32) {
	ctrlplane := controlplanev1.KubeadmControlPlane{}
	Expect(c.Get(ctx, name, &ctrlplane)).To(Succeed())
	helper, err := patch.NewHelper(&ctrlplane, c)
	Expect(err).ToNot(HaveOccurred(), "Failed to create new patch helper")

	ctrlplane.Spec.Replicas = ptr.To(newReplicaCount)
	Expect(helper.Patch(ctx, &ctrlplane)).To(Succeed())
}

func DeploymentRolledOut(ctx context.Context, clientSet *kubernetes.Clientset, name string, namespace string, desiredGeneration int64) bool {
	deploy, err := clientSet.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	if deploy != nil {
		// When the number of replicas is equal to the number of available and updated
		// replicas, we know that only "new" pods are running. When we also
		// have the desired number of replicas and a new enough generation, we
		// know that the rollout is complete.
		return (deploy.Status.UpdatedReplicas == *deploy.Spec.Replicas) &&
			(deploy.Status.AvailableReplicas == *deploy.Spec.Replicas) &&
			(deploy.Status.Replicas == *deploy.Spec.Replicas) &&
			(deploy.Status.ObservedGeneration >= desiredGeneration)
	}
	return false
}

func GetAllBmhs(ctx context.Context, c client.Client, namespace string) ([]bmov1alpha1.BareMetalHost, error) {
	bmhs := bmov1alpha1.BareMetalHostList{}
	err := c.List(ctx, &bmhs, client.InNamespace(namespace))
	return bmhs.Items, err
}

// FilterNodeCondition will filter the slice of NodeConditions so that only the given conditionType remains
// and return the resulting slice.
func FilterNodeCondition(conditions []corev1.NodeCondition, conditionType corev1.NodeConditionType) []corev1.NodeCondition {
	filtered := []corev1.NodeCondition{}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			filtered = append(filtered, conditions[i])
		}
	}
	return filtered
}

// ListBareMetalHosts logs the names, provisioning status, consumer and power status
// of all BareMetalHosts matching the opts. Similar to kubectl get baremetalhosts.
func ListBareMetalHosts(ctx context.Context, c client.Client, opts ...client.ListOption) {
	bmhs := bmov1alpha1.BareMetalHostList{}
	Expect(c.List(ctx, &bmhs, opts...)).To(Succeed())

	rows := make([][]string, len(bmhs.Items)+1)
	// Add column names
	rows[0] = []string{"Name:", "Status:", "Consumer:", "Online:"}

	for i, bmh := range bmhs.Items {
		consumer := ""
		if bmh.Spec.ConsumerRef != nil {
			consumer = bmh.Spec.ConsumerRef.Name
		}
		rows[i+1] = []string{bmh.GetName(), fmt.Sprint(bmh.Status.Provisioning.State), consumer, fmt.Sprint(bmh.Status.PoweredOn)}
	}
	logTable("Listing BareMetalHosts", rows)
}

// ListMetal3Machines logs the names, ready status and provider ID of all Metal3Machines in the namespace.
// Similar to kubectl get metal3machines.
func ListMetal3Machines(ctx context.Context, c client.Client, opts ...client.ListOption) {
	metal3Machines := infrav1.Metal3MachineList{}
	Expect(c.List(ctx, &metal3Machines, opts...)).To(Succeed())

	rows := make([][]string, len(metal3Machines.Items)+1)
	// Add column names
	rows[0] = []string{"Name:", "Ready:", "Provider ID:"}
	for i, metal3Machine := range metal3Machines.Items {
		providerID := ""
		if metal3Machine.Spec.ProviderID != nil {
			providerID = *metal3Machine.Spec.ProviderID
		}
		rows[i+1] = []string{metal3Machine.GetName(), fmt.Sprint(metal3Machine.Status.Ready), providerID}
	}
	logTable("Listing Metal3Machines", rows)
}

// ListMachines logs the names, status phase, provider ID and Kubernetes version
// of all Machines in the namespace. Similar to kubectl get machines.
func ListMachines(ctx context.Context, c client.Client, opts ...client.ListOption) {
	machines := clusterv1.MachineList{}
	Expect(c.List(ctx, &machines, opts...)).To(Succeed())

	rows := make([][]string, len(machines.Items)+1)
	// Add column names
	rows[0] = []string{"Name:", "Status:", "Provider ID:", "Version:"}
	for i, machine := range machines.Items {
		providerID := ""
		if machine.Spec.ProviderID != nil {
			providerID = *machine.Spec.ProviderID
		}
		rows[i+1] = []string{machine.GetName(), fmt.Sprint(machine.Status.GetTypedPhase()), providerID, *machine.Spec.Version}
	}
	logTable("Listing Machines", rows)
}

// ListNodes logs the names, status and Kubernetes version of all Nodes.
// Similar to kubectl get nodes.
func ListNodes(ctx context.Context, c client.Client) {
	nodes := corev1.NodeList{}
	Expect(c.List(ctx, &nodes)).To(Succeed())

	rows := make([][]string, len(nodes.Items)+1)
	// Add column names
	rows[0] = []string{"Name:", "Status:", "Version:"}
	for i, node := range nodes.Items {
		ready := "NotReady"
		if node.Status.Conditions != nil {
			readyCondition := FilterNodeCondition(node.Status.Conditions, corev1.NodeReady)
			Expect(readyCondition).To(HaveLen(1))
			if readyCondition[0].Status == corev1.ConditionTrue {
				ready = "Ready"
			}
		}
		rows[i+1] = []string{node.Name, ready, node.Status.NodeInfo.KubeletVersion}
	}
	logTable("Listing Nodes", rows)
}

func CreateNewM3MachineTemplate(ctx context.Context, namespace string, newM3MachineTemplateName string, m3MachineTemplateName string, clusterClient client.Client, imageURL string, imageChecksum string) {
	checksumType := "sha256"
	imageFormat := "raw"

	m3MachineTemplate := infrav1.Metal3MachineTemplate{}
	Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3MachineTemplateName}, &m3MachineTemplate)).To(Succeed())

	newM3MachineTemplate := m3MachineTemplate.DeepCopy()
	cleanObjectMeta(&newM3MachineTemplate.ObjectMeta)

	newM3MachineTemplate.Spec.Template.Spec.Image.URL = imageURL
	newM3MachineTemplate.Spec.Template.Spec.Image.Checksum = imageChecksum
	newM3MachineTemplate.Spec.Template.Spec.Image.DiskFormat = &imageFormat
	newM3MachineTemplate.Spec.Template.Spec.Image.ChecksumType = &checksumType
	newM3MachineTemplate.ObjectMeta.Name = newM3MachineTemplateName

	Expect(clusterClient.Create(ctx, newM3MachineTemplate)).To(Succeed(), "Failed to create new Metal3MachineTemplate")
}

type WaitForNumInput struct {
	Client    client.Client
	Options   []client.ListOption
	Replicas  int
	Intervals []interface{}
}

// WaitForNumBmhInState will wait for the given number of BMHs to be in the given state.
func WaitForNumBmhInState(ctx context.Context, state bmov1alpha1.ProvisioningState, input WaitForNumInput) {
	Logf("Waiting for %d BMHs to be in %s state", input.Replicas, state)
	Eventually(func(g Gomega) {
		bmhList := bmov1alpha1.BareMetalHostList{}
		g.Expect(input.Client.List(ctx, &bmhList, input.Options...)).To(Succeed())
		g.Expect(FilterBmhsByProvisioningState(bmhList.Items, state)).To(HaveLen(input.Replicas))
	}, input.Intervals...).Should(Succeed())
	ListBareMetalHosts(ctx, input.Client, input.Options...)
}

// WaitForNumMetal3MachinesReady will wait for the given number of M3Ms to be ready.
func WaitForNumMetal3MachinesReady(ctx context.Context, input WaitForNumInput) {
	Logf("Waiting for %d Metal3Machines to be ready", input.Replicas)
	Eventually(func(g Gomega) {
		m3mList := infrav1.Metal3MachineList{}
		g.Expect(input.Client.List(ctx, &m3mList, input.Options...)).To(Succeed())
		numReady := 0
		for _, m3m := range m3mList.Items {
			if m3m.Status.Ready {
				numReady++
			}
		}
		g.Expect(numReady).To(BeEquivalentTo(input.Replicas))
	}, input.Intervals...).Should(Succeed())
	ListMetal3Machines(ctx, input.Client, input.Options...)
}

// WaitForNumMachinesInState will wait for the given number of Machines to be in the given state.
func WaitForNumMachinesInState(ctx context.Context, phase clusterv1.MachinePhase, input WaitForNumInput) {
	Logf("Waiting for %d Machines to be in %s phase", input.Replicas, phase)
	inPhase := func(machine clusterv1.Machine) bool {
		return machine.Status.GetTypedPhase() == phase
	}
	WaitForNumMachines(ctx, inPhase, input)
}

// WaitForNumMachines will wait for the given number of Machines to be accepted by the accept function.
// This is a more generic function than WaitForNumMachinesInState. It can be used to wait for any condition,
// e.g. that the Kubernetes version is correct.
func WaitForNumMachines(ctx context.Context, accept func(clusterv1.Machine) bool, input WaitForNumInput) {
	Eventually(func(g Gomega) {
		machineList := clusterv1.MachineList{}
		g.Expect(input.Client.List(ctx, &machineList, input.Options...)).To(Succeed())
		g.Expect(FilterMachines(machineList.Items, accept)).To(HaveLen(input.Replicas))
	}, input.Intervals...).Should(Succeed())
	ListMachines(ctx, input.Client, input.Options...)
}

// Get the machine object given its object name.
func GetMachine(ctx context.Context, c client.Client, name client.ObjectKey) (result clusterv1.Machine) {
	Expect(c.Get(ctx, name, &result)).To(Succeed())
	return
}

func GetMetal3Machines(ctx context.Context, c client.Client, _, namespace string) ([]infrav1.Metal3Machine, []infrav1.Metal3Machine) {
	var controlplane, workers []infrav1.Metal3Machine
	allMachines := &infrav1.Metal3MachineList{}
	Expect(c.List(ctx, allMachines, client.InNamespace(namespace))).To(Succeed())

	for _, machine := range allMachines.Items {
		if _, ok := machine.GetLabels()[clusterv1.MachineControlPlaneLabel]; ok {
			controlplane = append(controlplane, machine)
		} else {
			workers = append(workers, machine)
		}
	}

	return controlplane, workers
}

// GetIPPools return baremetal and provisioning IPPools.
func GetIPPools(ctx context.Context, c client.Client, _, namespace string) ([]ipamv1.IPPool, []ipamv1.IPPool) {
	var bmv4IPPool, provisioningIPPool []ipamv1.IPPool
	allIPPools := &ipamv1.IPPoolList{}
	Expect(c.List(ctx, allIPPools, client.InNamespace(namespace))).To(Succeed())

	for _, ippool := range allIPPools.Items {
		if strings.Contains(ippool.ObjectMeta.Name, "baremetalv4") {
			bmv4IPPool = append(bmv4IPPool, ippool)
		} else {
			provisioningIPPool = append(provisioningIPPool, ippool)
		}
	}

	return bmv4IPPool, provisioningIPPool
}

// GenerateIPPoolPreallocations fetches the current allocated IPs from an IPPool and returns a new map concatenating BMH and IPPool names as a
// key and an IPAddress as a value.
func GenerateIPPoolPreallocations(ctx context.Context, ippool ipamv1.IPPool, poolName string, c client.Client) (map[string]ipamv1.IPAddressStr, error) {
	allocations := ippool.Status.Allocations
	m3DataList, m3MachineList := infrav1.Metal3DataList{}, infrav1.Metal3MachineList{}
	Expect(c.List(ctx, &m3DataList, &client.ListOptions{})).To(Succeed())
	Expect(c.List(ctx, &m3MachineList, &client.ListOptions{})).To(Succeed())
	newAllocations := make(map[string]ipamv1.IPAddressStr)
	for m3dataPoolName, ipaddress := range allocations {
		fmt.Println("datapoolName:", m3dataPoolName, "=>", "ipaddress:", ipaddress)
		BMHName := strings.Split(m3dataPoolName, "-"+poolName)[0]
		Logf("poolName: %s", poolName)
		Logf("BMHName: %s", BMHName)
		newAllocations[BMHName+"-"+ippool.Name] = ipaddress
	}
	return newAllocations, nil
}

// Metal3DataToMachineName finds the relevant owner reference in Metal3Data
// and returns the name of corresponding Metal3Machine.
func Metal3DataToMachineName(m3data infrav1.Metal3Data) (string, error) {
	ownerReferences := m3data.GetOwnerReferences()
	for _, reference := range ownerReferences {
		if reference.Kind == "Metal3Machine" {
			return reference.Name, nil
		}
	}
	return "", fmt.Errorf("metal3Data missing a \"Metal3Machine\" kind owner reference")
}

// FilterMetal3DatasByName returns a filtered list of m3data objects with specific name.
func FilterMetal3DatasByName(m3datas []infrav1.Metal3Data, name string) (result []infrav1.Metal3Data) {
	Logf("m3datas: %v", m3datas)
	Logf("looking for name: %s", name)
	for _, m3data := range m3datas {
		Logf("m3data: %v", m3data)
		if m3data.ObjectMeta.Name == name {
			result = append(result, m3data)
		}
	}
	return result
}

// FilterMetal3MachinesByName returns a filtered list of m3machine objects with specific name.
func FilterMetal3MachinesByName(m3ms []infrav1.Metal3Machine, name string) (result []infrav1.Metal3Machine) {
	for _, m3m := range m3ms {
		if m3m.ObjectMeta.Name == name {
			result = append(result, m3m)
		}
	}
	return result
}

// Metal3MachineToMachineName finds the relevant owner reference in Metal3Machine
// and returns the name of corresponding Machine.
func Metal3MachineToMachineName(m3machine infrav1.Metal3Machine) (string, error) {
	ownerReferences := m3machine.GetOwnerReferences()
	for _, reference := range ownerReferences {
		if reference.Kind == "Machine" {
			return reference.Name, nil
		}
	}
	return "", fmt.Errorf("metal3machine missing a \"Machine\" kind owner reference")
}

func Metal3MachineToBmhName(m3machine infrav1.Metal3Machine) string {
	return strings.Replace(m3machine.GetAnnotations()["metal3.io/BareMetalHost"], "metal3/", "", 1)
}

// Derives the name of a VM created by metal3-dev-env from the name of a BareMetalHost object.
func BmhToVMName(host bmov1alpha1.BareMetalHost) string {
	return strings.ReplaceAll(host.Name, "-", "_")
}

func BmhNameToVMName(hostname string) string {
	return strings.ReplaceAll(hostname, "-", "_")
}

func MachineToVMName(ctx context.Context, cli client.Client, m *clusterv1.Machine) (string, error) {
	allMetal3Machines := &infrav1.Metal3MachineList{}
	Expect(cli.List(ctx, allMetal3Machines, client.InNamespace(m.Namespace))).To(Succeed())
	for _, machine := range allMetal3Machines.Items {
		name, err := Metal3MachineToMachineName(machine)
		if err != nil {
			Logf("error getting Machine name from Metal3machine: %w", err)
		} else if name == m.Name {
			return BmhNameToVMName(Metal3MachineToBmhName(machine)), nil
		}
	}
	return "", fmt.Errorf("no matching Metal3Machine found for current Machine")
}

// MachineTiIPAddress gets IPAddress based on machine, from machine -> m3machine -> m3data -> IPAddress.
func MachineToIPAddress(ctx context.Context, cli client.Client, m *clusterv1.Machine, ippool ipamv1.IPPool) (string, error) {
	m3Machine := &infrav1.Metal3Machine{}
	err := cli.Get(ctx, types.NamespacedName{
		Namespace: m.Spec.InfrastructureRef.Namespace,
		Name:      m.Spec.InfrastructureRef.Name},
		m3Machine)

	if err != nil {
		return "", fmt.Errorf("couldn't get a Metal3Machine within namespace %s with name %s : %w", m.Spec.InfrastructureRef.Namespace, m.Spec.InfrastructureRef.Name, err)
	}
	m3DataList := &infrav1.Metal3DataList{}
	m3Data := &infrav1.Metal3Data{}
	err = cli.List(ctx, m3DataList)
	if err != nil {
		return "", fmt.Errorf("coudln't list Metal3Data objects: %w", err)
	}
	for i, m3d := range m3DataList.Items {
		for _, owner := range m3d.OwnerReferences {
			if owner.Name == m3Machine.Name {
				m3Data = &m3DataList.Items[i]
			}
		}
	}
	if m3Data.Name == "" {
		return "", fmt.Errorf("couldn't find a matching Metal3Data object")
	}

	IPAddresses := &ipamv1.IPAddressList{}
	IPAddress := &ipamv1.IPAddress{}
	err = cli.List(ctx, IPAddresses)
	if err != nil {
		return "", fmt.Errorf("couldn't list IPAddress objects: %w", err)
	}
	for i, ip := range IPAddresses.Items {
		for _, owner := range ip.OwnerReferences {
			if owner.Name == m3Data.Name && ip.Spec.Pool.Name == ippool.Name {
				IPAddress = &IPAddresses.Items[i]
			}
		}
	}
	if IPAddress.Name == "" {
		return "", fmt.Errorf("couldn't find a matching IPAddress object")
	}

	return string(IPAddress.Spec.Address), nil
}

// RunCommand runs a command via ssh. If logfolder is "", no logs are saved.
func runCommand(logFolder, filename, machineIP, user, command string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not get home directory: %w", err)
	}
	keyPath := path.Join(filepath.Clean(home), ".ssh", "id_rsa")
	privkey, err := os.ReadFile(keyPath) //#nosec G304:gosec
	if err != nil {
		return fmt.Errorf("couldn't read private key")
	}
	signer, err := ssh.ParsePrivateKey(privkey)
	if err != nil {
		return fmt.Errorf("couldn't form a signer from ssh key: %w", err)
	}
	cfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(_ string, _ net.Addr, _ ssh.PublicKey) error { return nil },
		Timeout:         60 * time.Second,
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", machineIP), cfg)
	if err != nil {
		return fmt.Errorf("couldn't dial the machinehost at %s : %w", machineIP, err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("couldn't open a new session: %w", err)
	}
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf
	if err := session.Run("sudo " + command + "\n"); err != nil {
		return fmt.Errorf("unable to send command %q: %w", "sudo "+command, err)
	}
	result := strings.TrimSuffix(stdoutBuf.String(), "\n") + "\n" + strings.TrimSuffix(stderrBuf.String(), "\n")
	if logFolder != "" {
		// Write logs is folder path is provided.
		logFile := path.Join(logFolder, filename)
		if err := os.WriteFile(logFile, []byte(result), 0400); err != nil {
			return fmt.Errorf("error writing log file: %w", err)
		}
	}
	return nil
}

// LabelCRD is adding the specified labels to the CRD crdName. Existing labels with matching keys will be overwritten.
func LabelCRD(ctx context.Context, c client.Client, crdName string, labels map[string]string) error {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := c.Get(ctx, client.ObjectKey{Name: crdName}, crd)
	if err != nil {
		return err
	}
	// Apply labels to the CRD
	if crd.Labels == nil {
		crd.Labels = make(map[string]string)
	}
	for key, value := range labels {
		crd.Labels[key] = value
	}
	// Update the CRD
	err = c.Update(ctx, crd)
	if err != nil {
		return err
	}
	Logf("CRD '%s' labeled successfully\n", crdName)
	return nil
}

// GetCAPM3StableReleaseOfMinor returns latest stable version of minorRelease.
func GetCAPM3StableReleaseOfMinor(ctx context.Context, minorRelease string) (string, error) {
	releaseMarker := fmt.Sprintf(releaseMarkerPrefix, minorRelease)
	return clusterctl.ResolveRelease(ctx, releaseMarker)
}

// GetLatestPatchRelease returns latest patch release against minor release.
func GetLatestPatchRelease(goProxyPath string, minorReleaseVersion string) (string, error) {
	if strings.EqualFold("main", minorReleaseVersion) || strings.EqualFold("latest", minorReleaseVersion) {
		return strings.ToUpper(minorReleaseVersion), nil
	}
	semVersion, err := semver.Parse(minorReleaseVersion)
	if err != nil {
		return "", errors.Wrapf(err, "parsing semver for %s", minorReleaseVersion)
	}
	parsedTags, err := getVersions(goProxyPath)
	if err != nil {
		return "", err
	}

	var picked semver.Version
	for i, tag := range parsedTags {
		if tag.Major == semVersion.Major && tag.Minor == semVersion.Minor {
			picked = parsedTags[i]
		}
	}
	if picked.Major == 0 && picked.Minor == 0 && picked.Patch == 0 {
		return "", errors.Errorf("no suitable release available for path %s and version %s", goProxyPath, minorReleaseVersion)
	}
	return picked.String(), nil
}

// GetVersions returns the a sorted list of semantical versions which exist for a go module.
func getVersions(gomodulePath string) (semver.Versions, error) {
	// Get the data
	/* #nosec G107 */
	resp, err := http.Get(gomodulePath) //nolint:noctx
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to get versions from url %s got %d %s", gomodulePath, resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	defer resp.Body.Close()

	rawResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		retryError := errors.Wrap(err, "failed to get versions: error reading goproxy response body")
		return nil, retryError
	}
	parsedVersions := semver.Versions{}
	for _, s := range strings.Split(string(rawResponse), "\n") {
		if s == "" {
			continue
		}
		s = strings.TrimSuffix(s, "+incompatible")
		parsedVersion, err := semver.ParseTolerant(s)
		if err != nil {
			// Discard releases with tags that are not a valid semantic versions (the user can point explicitly to such releases).
			continue
		}
		parsedVersions = append(parsedVersions, parsedVersion)
	}

	if len(parsedVersions) == 0 {
		return nil, fmt.Errorf("no versions found for go module %q", gomodulePath)
	}
	sort.Sort(parsedVersions)
	return parsedVersions, nil
}

// BuildAndApplyKustomizationInput provides input for BuildAndApplyKustomize().
// If WaitForDeployment and/or WatchDeploymentLogs is set to true, then DeploymentName
// and DeploymentNamespace are expected.
type BuildAndApplyKustomizationInput struct {
	// Path to the kustomization to build
	Kustomization string

	ClusterProxy framework.ClusterProxy

	// If this is set to true. Perform a wait until the deployment specified by
	// DeploymentName and DeploymentNamespace is available or WaitIntervals is timed out
	WaitForDeployment bool

	// If this is set to true. Set up a log watcher for the deployment specified by
	// DeploymentName and DeploymentNamespace
	WatchDeploymentLogs bool

	// DeploymentName and DeploymentNamespace specified a deployment that will be waited and/or logged
	DeploymentName      string
	DeploymentNamespace string

	// Path to store the deployment logs
	LogPath string

	// Intervals to use in checking and waiting for the deployment
	WaitIntervals []interface{}
}

func (input *BuildAndApplyKustomizationInput) validate() error {
	// If neither WaitForDeployment nor WatchDeploymentLogs is true, we don't need to validate the input
	if !input.WaitForDeployment && !input.WatchDeploymentLogs {
		return nil
	}
	if input.WaitForDeployment && input.WaitIntervals == nil {
		return errors.Errorf("WaitIntervals is expected if WaitForDeployment is set to true")
	}
	if input.WatchDeploymentLogs && input.LogPath == "" {
		return errors.Errorf("LogPath is expected if WatchDeploymentLogs is set to true")
	}
	if input.DeploymentName == "" || input.DeploymentNamespace == "" {
		return errors.Errorf("DeploymentName and DeploymentNamespace are expected if WaitForDeployment or WatchDeploymentLogs is true")
	}
	return nil
}

// BuildAndApplyKustomization takes input from BuildAndApplyKustomizationInput. It builds the provided kustomization
// and apply it to the cluster provided by clusterProxy.
func BuildAndApplyKustomization(ctx context.Context, input *BuildAndApplyKustomizationInput) error {
	Expect(input.validate()).To(Succeed())
	var err error
	kustomization := input.Kustomization
	clusterProxy := input.ClusterProxy
	manifest, err := buildKustomizeManifest(kustomization)
	if err != nil {
		return err
	}

	err = clusterProxy.CreateOrUpdate(ctx, manifest)
	if err != nil {
		return err
	}

	if !input.WaitForDeployment && !input.WatchDeploymentLogs {
		return nil
	}

	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.DeploymentName,
			Namespace: input.DeploymentNamespace,
		},
	}

	if input.WaitForDeployment {
		// Wait for the deployment to become available
		framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
			Getter:     clusterProxy.GetClient(),
			Deployment: deployment,
		}, input.WaitIntervals...)
	}

	if input.WatchDeploymentLogs {
		// Set up log watcher
		framework.WatchDeploymentLogsByName(ctx, framework.WatchDeploymentLogsByNameInput{
			GetLister:  clusterProxy.GetClient(),
			Cache:      clusterProxy.GetCache(ctx),
			ClientSet:  clusterProxy.GetClientSet(),
			Deployment: deployment,
			LogPath:    input.LogPath,
		})
	}
	return nil
}

// BuildAndRemoveKustomization builds the provided kustomization to resources and removes them from the cluster
// provided by clusterProxy.
func BuildAndRemoveKustomization(ctx context.Context, kustomization string, clusterProxy framework.ClusterProxy) error {
	manifest, err := buildKustomizeManifest(kustomization)
	if err != nil {
		return err
	}
	return KubectlDelete(ctx, clusterProxy.GetKubeconfigPath(), manifest)
}

// KubectlDelete shells out to kubectl delete.
func KubectlDelete(ctx context.Context, kubeconfigPath string, resources []byte, args ...string) error {
	aargs := append([]string{"delete", "--kubeconfig", kubeconfigPath, "-f", "-"}, args...)
	rbytes := bytes.NewReader(resources)
	deleteCmd := testexec.NewCommand(
		testexec.WithCommand("kubectl"),
		testexec.WithArgs(aargs...),
		testexec.WithStdin(rbytes),
	)

	fmt.Printf("Running kubectl %s\n", strings.Join(aargs, " "))
	stdout, stderr, err := deleteCmd.Run(ctx)
	fmt.Printf("stderr:\n%s\n", string(stderr))
	fmt.Printf("stdout:\n%s\n", string(stdout))
	return err
}

func buildKustomizeManifest(source string) ([]byte, error) {
	kustomizer := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	fSys := filesys.MakeFsOnDisk()
	resources, err := kustomizer.Run(fSys, source)
	if err != nil {
		return nil, err
	}
	return resources.AsYaml()
}

// CreateOrUpdateWithNamespace creates or updates objects using the clusterProxy client with specific namespace.
func CreateOrUpdateWithNamespace(ctx context.Context, p framework.ClusterProxy, resources []byte, namespace string) error {
	Expect(ctx).NotTo(BeNil(), "ctx is required for CreateOrUpdate")
	Expect(resources).NotTo(BeNil(), "resources is required for CreateOrUpdate")
	objs, err := yaml.ToUnstructured(resources)
	if err != nil {
		return err
	}
	existingObject := &unstructured.Unstructured{}
	var retErrs []error
	for _, o := range objs {
		o.SetNamespace(namespace)
		objectKey := types.NamespacedName{
			Name:      o.GetName(),
			Namespace: o.GetNamespace(),
		}
		existingObject.SetAPIVersion(o.GetAPIVersion())
		existingObject.SetKind(o.GetKind())
		if err := p.GetClient().Get(ctx, objectKey, existingObject); err != nil {
			// Expected error -- if the object does not exist, create it
			if apierrors.IsNotFound(err) {
				if err := p.GetClient().Create(ctx, &o); err != nil {
					retErrs = append(retErrs, err)
				}
			} else {
				retErrs = append(retErrs, err)
			}
		} else {
			o.SetResourceVersion(existingObject.GetResourceVersion())
			if err := p.GetClient().Update(ctx, &o); err != nil {
				retErrs = append(retErrs, err)
			}
		}
	}
	return kerrors.NewAggregate(retErrs)
}

type CreateTargetClusterInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	SpecName              string
	ClusterName           string
	K8sVersion            string
	KCPMachineCount       int64
	WorkerMachineCount    int64
	ClusterctlLogFolder   string
	ClusterctlConfigPath  string
	OSType                string
	Namespace             string
}

func CreateTargetCluster(ctx context.Context, inputGetter func() CreateTargetClusterInput) (framework.ClusterProxy, *clusterctl.ApplyClusterTemplateAndWaitResult) {
	By("Creating a high available cluster")
	input := inputGetter()
	imageURL, imageChecksum := EnsureImage(input.K8sVersion)
	os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
	os.Setenv("IMAGE_RAW_URL", imageURL)
	controlPlaneMachineCount := input.KCPMachineCount
	workerMachineCount := input.WorkerMachineCount
	result := clusterctl.ApplyClusterTemplateAndWaitResult{}
	clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy: input.BootstrapClusterProxy,
		ConfigCluster: clusterctl.ConfigClusterInput{
			LogFolder:                input.ClusterctlLogFolder,
			ClusterctlConfigPath:     input.ClusterctlConfigPath,
			KubeconfigPath:           input.BootstrapClusterProxy.GetKubeconfigPath(),
			InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
			Flavor:                   input.OSType,
			Namespace:                input.Namespace,
			ClusterName:              input.ClusterName,
			KubernetesVersion:        input.K8sVersion,
			ControlPlaneMachineCount: &controlPlaneMachineCount,
			WorkerMachineCount:       &workerMachineCount,
		},
		WaitForClusterIntervals:      input.E2EConfig.GetIntervals(input.SpecName, "wait-cluster"),
		WaitForControlPlaneIntervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-control-plane"),
		WaitForMachineDeployments:    input.E2EConfig.GetIntervals(input.SpecName, "wait-worker-nodes"),
	}, &result)
	targetCluster := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace, result.Cluster.Name)
	framework.WaitForPodListCondition(ctx, framework.WaitForPodListConditionInput{
		Lister:      targetCluster.GetClient(),
		ListOptions: &client.ListOptions{LabelSelector: labels.Everything(), Namespace: "kube-system"},
		Condition:   framework.PhasePodCondition(corev1.PodRunning),
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-all-pod-to-be-running-on-target-cluster")...)
	return targetCluster, &result
}
