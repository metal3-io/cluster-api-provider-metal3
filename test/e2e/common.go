package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
)

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}

func Logf(format string, a ...interface{}) {
	fmt.Fprintf(GinkgoWriter, "INFO: "+format+"\n", a...)
}

func LogFromFile(logFile string) {
	data, err := os.ReadFile(filepath.Clean(logFile))
	Expect(err).To(BeNil(), "No log file found")
	Logf(string(data))
}

// return only the boolean value from ParseBool.
func getBool(s string) bool {
	b, err := strconv.ParseBool(s)
	Expect(err).To(BeNil())
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

func DumpSpecResourcesAndCleanup(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string, namespace string, intervalsGetter func(spec, key string) []interface{}, clusterName, clusterctlLogFolder string, skipCleanup bool) {
	Expect(os.RemoveAll(clusterctlLogFolder)).Should(Succeed())
	client := clusterProxy.GetClient()

	// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
	By(fmt.Sprintf("Dumping all the Cluster API resources in the %q namespace", namespace))
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    client,
		Namespace: namespace,
		LogPath:   filepath.Join(artifactFolder, "clusters", clusterProxy.GetName(), "resources"),
	})

	if !skipCleanup {
		By(fmt.Sprintf("Deleting cluster %s/%s", namespace, clusterName))
		// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
		// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
		// instead of DeleteClusterAndWait
		framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
			Client:    client,
			Namespace: namespace,
		}, intervalsGetter(specName, "wait-delete-cluster")...)
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
	imageChecksum = fmt.Sprintf("%s/%s.md5sum", imagesURL, rawImageName)

	// Check if node image with upgraded k8s version exist, if not download it
	imagePath := filepath.Join(ironicImageDir, imageName)
	rawImagePath := filepath.Join(ironicImageDir, rawImageName)
	if _, err := os.Stat(rawImagePath); err == nil {
		Logf("Local image %v already exists", rawImagePath)
	} else if os.IsNotExist(err) {
		Logf("Local image %v is not found \nDownloading..", rawImagePath)
		err = DownloadFile(imagePath, fmt.Sprintf("%s/%s", imageLocation, imageName))
		Expect(err).To(BeNil())
		cmd := exec.Command("qemu-img", "convert", "-O", "raw", imagePath, rawImagePath) // #nosec G204:gosec
		err = cmd.Run()
		Expect(err).To(BeNil())
		cmd = exec.Command("md5sum", rawImagePath) // #nosec G204:gosec
		output, err := cmd.CombinedOutput()
		Expect(err).To(BeNil())
		md5sum := strings.Fields(string(output))[0]
		err = os.WriteFile(fmt.Sprintf("%s/%s.md5sum", ironicImageDir, rawImageName), []byte(md5sum), 0544)
		Expect(err).To(BeNil())
		Logf("Image: %v downloaded", rawImagePath)
	} else {
		fmt.Fprintf(GinkgoWriter, "ERROR: %v\n", err)
		os.Exit(1)
	}
	return imageURL, imageChecksum
}

// DownloadFile will download a url and store it in local filepath.
func DownloadFile(filePath string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath.Clean(filePath))
	if err != nil {
		return err
	}
	defer func() {
		err := out.Close()
		Expect(err).To(BeNil(), "Error closing file")
	}()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
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
	Expect(len(machineDeployments)).To(Equal(1), "Expected exactly 1 MachineDeployment")
	machineDeploy := machineDeployments[0]
	patch := []byte(fmt.Sprintf(`{"spec": {"replicas": %d}}`, newReplicas))
	err := clusterClient.Patch(ctx, machineDeploy, client.RawPatch(types.MergePatchType, patch))
	Expect(err).To(BeNil(), "Failed to patch workers MachineDeployment")
}

// ScaleKubeadmControlPlane scales up/down KubeadmControlPlane object to desired replicas.
func ScaleKubeadmControlPlane(ctx context.Context, c client.Client, name client.ObjectKey, newReplicaCount int) {
	ctrlplane := controlplanev1.KubeadmControlPlane{}
	Expect(c.Get(ctx, name, &ctrlplane)).To(Succeed())
	helper, err := patch.NewHelper(&ctrlplane, c)
	Expect(err).To(BeNil(), "Failed to create new patch helper")

	ctrlplane.Spec.Replicas = pointer.Int32Ptr(int32(newReplicaCount))
	Expect(helper.Patch(ctx, &ctrlplane)).To(Succeed())
}

func DeploymentRolledOut(ctx context.Context, clientSet *kubernetes.Clientset, name string, namespace string, desiredGeneration int64) bool {
	deploy, err := clientSet.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	Expect(err).To(BeNil())
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

// ListIPPools logs the names, namespace, cluster and age of all IPPools in the namespace.
// Similar to kubectl get ippools.
func ListIPPools(ctx context.Context, c client.Client, opts ...client.ListOption) {
	IPPools := ipamv1.IPPoolList{}
	Expect(c.List(ctx, &IPPools, opts...)).To(Succeed())

	rows := make([][]string, len(IPPools.Items)+1)
	// Add column names
	rows[0] = []string{"Name:", "Namepace:"}
	for i, ippool := range IPPools.Items {
		rows[i+1] = []string{ippool.GetName(), ippool.GetNamespace()}
	}
	logTable("Listing IPPools", rows)
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

func GetMetal3Machines(ctx context.Context, c client.Client, cluster, namespace string) ([]infrav1.Metal3Machine, []infrav1.Metal3Machine) {
	var controlplane, workers []infrav1.Metal3Machine
	allMachines := &infrav1.Metal3MachineList{}
	Expect(c.List(ctx, allMachines, client.InNamespace(namespace))).To(Succeed())

	for _, machine := range allMachines.Items {
		if strings.Contains(machine.ObjectMeta.Name, "workers") {
			workers = append(workers, machine)
		} else {
			controlplane = append(controlplane, machine)
		}
	}

	return controlplane, workers
}

func GetIPPools(ctx context.Context, c client.Client, cluster, namespace string) ([]ipamv1.IPPool, []ipamv1.IPPool) {
	var bmv4, provisioning []ipamv1.IPPool
	allIPPools := &ipamv1.IPPoolList{}
	Expect(c.List(ctx, allIPPools, client.InNamespace(namespace))).To(Succeed())

	for _, ippool := range allIPPools.Items {
		if strings.Contains(ippool.ObjectMeta.Name, "baremetalv4") {
			bmv4 = append(bmv4, ippool)
		} else {
			provisioning = append(provisioning, ippool)
		}
	}

	return bmv4, provisioning
}

// GetIPPoolIndexes finds status indexes in IPPool
// and returns the indexes.
func GetIPPoolIndexes(ctx context.Context, ippool ipamv1.IPPool, poolName string, c client.Client) (map[string]ipamv1.IPAddressStr, error) {
	allocations := ippool.Status.Allocations
	m3DataList, m3MachineList := infrav1.Metal3DataList{}, infrav1.Metal3MachineList{}

	// var newAllocations map[string]ipamv1.IPAddressStr
	var newAllocations map[string]ipamv1.IPAddressStr
	for m3data_poolName, ipaddress := range allocations {
		fmt.Println("datapoolName:", m3data_poolName, "=>", "ipaddress:", ipaddress)
		m3dataName := strings.Split(m3data_poolName, "-"+poolName)[0]
		m3dataObj := FilterMetal3DatasByName(m3DataList.Items, m3dataName)
		m3mName, err := Metal3DataToMachineName(m3dataObj[0])
		if err != nil {
			return nil, nil
		}
		m3mObj := FilterMetal3MachinesByName(m3MachineList.Items, m3mName)
		getBmhFromM3Machine := func(m3Machine infrav1.Metal3Machine) (result bmov1alpha1.BareMetalHost) {
			Expect(c.Get(ctx, client.ObjectKey{Namespace: m3Machine.Namespace, Name: Metal3MachineToBmhName(m3Machine)}, &result)).To(Succeed())
			return result
		}
		bmh := getBmhFromM3Machine(m3mObj[0])
		newAllocations[bmh.Name] = ipaddress
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

// FilterMetal3DatasByName returns a filtered list of m3data object with specific name.
func FilterMetal3DatasByName(m3datas []infrav1.Metal3Data, name string) (result []infrav1.Metal3Data) {
	for _, m3data := range m3datas {
		if m3data.ObjectMeta.Name == name {
			result = append(result, m3data)
		}
	}
	return
}

// FilterMetal3MachinesByName returns a filtered list of m3machine object with specific name.
func FilterMetal3MachinesByName(m3ms []infrav1.Metal3Machine, name string) (result []infrav1.Metal3Machine) {
	for _, m3m := range m3ms {
		if m3m.ObjectMeta.Name == name {
			result = append(result, m3m)
		}
	}
	return
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
