package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/describe"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Metal3LogCollector struct{}

// CollectMachineLog collects specific logs from machines.
func (Metal3LogCollector) CollectMachineLog(ctx context.Context, cli client.Client, m *clusterv1.Machine, outputPath string) error {
	VMName, err := MachineToVMName(ctx, cli, m)
	if err != nil {
		return fmt.Errorf("error while fetching the VM name: %w", err)
	}

	qemuFolder := path.Join(outputPath, VMName)
	if err = os.MkdirAll(qemuFolder, 0o750); err != nil {
		fmt.Fprintf(GinkgoWriter, "couldn't create directory %q : %s\n", qemuFolder, err)
	}

	serialLog := fmt.Sprintf("/var/log/libvirt/qemu/%s-serial0.log", VMName)
	if _, err = os.Stat(serialLog); os.IsNotExist(err) {
		return fmt.Errorf("error finding the serial log: %w", err)
	}

	copyCmd := fmt.Sprintf("sudo cp %s %s", serialLog, qemuFolder)
	cmd := exec.Command("/bin/sh", "-c", copyCmd) // #nosec G204:gosec
	var output []byte
	if output, err = cmd.Output(); err != nil {
		return fmt.Errorf("something went wrong when executing '%s': %w, output: %s", cmd.String(), err, output)
	}
	setPermsCmd := "sudo chmod -v 777 " + path.Join(qemuFolder, filepath.Base(serialLog))
	cmd = exec.Command("/bin/sh", "-c", setPermsCmd) // #nosec G204:gosec
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("error changing file permissions after copying: %w, output: %s", err, output)
	}

	kubeadmCP := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      cli,
		ClusterName: m.Spec.ClusterName,
		Namespace:   m.Namespace,
	})

	if len(kubeadmCP.Spec.KubeadmConfigSpec.Users) < 1 {
		return errors.New("no valid credentials found: KubeadmConfigSpec.Users is empty")
	}
	creds := kubeadmCP.Spec.KubeadmConfigSpec.Users[0]

	// get baremetal ip pool for retreiving ip addresses of controlpane and worker nodes
	baremetalv4Pool, _ := GetIPPools(ctx, cli, m.Spec.ClusterName, m.Namespace)
	Expect(baremetalv4Pool).ToNot(BeEmpty())

	ip, err := MachineToIPAddress(ctx, cli, m, baremetalv4Pool[0])
	if err != nil {
		return fmt.Errorf("couldn't get IP address of machine: %w", err)
	}

	commands := map[string]string{
		"cloud-final.log": "journalctl --no-pager -u cloud-final",
		"kubelet.log":     "journalctl --no-pager -u kubelet.service",
		"containerd.log":  "journalctl --no-pager -u containerd.service",
	}

	for title, cmd := range commands {
		err = runCommand(outputPath, title, ip, creds.Name, cmd)
		if err != nil {
			return fmt.Errorf("couldn't gather logs: %w", err)
		}
	}

	Logf("Successfully collected logs for machine %s", m.Name)
	return nil
}

func (Metal3LogCollector) CollectInfrastructureLogs(_ context.Context, _ client.Client, _ *clusterv1.Cluster, _ string) error {
	return errors.New("CollectInfrastructureLogs not implemented")
}

func (Metal3LogCollector) CollectMachinePoolLog(_ context.Context, _ client.Client, _ *clusterv1.MachinePool, _ string) error {
	return errors.New("CollectMachinePoolLog not implemented")
}

// FetchManifests fetches relevant Metal3, CAPI, and Kubernetes core resources
// and dumps them to a file.
func FetchManifests(clusterProxy framework.ClusterProxy, outputPath string) error {
	outputPath = filepath.Join(outputPath, "manifests")
	ctx := context.Background()
	restConfig := clusterProxy.GetRESTConfig()
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("could not create dynamic client: %w", err)
	}

	k8sCoreManifests := []string{
		// This list must contain the plural form of the Kubernetes core
		// resources
		"deployments",
		"replicasets",
	}

	// Set group and version to get Kubernetes core resources and dump them
	for _, manifest := range k8sCoreManifests {
		gvr := schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: manifest,
		}

		if err := DumpGVR(ctx, dynamicClient, gvr, outputPath); err != nil {
			return err
		}
	}

	manifests := []string{
		// This list contains all resources of interest that are NOT Kubernetes
		// core resources.
		"bmh",
		"hardwaredata",
		"cluster",
		"machine",
		"machinedeployment",
		"machinehealthchecks",
		"machinesets",
		"machinepools",
		"m3cluster",
		"m3machine",
		"metal3machinetemplate",
		"kubeadmconfig",
		"kubeadmconfigtemplates",
		"kubeadmcontrolplane",
		"ippool",
		"ipclaim",
		"ipaddress",
		"m3data",
		"m3dataclaim",
		"m3datatemplate",
		"ironic",
	}
	client := clusterProxy.GetClient()

	// Get all CustomResourceDefinitions (CRDs)
	crds := apiextensionsv1.CustomResourceDefinitionList{}
	if err := client.List(ctx, &crds); err != nil {
		return fmt.Errorf("could not list CRDs: %w", err)
	}
	if len(crds.Items) == 0 {
		return nil
	}

	// Check if the resource is in the manifest list.
	// If it is, dump it to a file
	for _, crd := range crds.Items {
		if crdIsInList(crd, manifests) {
			gvr := schema.GroupVersionResource{
				Group:    crd.Spec.Group,
				Version:  crd.Status.StoredVersions[0],
				Resource: crd.Spec.Names.Plural,
			}

			if err := DumpGVR(ctx, dynamicClient, gvr, outputPath); err != nil {
				return err
			}
		}
	}
	Logf("Successfully collected manifests for cluster %s.", clusterProxy.GetName())
	return nil
}

// FetchClusterLogs fetches logs from all pods in the cluster and writes them
// to files.
func FetchClusterLogs(clusterProxy framework.ClusterProxy, outputPath string) error {
	outputPath = filepath.Join(outputPath, "controller_logs")
	ctx := context.Background()
	// Ensure the base directory exists
	if err := os.MkdirAll(outputPath, 0o750); err != nil {
		return fmt.Errorf("couldn't create directory: %w", err)
	}

	// Get the clientset
	clientset := clusterProxy.GetClientSet()

	// Print the Pods' information to file
	// This does the same thing as:
	// kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" get pods -A
	outputFile := filepath.Join(outputPath, "pods.log")
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// List pods across all namespaces
	pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// Print header to file
	header := fmt.Sprintf("%-32s %-16s %-12s %-24s %-8s\n", "NAMESPACE", "NAME", "STATUS", "NODE", "AGE")
	if _, err = file.WriteString(header); err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	// Iterate through pods and print information
	for _, pod := range pods.Items {
		age := time.Since(pod.CreationTimestamp.Time).Round(time.Second)
		podInfo := fmt.Sprintf("%-32s %-16s %-12s %-24s %-8s\n",
			pod.Namespace,
			pod.Name,
			string(pod.Status.Phase),
			pod.Spec.NodeName,
			age,
		)

		if _, err = file.WriteString(podInfo); err != nil {
			return fmt.Errorf("error writing to file: %w", err)
		}
	}

	// Get all namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("couldn't get namespaces: %w", err)
	}
	for _, namespace := range namespaces.Items {
		// Get all pods in the namespace
		pods, err := clientset.CoreV1().Pods(namespace.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			Logf("couldn't list pods in namespace %s: %v", namespace.Name, err)
			continue
		}
		for _, pod := range pods.Items {
			// Get detailed information about the Pod
			// This does the same thing as:
			// kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" describe pods -n "${NAMESPACE}" "${POD}"
			describerSettings := describe.DescriberSettings{
				ShowEvents: true,
			}
			podDescriber := describe.PodDescriber{
				Interface: clientset,
			}
			podDescription, err := podDescriber.Describe(namespace.Name, pod.Name, describerSettings)
			if err != nil {
				Logf("couldn't describe pod %s in namespace %s: %v", pod.Name, namespace.Name, err)
				continue
			}

			machineName := pod.Spec.NodeName
			podDir := filepath.Join(outputPath, "machines", machineName, namespace.Name, pod.Name)
			if err = os.MkdirAll(podDir, 0o750); err != nil {
				return fmt.Errorf("couldn't create directory: %w", err)
			}
			err = writeToFile([]byte(podDescription), "stdout_describe.log", podDir)
			if err != nil {
				return fmt.Errorf("couldn't write to file: %w", err)
			}

			// Get containers of the Pod
			for _, container := range pod.Spec.Containers {
				// Create a directory for each container
				containerDir := filepath.Join(podDir, container.Name)
				if err := os.MkdirAll(containerDir, 0o750); err != nil {
					return fmt.Errorf("couldn't create directory: %w", err)
				}

				err := CollectContainerLogs(ctx, namespace.Name, pod.Name, container.Name, clientset, containerDir)
				if err != nil {
					Logf("Error %v.", err)
					continue
				}
			}
		}
	}
	Logf("Successfully collected logs for cluster %s.", clusterProxy.GetName())
	return nil
}

// CollectContainerLogs fetches logs from a specific container in a pod and
// writes them to a file.
func CollectContainerLogs(ctx context.Context, namespace string, podName string, containerName string, clientset *kubernetes.Clientset, outputPath string) error {
	// Get logs of a container
	// Does the same thing as:
	// kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" logs -n "${NAMESPACE}" "${POD}" "${CONTAINER}"
	podLogOptions := corev1.PodLogOptions{
		Container: containerName,
	}
	podLogs, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOptions).Stream(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get container logs: %w", err)
	}
	defer podLogs.Close()

	// Read the logs into a string
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return fmt.Errorf("couldn't buffer container logs: %w", err)
	}
	podStr := buf.String()

	err = writeToFile([]byte(podStr), "stdout.log", outputPath)
	if err != nil {
		return fmt.Errorf("couldn't write to file: %w", err)
	}

	return nil
}

// writeToFile writes content to a file,
// creating any missing directories in the path.
func writeToFile(content []byte, fileName string, filePath string) error {
	// Create any missing directories in the path
	err := os.MkdirAll(filePath, 0775)
	if err != nil {
		return fmt.Errorf("couldn't create directory: %w", err)
	}
	// Write content to file
	file := filepath.Join(filePath, fileName)
	err = os.WriteFile(file, content, 0600)
	if err != nil {
		return fmt.Errorf("couldn't write to file: %w", err)
	}
	return nil
}

// crdIsInList checks if a CustomResourceDefinition is in the provided list of
// resource names.
func crdIsInList(crd apiextensionsv1.CustomResourceDefinition, list []string) bool {
	plural := crd.Spec.Names.Plural
	singular := crd.Spec.Names.Singular
	shortNames := crd.Spec.Names.ShortNames

	for _, name := range list {
		if name == plural {
			return true
		}
		if name == singular {
			return true
		}
		if slices.Contains(shortNames, name) {
			return true
		}
	}
	return false
}

// DumpGVR fetches resources of a given GroupVersionResource
// and writes them to files.
func DumpGVR(ctx context.Context, dynamicClient *dynamic.DynamicClient, gvr schema.GroupVersionResource, outputPath string) error {
	resources, err := dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("could not get resources: %w", err)
	}
	if len(resources.Items) == 0 {
		return nil
	}

	// Write resource to file
	for _, resource := range resources.Items {
		filePath := filepath.Join(outputPath, resource.GetNamespace(), resource.GetKind())
		fileName := resource.GetName() + ".yaml"
		content, err := yaml.Marshal(resource)
		if err != nil {
			return fmt.Errorf("could not marshal content: %w", err)
		}
		err = writeToFile(content, fileName, filePath)
		if err != nil {
			return fmt.Errorf("couldn't write to file: %w", err)
		}
	}
	return nil
}

type FetchManifestsAndLogsInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	WorkloadClusterProxy  framework.ClusterProxy
	ArtifactFolder        string
	LogCollectionPath     string
}

func FetchManifestsAndLogs(inputGetter func() FetchManifestsAndLogsInput) {
	input := inputGetter()
	By("Fetch logs and manifests for path: " + input.LogCollectionPath)
	By("Fetch manifests from cluster " + input.WorkloadClusterProxy.GetName())
	err := FetchManifests(input.WorkloadClusterProxy, filepath.Join(input.ArtifactFolder, workloadClusterLogCollectionBasePath, input.WorkloadClusterProxy.GetName(), input.LogCollectionPath))
	if err != nil {
		Logf("Error fetching manifests for workload cluster: %v", err)
	}

	By("Fetch manifests from cluster " + input.BootstrapClusterProxy.GetName())
	err = FetchManifests(input.BootstrapClusterProxy, filepath.Join(input.ArtifactFolder, bootstrapClusterLogCollectionBasePath, input.LogCollectionPath))
	if err != nil {
		Logf("Error fetching manifests for bootstrap: %v", err)
	}

	By("Fetch logs from cluster " + input.WorkloadClusterProxy.GetName())
	err = FetchClusterLogs(input.WorkloadClusterProxy, filepath.Join(input.ArtifactFolder, workloadClusterLogCollectionBasePath, input.WorkloadClusterProxy.GetName(), input.LogCollectionPath))
	if err != nil {
		Logf("Error fetching logs from workload cluster: %v", err)
	}
	By("Fetch logs from cluster " + input.BootstrapClusterProxy.GetName())
	err = FetchClusterLogs(input.BootstrapClusterProxy, filepath.Join(input.ArtifactFolder, bootstrapClusterLogCollectionBasePath, input.LogCollectionPath))
	if err != nil {
		Logf("Error fetching logs from bootstrap cluster: %v", err)
	}
}
