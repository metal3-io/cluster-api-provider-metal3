package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
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
	infrav1beta1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	irsov1alpha1 "github.com/metal3-io/ironic-standalone-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
	imagesURL              = "http://192.168.111.1:8080/images"
	ironicImageDir         = "/tmp/metal3/images"
	osTypeCentos           = "centos"
	osTypeUbuntu           = "ubuntu"
	osTypeLeap             = "opensuse-leap"
	ironicSuffix           = "-ironic"
	// Out-of-service Taint test actions.
	oostAdded   = "added"
	oostRemoved = "removed"

	retryableOperationInterval = 3 * time.Second
	retryableOperationTimeout  = 3 * time.Minute
)

func Byf(format string, a ...any) {
	By(fmt.Sprintf(format, a...))
}

func Logf(format string, a ...any) {
	fmt.Fprintf(GinkgoWriter, "INFO: "+format+"\n", a...)
}

func LogFromFile(logFile string) {
	data, err := os.ReadFile(filepath.Clean(logFile))
	Expect(err).ToNot(HaveOccurred(), "No log file found")
	Logf(string(data))
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
		err = file.Close()
		Expect(err).ToNot(HaveOccurred(), "Error closing file: "+filename)
	}()
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

var falseValues = []string{"", "false", "no"}

// GetBoolVariable returns a variable from environment variables or from the e2e config file as boolean.
func GetBoolVariable(e2eConfig *clusterctl.E2EConfig, varName string) bool {
	value := e2eConfig.MustGetVariable(varName)
	for _, falseVal := range falseValues {
		if strings.EqualFold(value, falseVal) {
			return false
		}
	}
	return true
}

// TODO change this function to handle multiple workload(target) clusters.
func DumpSpecResourcesAndCleanup(ctx context.Context, specName string, bootstrapClusterProxy framework.ClusterProxy, targetClusterProxy framework.ClusterProxy, artifactFolder string, namespace string, intervalsGetter func(spec, key string) []any, clusterName, clusterctlLogFolder string, skipCleanup bool, clusterctlConfigPath string) {
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
		Lister:               clusterClient,
		Namespace:            namespace,
		LogPath:              filepath.Join(artifactFolder, bootstrapClusterProxy.GetName(), "resources"),
		KubeConfigPath:       bootstrapClusterProxy.GetKubeconfigPath(),
		ClusterctlConfigPath: clusterctlConfigPath,
	})

	if !skipCleanup {
		By(fmt.Sprintf("Deleting cluster %s/%s", namespace, clusterName))
		// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
		// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
		// instead of DeleteClusterAndWait
		framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
			ClusterProxy:         bootstrapClusterProxy,
			Namespace:            namespace,
			ClusterctlConfigPath: clusterctlConfigPath,
			ArtifactFolder:       filepath.Join(artifactFolder, "delete-cluster"),
		}, intervalsGetter(specName, "wait-delete-cluster")...)

		// Waiting for Metal3Datas, Metal3DataTemplates and Metal3DataClaims, as these may take longer time to delete
		By("Checking leftover Metal3Datas, Metal3DataTemplates and Metal3DataClaims")
		Eventually(func(g Gomega) {
			opts := &client.ListOptions{}
			datas := infrav1beta1.Metal3DataList{}
			dataTemplates := infrav1beta1.Metal3DataTemplateList{}
			dataClaims := infrav1beta1.Metal3DataClaimList{}
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
	Expect(osType).To(BeElementOf([]string{osTypeUbuntu, osTypeCentos, osTypeLeap}))
	imageNamePrefix := ""
	switch osType {
	case osTypeCentos:
		imageNamePrefix = "CENTOS_10_NODE_IMAGE_K8S"
	case osTypeUbuntu:
		imageNamePrefix = "UBUNTU_24.04_NODE_IMAGE_K8S"
	case osTypeLeap:
		imageNamePrefix = "LEAP_15_6_NODE_IMAGE_K8S"
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
		cmd := exec.CommandContext(context.Background(), "qemu-img", "convert", "-O", "raw", imagePath, rawImagePath) // #nosec G204:gosec
		err = cmd.Run()
		Expect(err).ToNot(HaveOccurred())
		var sha256sum []byte
		sha256sum, err = getSha256Hash(rawImagePath)
		Expect(err).ToNot(HaveOccurred())
		formattedSha256sum := hex.EncodeToString(sha256sum)
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
	cmd := exec.CommandContext(context.Background(), "wget", "-O", filePath, url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wget failed: %w, output: %s", err, string(output))
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
func AnnotateBmh(ctx context.Context, clusterClient client.Client, host bmov1alpha1.BareMetalHost, key string, value *string) {
	bmh := &bmov1alpha1.BareMetalHost{}
	bmhKey := client.ObjectKey{Name: host.Name, Namespace: host.Namespace}
	err := clusterClient.Get(ctx, bmhKey, bmh)
	Expect(err).ToNot(HaveOccurred(), "Failed to get BareMetalHost %s", host.Name)
	helper, err := patch.NewHelper(bmh, clusterClient)
	Expect(err).NotTo(HaveOccurred())

	if value == nil {
		Logf("Removing annotation %s from BMH %s", key, bmh.Name)
		delete(bmh.Annotations, key)
	} else {
		Logf("Adding annotation %s to BMH %s", key, bmh.Name)
		if bmh.Annotations == nil {
			bmh.Annotations = make(map[string]string)
		}
		bmh.Annotations[key] = *value
	}
	Expect(helper.Patch(ctx, bmh)).To(Succeed())
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
	patch := fmt.Appendf(nil, `{"spec": {"replicas": %d}}`, newReplicas)
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
		rows[i+1] = []string{bmh.GetName(), fmt.Sprint(bmh.Status.Provisioning.State), consumer, strconv.FormatBool(bmh.Status.PoweredOn)}
	}
	logTable("Listing BareMetalHosts", rows)
}

// ListMetal3Machines logs the names, ready status and provider ID of all Metal3Machines in the namespace.
// Similar to kubectl get metal3machines.
func ListMetal3Machines(ctx context.Context, c client.Client, opts ...client.ListOption) {
	metal3Machines := infrav1beta1.Metal3MachineList{}
	Expect(c.List(ctx, &metal3Machines, opts...)).To(Succeed())

	rows := make([][]string, len(metal3Machines.Items)+1)
	// Add column names
	rows[0] = []string{"Name:", "Ready:", "Provider ID:"}
	for i, metal3Machine := range metal3Machines.Items {
		providerID := ""
		if metal3Machine.Spec.ProviderID != nil {
			providerID = *metal3Machine.Spec.ProviderID
		}
		rows[i+1] = []string{metal3Machine.GetName(), strconv.FormatBool(metal3Machine.Status.Ready), providerID}
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
		if machine.Spec.ProviderID != "" {
			providerID = machine.Spec.ProviderID
		}
		rows[i+1] = []string{machine.GetName(), fmt.Sprint(machine.Status.GetTypedPhase()), providerID, machine.Spec.Version}
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

	m3MachineTemplate := infrav1beta1.Metal3MachineTemplate{}
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
	Intervals []any
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
		m3mList := infrav1beta1.Metal3MachineList{}
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

func GetMetal3Machines(ctx context.Context, c client.Client, _, namespace string) ([]infrav1beta1.Metal3Machine, []infrav1beta1.Metal3Machine) {
	var controlplane, workers []infrav1beta1.Metal3Machine
	allMachines := &infrav1beta1.Metal3MachineList{}
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
	m3DataList, m3MachineList := infrav1beta1.Metal3DataList{}, infrav1beta1.Metal3MachineList{}
	Expect(c.List(ctx, &m3DataList, &client.ListOptions{})).To(Succeed())
	Expect(c.List(ctx, &m3MachineList, &client.ListOptions{})).To(Succeed())
	newAllocations := make(map[string]ipamv1.IPAddressStr)
	for m3dataPoolName, ipaddress := range allocations {
		Logf("datapoolName:", m3dataPoolName, "=>", "ipaddress:", ipaddress)
		BMHName := strings.Split(m3dataPoolName, "-"+poolName)[0]
		Logf("poolName: %s", poolName)
		Logf("BMHName: %s", BMHName)
		newAllocations[BMHName+"-"+ippool.Name] = ipaddress
	}
	return newAllocations, nil
}

// Metal3DataToMachineName finds the relevant owner reference in Metal3Data
// and returns the name of corresponding Metal3Machine.
func Metal3DataToMachineName(m3data infrav1beta1.Metal3Data) (string, error) {
	ownerReferences := m3data.GetOwnerReferences()
	for _, reference := range ownerReferences {
		if reference.Kind == "Metal3Machine" {
			return reference.Name, nil
		}
	}
	return "", errors.New("metal3Data missing a \"Metal3Machine\" kind owner reference")
}

// FilterMetal3DatasByName returns a filtered list of m3data objects with specific name.
func FilterMetal3DatasByName(m3datas []infrav1beta1.Metal3Data, name string) (result []infrav1beta1.Metal3Data) {
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
func FilterMetal3MachinesByName(m3ms []infrav1beta1.Metal3Machine, name string) (result []infrav1beta1.Metal3Machine) {
	for _, m3m := range m3ms {
		if m3m.ObjectMeta.Name == name {
			result = append(result, m3m)
		}
	}
	return result
}

// Metal3MachineToMachineName finds the relevant owner reference in Metal3Machine
// and returns the name of corresponding Machine.
func Metal3MachineToMachineName(m3machine infrav1beta1.Metal3Machine) (string, error) {
	ownerReferences := m3machine.GetOwnerReferences()
	for _, reference := range ownerReferences {
		if reference.Kind == "Machine" {
			return reference.Name, nil
		}
	}
	return "", errors.New("metal3machine missing a \"Machine\" kind owner reference")
}

func Metal3MachineToBmhName(m3machine infrav1beta1.Metal3Machine) string {
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
	allMetal3Machines := &infrav1beta1.Metal3MachineList{}
	Expect(cli.List(ctx, allMetal3Machines, client.InNamespace(m.Namespace))).To(Succeed())
	for _, machine := range allMetal3Machines.Items {
		name, err := Metal3MachineToMachineName(machine)
		if err != nil {
			Logf("error getting Machine name from Metal3machine: %w", err)
		} else if name == m.Name {
			return BmhNameToVMName(Metal3MachineToBmhName(machine)), nil
		}
	}
	return "", errors.New("no matching Metal3Machine found for current Machine")
}

func MachineToVMNamev1beta1(ctx context.Context, cli client.Client, m *clusterv1.Machine) (string, error) {
	allMetal3Machines := &infrav1beta1.Metal3MachineList{}
	Expect(cli.List(ctx, allMetal3Machines, client.InNamespace(m.Namespace))).To(Succeed())
	for _, machine := range allMetal3Machines.Items {
		name, err := Metal3MachineToMachineName(machine)
		if err != nil {
			Logf("error getting Machine name from Metal3machine: %w", err)
		} else if name == m.Name {
			return BmhNameToVMName(Metal3MachineToBmhName(machine)), nil
		}
	}
	return "", errors.New("no matching Metal3Machine found for current Machine")
}

// MachineToIPAddress gets IPAddress based on machine, from machine -> m3machine -> m3data -> IPAddress.
func MachineToIPAddress(ctx context.Context, cli client.Client, m *clusterv1.Machine, ippool ipamv1.IPPool) (string, error) {
	m3Machine := &infrav1beta1.Metal3Machine{}
	namespace := m.GetObjectMeta().GetNamespace()
	err := cli.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      m.Spec.InfrastructureRef.Name},
		m3Machine)

	if err != nil {
		return "", fmt.Errorf("couldn't get a Metal3Machine within namespace %s with name %s : %w", namespace, m.Spec.InfrastructureRef.Name, err)
	}
	m3DataList := &infrav1beta1.Metal3DataList{}
	m3Data := &infrav1beta1.Metal3Data{}
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
		return "", errors.New("couldn't find a matching Metal3Data object")
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
		return "", errors.New("couldn't find a matching IPAddress object")
	}

	return string(IPAddress.Spec.Address), nil
}

// MachineTiIPAddress gets IPAddress based on machine, from machine -> m3machine -> m3data -> IPAddress.
// This is a duplicate of MachineToIPAddress, but for v1beta1 API. Remove this function when we switch to CAPI v1beta2 API only.
func MachineToIPAddress1beta1(ctx context.Context, cli client.Client, m *clusterv1.Machine, ippool ipamv1.IPPool) (string, error) {
	m3Machine := &infrav1beta1.Metal3Machine{}
	err := cli.Get(ctx, types.NamespacedName{
		Namespace: m.Namespace,
		Name:      m.Spec.InfrastructureRef.Name},
		m3Machine)

	if err != nil {
		return "", fmt.Errorf("couldn't get a Metal3Machine within namespace %s with name %s : %w", m.Namespace, m.Spec.InfrastructureRef.Name, err)
	}
	m3DataList := &infrav1beta1.Metal3DataList{}
	m3Data := &infrav1beta1.Metal3Data{}
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
		return "", errors.New("couldn't find a matching Metal3Data object")
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
		return "", errors.New("couldn't find a matching IPAddress object")
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
		return errors.New("couldn't read private key")
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
	client, err := ssh.Dial("tcp", machineIP+":22", cfg)
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
	maps.Copy(crd.Labels, labels)
	// Update the CRD
	err = c.Update(ctx, crd)
	if err != nil {
		return err
	}
	Logf("CRD '%s' labeled successfully\n", crdName)
	return nil
}

// GetCAPM3StableReleaseOfMinor returns latest stable version of minorRelease.
func GetStableReleaseOfMinor(ctx context.Context, releaseMarkerPrefix string, minorRelease string) (string, error) {
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
		return "", fmt.Errorf("parsing semver for %s: %w", minorReleaseVersion, err)
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
		return "", fmt.Errorf("no suitable release available for path %s and version %s", goProxyPath, minorReleaseVersion)
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
		return nil, fmt.Errorf("failed to get versions from url %s got %d %s", gomodulePath, resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	defer resp.Body.Close()

	rawResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		retryError := fmt.Errorf("failed to get versions: error reading goproxy response body: %w", err)
		return nil, retryError
	}
	parsedVersions := semver.Versions{}
	for s := range strings.SplitSeq(string(rawResponse), "\n") {
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
	WaitIntervals []any
}

func (input *BuildAndApplyKustomizationInput) validate() error {
	// If neither WaitForDeployment nor WatchDeploymentLogs is true, we don't need to validate the input
	if !input.WaitForDeployment && !input.WatchDeploymentLogs {
		return nil
	}
	if input.WaitForDeployment && input.WaitIntervals == nil {
		return errors.New("waitIntervals is expected if WaitForDeployment is set to true")
	}
	if input.WatchDeploymentLogs && input.LogPath == "" {
		return errors.New("logPath is expected if WatchDeploymentLogs is set to true")
	}
	if input.DeploymentName == "" || input.DeploymentNamespace == "" {
		return errors.New("deploymentName and DeploymentNamespace are expected if WaitForDeployment or WatchDeploymentLogs is true")
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

	deployment := &appsv1.Deployment{
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

	Logf("Running kubectl %s\n", strings.Join(aargs, " "))
	stdout, stderr, err := deleteCmd.Run(ctx)
	Logf("stderr:\n%s", string(stderr))
	Logf("stdout:\n%s", string(stdout))
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
				if err = p.GetClient().Create(ctx, &o); err != nil {
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

func ApplyBmh(ctx context.Context, e2eConfig *clusterctl.E2EConfig, clusterProxy framework.ClusterProxy, clusterNamespace string, specName string) {
	workingDir := "/opt/metal3-dev-env/"
	numNodes := int(*e2eConfig.MustGetInt32PtrVariable("NUM_NODES"))
	// Apply secrets and bmhs for [node_0 and node_1] in the management cluster to host the target management cluster
	for i := range numNodes {
		resource, err := os.ReadFile(filepath.Join(workingDir, fmt.Sprintf("bmhs/node_%d.yaml", i)))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(CreateOrUpdateWithNamespace(ctx, clusterProxy, resource, clusterNamespace)).ShouldNot(HaveOccurred())
	}
	clusterClient := clusterProxy.GetClient()
	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(clusterNamespace))
	WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(clusterNamespace)},
		Replicas:  numNodes,
		Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
	})
	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(clusterNamespace))
}

// WaitForResourceVersionsToStabilize waits for the resource versions of the specified GVKs in the given namespace to stabilize.
func WaitForResourceVersionsToStabilize(ctx context.Context, clusterProxy framework.ClusterProxy, namespace string, gvkList []schema.GroupVersionKind, intervals []any) {
	for _, gvk := range gvkList {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)
		Expect(clusterProxy.GetClient().List(ctx, list, client.InNamespace(namespace))).To(Succeed())
		Logf("Found %d resources of kind %s \n", len(list.Items), gvk.Kind)

		for _, obj := range list.Items {
			Logf("Found res %s  of kind %s \n", obj.GetName(), obj.GetKind())
			if obj.GetDeletionTimestamp() != nil {
				Logf("Res %s  of kind %s has deletionTimeStamp \n", obj.GetName(), obj.GetKind())
				key := client.ObjectKey{Namespace: namespace, Name: obj.GetName()}
				Eventually(func() error {
					err := clusterProxy.GetClient().Get(ctx, key, &obj)
					if apierrors.IsNotFound(err) {
						Logf("Res %s  of kind %s has deleted \n", obj.GetName(), obj.GetKind())
						return nil // Deleted
					}
					Logf("Res %s  of kind %s still exists \n", obj.GetName(), obj.GetKind())
					return err // Still exists
				}, intervals...).Should(Succeed(), "Resource %s/%s of kind %s not deleted", namespace, obj.GetName(), gvk.Kind)
			}
		}
	}

	// Check if the number of Metal3Data and Machine resources are equal
	Logf("Checking if the number of Metal3Data and Machine resources are equal in namespace %s", namespace)
	Eventually(func() bool {
		return IsMetal3DataCountEqualToMachineCount(ctx, clusterProxy.GetClient(), namespace)
	}, intervals...).Should(BeTrue(), "Metal3Data and Machine counts are not equal")

	// Check resource versions of the specified GVKs in the given namespace are stabilized
	var prevResourceVersions map[string]string
	Eventually(func(g Gomega) {
		currResourceVersions := getResourceVersions(ctx, clusterProxy.GetClient(), namespace, gvkList)
		if prevResourceVersions != nil {
			g.Expect(currResourceVersions).To(BeComparableTo(prevResourceVersions))
		}
		prevResourceVersions = currResourceVersions
	}, intervals...).Should(Succeed(), "resourceVersions never became stable")
}

func getResourceVersions(ctx context.Context, c client.Client, namespace string, gvkList []schema.GroupVersionKind) map[string]string {
	resourceVersions := make(map[string]string)
	for _, gvk := range gvkList {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)
		err := c.List(ctx, list, client.InNamespace(namespace))
		Expect(err).To(Succeed(), "Failed to list resources for kind %s", gvk.Kind)
		Logf("Found %d resources of kind %s checking resourceVersions stability \n", len(list.Items), gvk.Kind)
		for _, obj := range list.Items {
			key := fmt.Sprintf("%s/%s/%s", obj.GetKind(), obj.GetNamespace(), obj.GetName())
			resourceVersions[key] = obj.GetResourceVersion()
		}
	}
	return resourceVersions
}

func IsMetal3DataCountEqualToMachineCount(ctx context.Context, c client.Client, namespace string) bool {
	m3DataList := &infrav1beta1.Metal3DataList{}
	machineList := &clusterv1.MachineList{}

	err1 := c.List(ctx, m3DataList, client.InNamespace(namespace))
	err2 := c.List(ctx, machineList, client.InNamespace(namespace))

	if err1 != nil || err2 != nil {
		Logf("Error listing Metal3Data or Machine resources: %v %v", err1, err2)
		return false
	}

	return len(m3DataList.Items) == len(machineList.Items)
}

// getControlplaneNodes returns a list of control plane nodes in the cluster.
func getControlplaneNodes(ctx context.Context, clientSet *kubernetes.Clientset) *corev1.NodeList {
	controlplaneNodesRequirement, err := labels.NewRequirement("node-role.kubernetes.io/control-plane", selection.Exists, []string{})
	Expect(err).ToNot(HaveOccurred(), "Failed to set up worker Node requirements")
	controlplaneNodesSelector := labels.NewSelector().Add(*controlplaneNodesRequirement)
	controlplaneListOptions := metav1.ListOptions{LabelSelector: controlplaneNodesSelector.String()}
	controlplaneNodes, err := clientSet.CoreV1().Nodes().List(ctx, controlplaneListOptions)
	Expect(err).ToNot(HaveOccurred(), "Failed to get controlplane nodes")
	Logf("controlplaneNodes found %v", len(controlplaneNodes.Items))
	return controlplaneNodes
}

// untaintNodes removes the specified taints from the given nodes.
// Returns the count of nodes that were successfully untainted.
func untaintNodes(ctx context.Context, targetClusterClient client.Client, nodes *corev1.NodeList, taints []corev1.Taint) (count int) {
	count = 0
	for i := range nodes.Items {
		Logf("Untainting node %v ...", nodes.Items[i].Name)
		newNode, changed := removeTaint(&nodes.Items[i], taints)
		if changed {
			patchHelper, err := patch.NewHelper(&nodes.Items[i], targetClusterClient)
			Expect(err).ToNot(HaveOccurred())
			Expect(patchHelper.Patch(ctx, newNode)).To(Succeed(), "Failed to patch node")
			count++
		}
	}
	return
}

// removeTaint removes the specified taints from a node and returns the modified node.
// Returns true if any taint was removed.
func removeTaint(node *corev1.Node, taints []corev1.Taint) (*corev1.Node, bool) {
	newNode := node.DeepCopy()
	nodeTaints := newNode.Spec.Taints
	if len(nodeTaints) == 0 {
		return newNode, false
	}

	if !taintExists(nodeTaints, taints) {
		return newNode, false
	}

	newTaints, _ := deleteTaint(nodeTaints, taints)
	newNode.Spec.Taints = newTaints
	return newNode, true
}

// taintExists checks if any of the specified taints exist in the node's taints.
func taintExists(taints []corev1.Taint, taintsToFind []corev1.Taint) bool {
	for _, taint := range taints {
		for i := range taintsToFind {
			if taint.MatchTaint(&taintsToFind[i]) {
				return true
			}
		}
	}
	return false
}

// CreateSecret creates a secret in the specified namespace with the given data.
func CreateSecret(ctx context.Context, client client.Client, secretNamespace, secretName string, data map[string]string) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		StringData: data,
	}
	Expect(client.Create(ctx, &secret)).NotTo(HaveOccurred(), fmt.Sprintf("Failed to create secret '%s/%s'", secretNamespace, secretName))
}

type WaitForBmhInProvisioningStateInput struct {
	Client          client.Client
	Bmh             bmov1alpha1.BareMetalHost
	State           bmov1alpha1.ProvisioningState
	UndesiredStates []bmov1alpha1.ProvisioningState
}

func WaitForBmhInProvisioningState(ctx context.Context, input WaitForBmhInProvisioningStateInput, intervals ...interface{}) {
	Eventually(func(g Gomega) {
		bmh := bmov1alpha1.BareMetalHost{}
		key := types.NamespacedName{Namespace: input.Bmh.Namespace, Name: input.Bmh.Name}
		g.Expect(input.Client.Get(ctx, key, &bmh)).To(Succeed())

		currentStatus := bmh.Status.Provisioning.State

		// Check if the current state matches any of the undesired states
		if isUndesiredState(currentStatus, input.UndesiredStates) {
			StopTrying(fmt.Sprintf("BMH is in an unexpected state: %s", currentStatus)).Now()
		}

		g.Expect(currentStatus).To(Equal(input.State))
	}, intervals...).Should(Succeed())
}

func isUndesiredState(currentState bmov1alpha1.ProvisioningState, undesiredStates []bmov1alpha1.ProvisioningState) bool {
	if undesiredStates == nil {
		return false
	}

	for _, state := range undesiredStates {
		if (state == "" && currentState == "") || currentState == state {
			return true
		}
	}
	return false
}

// deleteTaint removes the specified taints from the node's taints and returns the result.
// Returns true if any taint was deleted.
func deleteTaint(taints []corev1.Taint, taintsToDelete []corev1.Taint) ([]corev1.Taint, bool) {
	newTaints := []corev1.Taint{}
	deleted := false
	for i := range taints {
		currentTaintDeleted := false
		for _, taintToDelete := range taintsToDelete {
			if taintToDelete.MatchTaint(&taints[i]) {
				deleted = true
				currentTaintDeleted = true
			}
		}
		if !currentTaintDeleted {
			newTaints = append(newTaints, taints[i])
		}
	}
	return newTaints, deleted
}

type UpgradeControlPlaneInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
	K8sToVersion          string
	K8sFromVersion        string
}

func UpgradeControlPlane(ctx context.Context, inputGetter func() UpgradeControlPlaneInput) {
	input := inputGetter()
	e2eConfig := input.E2EConfig
	clusterClient := input.BootstrapClusterProxy.GetClient()
	targetClusterClient := input.TargetCluster.GetClient()
	clientSet := input.TargetCluster.GetClientSet()
	k8sToVersion := input.K8sToVersion
	k8sFromVersion := input.K8sFromVersion
	specName := input.SpecName
	namespace := input.Namespace
	clusterName := input.ClusterName
	numberOfControlplane := int(*e2eConfig.MustGetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
	var (
		controlplaneTaints = []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule}}
	)
	// Upgrade process starts here
	// Download node image
	By("Download image")
	imageURL, imageChecksum := EnsureImage(k8sToVersion)

	By("Create new KCP Metal3MachineTemplate with upgraded image to boot")
	m3MachineTemplateName := clusterName + "-controlplane"
	newM3MachineTemplateName := clusterName + k8sToVersion + "-new-controlplane"
	CreateNewM3MachineTemplate(ctx, namespace, newM3MachineTemplateName, m3MachineTemplateName, clusterClient, imageURL, imageChecksum)

	Byf("Update KCP to upgrade k8s version and binaries from %s to %s", k8sFromVersion, k8sToVersion)
	kcpObj := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      clusterClient,
		ClusterName: clusterName,
		Namespace:   namespace,
	})
	helper, err := patch.NewHelper(kcpObj, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.MachineTemplate.Spec.InfrastructureRef.Name = newM3MachineTemplateName
	kcpObj.Spec.Version = k8sToVersion
	kcpObj.Spec.Rollout.Strategy.RollingUpdate.MaxSurge.IntVal = 0
	Expect(helper.Patch(ctx, kcpObj)).To(Succeed())

	Byf("Wait until %d BMH(s) are in deprovisioning state", 1)
	WaitForNumBmhInState(ctx, bmov1alpha1.StateDeprovisioning, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  1,
		Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning"),
	})

	Byf("Wait until %d Control Plane machine(s) become running and updated with the new %s k8s version", numberOfControlplane, k8sToVersion)
	runningAndUpgradedKCPMachines := func(machine clusterv1.Machine) bool {
		running := machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning
		upgraded := machine.Spec.Version == k8sToVersion
		_, isControlPlane := machine.GetLabels()[clusterv1.MachineControlPlaneLabel]
		return running && upgraded && isControlPlane
	}
	WaitForNumMachines(ctx, runningAndUpgradedKCPMachines, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  numberOfControlplane,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-running"),
	})

	By("Untaint Control Plane nodes")
	controlplaneNodes := getControlplaneNodes(ctx, clientSet)
	untaintNodes(ctx, targetClusterClient, controlplaneNodes, controlplaneTaints)

	By("Update maxSurge field in KubeadmControlPlane back to default value(1)")
	kcpObj = framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      clusterClient,
		ClusterName: clusterName,
		Namespace:   namespace,
	})
	helper, err = patch.NewHelper(kcpObj, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	kcpObj.Spec.Rollout.Strategy.RollingUpdate.MaxSurge.IntVal = 1
	for range 3 {
		err = helper.Patch(ctx, kcpObj)
		if err == nil {
			break
		}
		Logf("Failed to patch KCP maxSurge, retrying: %v", err)
		time.Sleep(30 * time.Second)
	}

	// Verify that all control plane nodes are using the k8s version
	Byf("Verify all %d control plane machines become running and updated with new %s k8s version", numberOfControlplane, k8sToVersion)
	WaitForNumMachines(ctx, runningAndUpgradedKCPMachines, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  numberOfControlplane,
		Intervals: e2eConfig.GetIntervals(specName, "wait-machine-running"),
	})
}

type InstallIRSOInput struct {
	E2EConfig             *clusterctl.E2EConfig
	ClusterProxy          framework.ClusterProxy
	IronicNamespace       string
	ClusterName           string
	IrsoOperatorKustomize string
	IronicKustomize       string
	LogPath               string
}

func InstallIRSO(ctx context.Context, input InstallIRSOInput) error {
	By("Create Ironic namespace")
	clusterClientSet := input.ClusterProxy.GetClientSet()
	ironicNamespaceObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: input.IronicNamespace,
		},
	}
	_, err := clusterClientSet.CoreV1().Namespaces().Create(ctx, ironicNamespaceObj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			Logf("Ironic namespace %q already exists, continuing", input.IronicNamespace)
		} else {
			Expect(err).ToNot(HaveOccurred(), "Unable to create the Ironic namespace")
		}
	}

	By(fmt.Sprintf("Installing IRSO from kustomization %s on the target cluster", input.IrsoOperatorKustomize))
	err = BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       input.IrsoOperatorKustomize,
		ClusterProxy:        input.ClusterProxy,
		WaitForDeployment:   true,
		WatchDeploymentLogs: true,
		LogPath:             input.LogPath,
		DeploymentName:      IRSOControllerManagerName,
		DeploymentNamespace: IRSOControllerNameSpace,
		WaitIntervals:       input.E2EConfig.GetIntervals("default", "wait-deployment"),
	})
	Expect(err).NotTo(HaveOccurred())

	By("Waiting for Ironic CRD to be available")
	Eventually(func(g Gomega) {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err = input.ClusterProxy.GetClient().Get(ctx, client.ObjectKey{
			Name: "ironics.ironic.metal3.io",
		}, crd)
		g.Expect(err).ToNot(HaveOccurred(), "Ironic CRD not found")
		// Check if CRD is established
		established := false
		for _, cond := range crd.Status.Conditions {
			if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
				established = true
				break
			}
		}
		g.Expect(established).To(BeTrue(), "Ironic CRD is not established yet")
	}, input.E2EConfig.GetIntervals("default", "wait-deployment")...).Should(Succeed())
	Logf("Ironic CRD is available and established")

	// Retry applying Ironic CR until it's successfully created
	Eventually(func(g Gomega) {
		err = BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
			Kustomization:       input.IronicKustomize,
			ClusterProxy:        input.ClusterProxy,
			WaitForDeployment:   false,
			WatchDeploymentLogs: false,
		})
		g.Expect(err).NotTo(HaveOccurred(), "Failed to apply Ironic CR")
		// Verify Ironic CR was actually created
		ironic := &irsov1alpha1.Ironic{}
		err = input.ClusterProxy.GetClient().Get(ctx, client.ObjectKey{
			Name:      "ironic",
			Namespace: input.IronicNamespace,
		}, ironic)
		g.Expect(err).NotTo(HaveOccurred(), "Ironic CR was not created")
		Logf("Ironic CR successfully created")
	}, input.E2EConfig.GetIntervals("default", "wait-deployment")...).Should(Succeed())

	return nil
}

// WaitForIronicReady waits until the given Ironic resource has Ready condition = True.
func WaitForIronicReady(ctx context.Context, input WaitForIronicInput) {
	Logf("Waiting for Ironic %q to be Ready", input.Name)

	Eventually(func(g Gomega) {
		ironic := &irsov1alpha1.Ironic{}
		err := input.Client.Get(ctx, client.ObjectKey{
			Namespace: input.Namespace,
			Name:      input.Name,
		}, ironic)
		g.Expect(err).ToNot(HaveOccurred())

		ready := false
		for _, cond := range ironic.Status.Conditions {
			if cond.Type == string(irsov1alpha1.IronicStatusReady) && cond.Status == metav1.ConditionTrue && ironic.Status.InstalledVersion != "" {
				ready = true
				break
			}
		}
		g.Expect(ready).To(BeTrue(), "Ironic %q is not Ready yet", input.Name)
	}, input.Intervals...).Should(Succeed())

	Logf("Ironic %q is Ready", input.Name)
}

// WaitForIronicInput bundles the parameters for WaitForIronicReady.
type WaitForIronicInput struct {
	Client    client.Client
	Name      string
	Namespace string
	Intervals []interface{} // e.g. []interface{}{time.Minute * 15, time.Second * 5}
}

// InstallBMOInput bundles parameters for InstallBMO.
type InstallBMOInput struct {
	E2EConfig        *clusterctl.E2EConfig
	ClusterProxy     framework.ClusterProxy
	Namespace        string // Namespace where BMO will run (shared with Ironic)
	BmoKustomization string // Kustomization path or URL for BMO manifests
	LogFolder        string // Optional explicit log folder; if empty a default is derived
	WaitIntervals    []any  // Optional override; if nil uses default e2e config intervals
	WatchLogs        bool   // Whether to watch deployment logs
}

// InstallBMO installs the Baremetal Operator (BMO) in the target cluster similar to InstallIRSO.
func InstallBMO(ctx context.Context, input InstallBMOInput) error {
	By("Ensure BMO namespace exists")
	clientset := input.ClusterProxy.GetClientSet()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: input.Namespace}}
	_, err := clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			Logf("Namespace %q already exists, continuing", input.Namespace)
		} else {
			return fmt.Errorf("failed creating namespace %q: %w", input.Namespace, err)
		}
	}

	// Determine log folder
	logFolder := input.LogFolder
	if logFolder == "" {
		logFolder = filepath.Join(os.TempDir(), "target_cluster_logs", "bmo-deploy-logs", input.ClusterProxy.GetName())
	}
	intervals := input.WaitIntervals
	if intervals == nil {
		intervals = input.E2EConfig.GetIntervals("default", "wait-deployment")
	}

	By(fmt.Sprintf("Installing BMO from kustomization %s on the target cluster", input.BmoKustomization))
	err = BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       input.BmoKustomization,
		ClusterProxy:        input.ClusterProxy,
		WaitForDeployment:   true,
		WatchDeploymentLogs: input.WatchLogs,
		LogPath:             logFolder,
		DeploymentName:      "baremetal-operator-controller-manager",
		DeploymentNamespace: input.Namespace,
		WaitIntervals:       intervals,
	})
	if err != nil {
		return fmt.Errorf("failed installing BMO: %w", err)
	}

	By("BMO deployment applied and available")
	return nil
}

type UninstallIRSOAndIronicResourcesInput struct {
	E2EConfig             *clusterctl.E2EConfig
	ClusterProxy          framework.ClusterProxy
	IronicNamespace       string
	IrsoOperatorKustomize string
	IronicKustomization   string
	IsDevEnvUninstall     bool
}

// UninstallIRSOAndIronicResources removes the IRSO deployment, Ironic CR, IronicDatabase CR (if present), and related secrets.
func UninstallIRSOAndIronicResources(ctx context.Context, input UninstallIRSOAndIronicResourcesInput) error {
	if input.IsDevEnvUninstall {
		ironicObj := &irsov1alpha1.Ironic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ironic",
				Namespace: input.IronicNamespace,
			},
		}
		err := input.ClusterProxy.GetClient().Delete(ctx, ironicObj)
		Expect(err).ToNot(HaveOccurred(), "Failed to delete Ironic")
	} else {
		By("Remove Ironic CR in the cluster " + input.ClusterProxy.GetName())
		err := BuildAndRemoveKustomization(ctx, input.IronicKustomization, input.ClusterProxy)
		Expect(err).NotTo(HaveOccurred())
	}

	By("Remove Ironic Service Deployment in the cluster " + input.ClusterProxy.GetName())
	RemoveDeployment(ctx, func() RemoveDeploymentInput {
		return RemoveDeploymentInput{
			ClusterProxy: input.ClusterProxy,
			Namespace:    input.IronicNamespace,
			Name:         "ironic-service",
		}
	})

	if input.IsDevEnvUninstall {
		By("Remove Ironic Standalone Operator Deployment in the cluster " + input.ClusterProxy.GetName())
		RemoveDeployment(ctx, func() RemoveDeploymentInput {
			return RemoveDeploymentInput{
				ClusterProxy: input.ClusterProxy,
				Namespace:    IRSOControllerNameSpace,
				Name:         IRSOControllerManagerName,
			}
		})
	} else {
		By("Uninstalling IRSO operator via kustomize")
		err := BuildAndRemoveKustomization(ctx, input.IrsoOperatorKustomize, input.ClusterProxy)
		Expect(err).NotTo(HaveOccurred())
	}

	clusterClient := input.ClusterProxy.GetClient()

	// Delete secrets
	secretNames := []string{"ironic-auth", "ironic-cert", "ironic-cacert"}
	for _, s := range secretNames {
		Byf("Deleting secret %s", s)
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: s, Namespace: input.IronicNamespace}}
		err := clusterClient.Delete(ctx, secret)
		if err != nil {
			Logf("Failed to delete secret %s: %v", s, err)
		}
	}

	// Wait for secrets to be deleted
	By("Waiting for Ironic secrets to be deleted")
	Eventually(func() bool {
		for _, s := range secretNames {
			errS := clusterClient.Get(ctx, client.ObjectKey{Name: s, Namespace: input.IronicNamespace}, &corev1.Secret{})
			if errS == nil || !apierrors.IsNotFound(errS) {
				return false
			}
		}
		return true
	}, input.E2EConfig.GetIntervals("default", "wait-delete-ironic")...).Should(BeTrue(), "IRSO/Ironic resources not fully deleted")

	By("IRSO and Ironic resources uninstalled")
	return nil
}

// GetDeploymentTLSSecretVersion returns the tls-secret-version from the Deployment's pod template annotations
// or, if not present there, from the Deployment's own annotations.
func GetDeploymentTLSSecretVersion(ctx context.Context, deployKey *appsv1.Deployment, clusterProxy framework.ClusterProxy) (string, error) {
	deployment := &appsv1.Deployment{}
	key := client.ObjectKeyFromObject(deployKey)
	Eventually(func() error {
		return clusterProxy.GetClient().Get(ctx, key, deployment)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to get deployment %s", klog.KObj(deployKey))

	if deployment.Spec.Template.Annotations != nil {
		if v, ok := deployment.Spec.Template.Annotations["ironic.metal3.io/tls-secret-version"]; ok {
			return v, nil
		}
	}

	return "", nil
}

// GetDeploymentRevision returns the highest ReplicaSet revision for a Deployment.
// This reflects rollout progression (deployment.kubernetes.io/revision).
func GetDeploymentRevision(ctx context.Context, deployKey *appsv1.Deployment, clusterProxy framework.ClusterProxy) (int, error) {
	deployment := &appsv1.Deployment{}
	key := client.ObjectKeyFromObject(deployKey)
	Eventually(func() error {
		return clusterProxy.GetClient().Get(ctx, key, deployment)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to get deployment %s", klog.KObj(deployKey))

	if deployment.Annotations != nil {
		if v, ok := deployment.Annotations["deployment.kubernetes.io/revision"]; ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				return 0, fmt.Errorf("invalid deployment.kubernetes.io/revision value %q: %w", v, err)
			}
			return n, nil
		}
	}
	return 0, fmt.Errorf("deployment.kubernetes.io/revision annotation not found on deployment %s/%s", deployment.Namespace, deployment.Name)
}
