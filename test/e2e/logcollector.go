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
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
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
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", copyCmd) // #nosec G204:gosec
	var output []byte
	if output, err = cmd.Output(); err != nil {
		return fmt.Errorf("something went wrong when executing '%s': %w, output: %s", cmd.String(), err, output)
	}
	setPermsCmd := "sudo chmod -v 777 " + path.Join(qemuFolder, filepath.Base(serialLog))
	cmd = exec.CommandContext(ctx, "/bin/sh", "-c", setPermsCmd) // #nosec G204:gosec
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
	return nil
}

func (Metal3LogCollector) CollectMachinePoolLog(_ context.Context, _ client.Client, _ *clusterv1.MachinePool, _ string) error {
	return nil
}

// specArtifactFolder returns the folder for the artifacts that the current
// spec collects in the given phase, for example
// <artifactFolder>/ip-reuse/afterEach. The first Ginkgo label of the spec is
// the label of the outermost Describe, and it identifies the test. If the spec
// has no label, specName is used.
func specArtifactFolder(artifactFolder, specName, phase string) string {
	name := specName
	if labels := CurrentSpecReport().Labels(); len(labels) > 0 {
		name = labels[0]
	}
	return filepath.Join(artifactFolder, name, phase)
}

// startedLogWatchers contains a key for each deployment log watcher that
// watchDeploymentLogs started. The mutex protects the map.
var (
	startedLogWatchersMu sync.Mutex
	startedLogWatchers   = map[string]bool{}
)

// watchDeploymentLogs streams the logs of all the pods of a deployment to
// <logPath>/<deployment name>. The deployment must exist. The function starts
// only one watcher for each cluster, deployment, and log path. Thus, if a
// caller calls it again, for example in an Eventually loop, the log lines are
// not written more than once. A call with a different log path starts a new
// watcher.
func watchDeploymentLogs(ctx context.Context, clusterProxy framework.ClusterProxy, deployment *appsv1.Deployment, logPath string) {
	key := strings.Join([]string{clusterProxy.GetName(), deployment.Namespace, deployment.Name, logPath}, "|")
	startedLogWatchersMu.Lock()
	if startedLogWatchers[key] {
		startedLogWatchersMu.Unlock()
		Logf("The log watcher for deployment %s/%s to %s already runs", deployment.Namespace, deployment.Name, logPath)
		return
	}
	startedLogWatchers[key] = true
	startedLogWatchersMu.Unlock()

	framework.WatchDeploymentLogsByName(ctx, framework.WatchDeploymentLogsByNameInput{
		GetLister:  clusterProxy.GetClient(),
		Cache:      clusterProxy.GetCache(ctx),
		ClientSet:  clusterProxy.GetClientSet(),
		Deployment: deployment,
		LogPath:    logPath,
	})
}

// CollectPhaseArtifacts collects the logs and manifests of each given cluster
// in <artifactFolder>/<feature>/<phase>/<cluster>/{logs,manifests}. Use it at
// the border between two test phases, when the next phase replaces nodes or
// pods and thus removes their logs. A nil cluster proxy is skipped. Errors are
// logged and do not fail the test.
func CollectPhaseArtifacts(artifactFolder, feature, phase string, clusterProxies ...framework.ClusterProxy) {
	folder := filepath.Join(artifactFolder, feature, phase)
	Byf("Fetch cluster logs and manifests [%s/%s]", feature, phase)
	for _, proxy := range clusterProxies {
		if proxy == nil {
			continue
		}
		if err := FetchClusterLogs(proxy, folder); err != nil {
			Logf("Error fetching logs for cluster %s: %v", proxy.GetName(), err)
		}
		if err := FetchManifests(proxy, folder); err != nil {
			Logf("Error fetching manifests for cluster %s: %v", proxy.GetName(), err)
		}
	}
}

// FetchManifests fetches relevant Metal3, CAPI, and Kubernetes core resources
// and dumps them to files in
// <outputPath>/<cluster-name>/manifests/<namespace>/<Kind>/<name>.yaml.
func FetchManifests(clusterProxy framework.ClusterProxy, outputPath string) error {
	outputPath = filepath.Join(outputPath, clusterProxy.GetName(), "manifests")
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

	// manifestCRDs contains the full names (<plural>.<group>) of the CRDs to
	// dump. A full name selects exactly one CRD. If a CRD is not installed in
	// the cluster, it is skipped. Do not add Secrets or ConfigMaps, because the
	// artifacts are public and they can contain credentials.
	manifestCRDs := []string{
		// Cluster API core.
		"clusters.cluster.x-k8s.io",
		"clusterclasses.cluster.x-k8s.io",
		"machines.cluster.x-k8s.io",
		"machinesets.cluster.x-k8s.io",
		"machinedeployments.cluster.x-k8s.io",
		"machinehealthchecks.cluster.x-k8s.io",
		"machinepools.cluster.x-k8s.io",
		"machinedrainrules.cluster.x-k8s.io",
		"extensionconfigs.runtime.cluster.x-k8s.io",
		"ipaddressclaims.ipam.cluster.x-k8s.io",
		"ipaddresses.ipam.cluster.x-k8s.io",
		// Kubeadm bootstrap and control plane providers.
		"kubeadmconfigs.bootstrap.cluster.x-k8s.io",
		"kubeadmconfigtemplates.bootstrap.cluster.x-k8s.io",
		"kubeadmcontrolplanes.controlplane.cluster.x-k8s.io",
		"kubeadmcontrolplanetemplates.controlplane.cluster.x-k8s.io",
		// CAPM3.
		"metal3clusters.infrastructure.cluster.x-k8s.io",
		"metal3clustertemplates.infrastructure.cluster.x-k8s.io",
		"metal3machines.infrastructure.cluster.x-k8s.io",
		"metal3machinetemplates.infrastructure.cluster.x-k8s.io",
		"metal3datas.infrastructure.cluster.x-k8s.io",
		"metal3dataclaims.infrastructure.cluster.x-k8s.io",
		"metal3datatemplates.infrastructure.cluster.x-k8s.io",
		"metal3remediations.infrastructure.cluster.x-k8s.io",
		"metal3remediationtemplates.infrastructure.cluster.x-k8s.io",
		// IPAM.
		"ippools.ipam.metal3.io",
		"ipclaims.ipam.metal3.io",
		"ipaddresses.ipam.metal3.io",
		// BMO.
		"baremetalhosts.metal3.io",
		"hardwaredata.metal3.io",
		"firmwareschemas.metal3.io",
		"hostfirmwaresettings.metal3.io",
		"hostfirmwarecomponents.metal3.io",
		"hostupdatepolicies.metal3.io",
		"preprovisioningimages.metal3.io",
		"dataimages.metal3.io",
		"bmceventsubscriptions.metal3.io",
		"baremetalswitches.metal3.io",
		"hostclaims.metal3.io",
		"hostdeploypolicies.metal3.io",
		// IRSO.
		"ironics.ironic.metal3.io",
	}
	client := clusterProxy.GetClient()

	// Get all CustomResourceDefinitions (CRDs)
	crds := apiextensionsv1.CustomResourceDefinitionList{}
	if err := client.List(ctx, &crds); err != nil {
		return fmt.Errorf("could not list CRDs: %w", err)
	}

	selected := []apiextensionsv1.CustomResourceDefinition{}
	kindCount := map[string]int{}
	for _, crd := range crds.Items {
		if slices.Contains(manifestCRDs, crd.Name) {
			selected = append(selected, crd)
			kindCount[crd.Spec.Names.Kind]++
		}
	}

	for _, crd := range selected {
		version := storageVersion(crd)
		if version == "" {
			Logf("Skipping CRD %s: no storage version found", crd.Name)
			continue
		}
		gvr := schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  version,
			Resource: crd.Spec.Names.Plural,
		}
		// Two CRDs can have the same Kind, for example IPAddress in
		// ipam.metal3.io and in ipam.cluster.x-k8s.io. For these CRDs, add the
		// group to the folder name, so that their files do not overwrite each
		// other.
		kindDir := crd.Spec.Names.Kind
		if kindCount[kindDir] > 1 {
			kindDir += "." + crd.Spec.Group
		}
		// Do not stop on an error, so that one bad CRD does not prevent the
		// dump of the other CRDs.
		if err := dumpGVR(ctx, dynamicClient, gvr, outputPath, kindDir); err != nil {
			Logf("Error fetching manifests for %s: %v", crd.Name, err)
		}
	}
	Logf("Successfully collected manifests for cluster %s.", clusterProxy.GetName())
	return nil
}

// FetchClusterLogs fetches logs from all pods in the cluster and writes them
// to files in <outputPath>/<cluster-name>/logs/<namespace>/<pod>/<container>.
// The pod list, with the node of each pod, is in
// <outputPath>/<cluster-name>/logs/pods.log.
func FetchClusterLogs(clusterProxy framework.ClusterProxy, outputPath string) error {
	ctx := context.Background()
	baseDir := filepath.Join(outputPath, clusterProxy.GetName(), "logs")
	// Ensure the base directory exists
	if err := os.MkdirAll(baseDir, 0o750); err != nil {
		return fmt.Errorf("couldn't create directory: %w", err)
	}

	// Get the clientset
	clientset := clusterProxy.GetClientSet()

	// Print the Pods' information to file
	// This does the same thing as:
	// kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" get pods -A
	outputFile := filepath.Join(baseDir, "pods.log")
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
		// Events show what the controllers did to the objects (BMH, Machine,
		// KCP, ...). The pod description shows only the events of the pod.
		if err := writeNamespaceEvents(ctx, clientset, namespace.Name, filepath.Join(baseDir, namespace.Name)); err != nil {
			Logf("couldn't collect events in namespace %s: %v", namespace.Name, err)
		}

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

			podDir := filepath.Join(baseDir, namespace.Name, pod.Name)
			if err = os.MkdirAll(podDir, 0o750); err != nil {
				return fmt.Errorf("couldn't create directory: %w", err)
			}
			err = writeToFile([]byte(podDescription), "stdout_describe.log", podDir)
			if err != nil {
				return fmt.Errorf("couldn't write to file: %w", err)
			}

			// Get the restart count of each container. If a container
			// restarted, also collect the logs of the previous run, because
			// they usually contain the cause of the crash.
			restartCounts := map[string]int32{}
			for _, status := range pod.Status.InitContainerStatuses {
				restartCounts[status.Name] = status.RestartCount
			}
			for _, status := range pod.Status.ContainerStatuses {
				restartCounts[status.Name] = status.RestartCount
			}

			// Get the init containers and the containers of the Pod. The
			// names are unique in a pod, so each container has its own
			// directory.
			containers := append(slices.Clone(pod.Spec.InitContainers), pod.Spec.Containers...)
			for _, container := range containers {
				// Create a directory for each container
				containerDir := filepath.Join(podDir, container.Name)
				if err := os.MkdirAll(containerDir, 0o750); err != nil {
					return fmt.Errorf("couldn't create directory: %w", err)
				}

				err := CollectContainerLogs(ctx, namespace.Name, pod.Name, container.Name, clientset, containerDir)
				if err != nil {
					Logf("Error %v.", err)
				}
				if restartCounts[container.Name] > 0 {
					err = collectContainerLogs(ctx, namespace.Name, pod.Name, container.Name, clientset, containerDir, true)
					if err != nil {
						Logf("Error collecting previous logs: %v.", err)
					}
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
	return collectContainerLogs(ctx, namespace, podName, containerName, clientset, outputPath, false)
}

// collectContainerLogs fetches the logs of a container and writes them to
// <outputPath>/stdout.log. If previous is true, it fetches the logs of the
// previous run of the container and writes them to
// <outputPath>/stdout-previous.log.
func collectContainerLogs(ctx context.Context, namespace, podName, containerName string, clientset *kubernetes.Clientset, outputPath string, previous bool) error {
	// Get logs of a container
	// Does the same thing as:
	// kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" logs [--previous] -n "${NAMESPACE}" "${POD}" "${CONTAINER}"
	podLogOptions := corev1.PodLogOptions{
		Container: containerName,
		Previous:  previous,
	}
	fileName := "stdout.log"
	if previous {
		fileName = "stdout-previous.log"
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

	err = writeToFile([]byte(podStr), fileName, outputPath)
	if err != nil {
		return fmt.Errorf("couldn't write to file: %w", err)
	}

	return nil
}

// writeNamespaceEvents writes the events of a namespace to
// <outputPath>/events.log, sorted from oldest to newest. It writes no file if
// the namespace has no events.
func writeNamespaceEvents(ctx context.Context, clientset *kubernetes.Clientset, namespace, outputPath string) error {
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("couldn't list events: %w", err)
	}
	if len(events.Items) == 0 {
		return nil
	}
	slices.SortStableFunc(events.Items, func(a, b corev1.Event) int {
		return eventTime(a).Compare(eventTime(b))
	})

	buf := new(bytes.Buffer)
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "LAST SEEN\tTYPE\tREASON\tOBJECT\tCOUNT\tMESSAGE")
	for _, event := range events.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s/%s\t%d\t%s\n",
			eventTime(event).UTC().Format(time.RFC3339),
			event.Type,
			event.Reason,
			event.InvolvedObject.Kind,
			event.InvolvedObject.Name,
			max(event.Count, 1),
			strings.TrimSpace(event.Message),
		)
	}
	if err = w.Flush(); err != nil {
		return fmt.Errorf("couldn't format events: %w", err)
	}
	return writeToFile(buf.Bytes(), "events.log", outputPath)
}

// eventTime returns the time when the event was last seen. Different event
// sources set different time fields, so it uses the first field that is set.
func eventTime(event corev1.Event) time.Time {
	switch {
	case event.Series != nil && !event.Series.LastObservedTime.IsZero():
		return event.Series.LastObservedTime.Time
	case !event.LastTimestamp.IsZero():
		return event.LastTimestamp.Time
	case !event.EventTime.IsZero():
		return event.EventTime.Time
	case !event.FirstTimestamp.IsZero():
		return event.FirstTimestamp.Time
	default:
		return event.CreationTimestamp.Time
	}
}

// CollectVbmctlContainerLogs collects logs and metadata from the vbmctl-managed
// lab containers (sushy-tools BMC emulator, image server, etc.). These run as
// docker containers on the host - not as Kubernetes pods - so the cluster log
// collection never captures them. Without these logs a mid-run crash of the BMC
// emulator (which knocks all BareMetalHosts back to "registering") is
// undiagnosable. Errors are logged but not returned, since log collection must
// never fail the surrounding cleanup.
func CollectVbmctlContainerLogs(ctx context.Context, outputPath string) {
	if err := os.MkdirAll(outputPath, 0o750); err != nil {
		Logf("couldn't create vbmctl container log directory %q: %v", outputPath, err)
		return
	}

	// All containers vbmctl creates are prefixed with "vbmctl-".
	out, err := exec.CommandContext(ctx, "docker", "ps", "-a",
		"--filter", "name=vbmctl-", "--format", "{{.Names}}").Output()
	if err != nil {
		Logf("couldn't list vbmctl containers: %v", err)
		return
	}

	names := strings.Fields(string(out))
	if len(names) == 0 {
		Logf("No vbmctl-* containers found to collect logs from")
		return
	}

	for _, name := range names {
		// docker logs writes to both stdout and stderr; capture both.
		logs, err := exec.CommandContext(ctx, "docker", "logs", name).CombinedOutput() // #nosec G204:gosec
		if err != nil {
			Logf("couldn't get logs for container %s: %v", name, err)
		}
		if err = writeToFile(logs, name+".log", outputPath); err != nil {
			Logf("couldn't write logs for container %s: %v", name, err)
		}

		inspect, err := exec.CommandContext(ctx, "docker", "inspect", name).Output() // #nosec G204:gosec
		if err != nil {
			Logf("couldn't inspect container %s: %v", name, err)
			continue
		}
		if err = writeToFile(inspect, name+".inspect.json", outputPath); err != nil {
			Logf("couldn't write inspect for container %s: %v", name, err)
		}
	}
	Logf("Successfully collected vbmctl container logs to %s", outputPath)
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

// storageVersion returns the storage version of the CRD. After an API upgrade,
// this is the new version (for example v1beta2), not the oldest stored
// version. It returns an empty string if the CRD has no storage version.
func storageVersion(crd apiextensionsv1.CustomResourceDefinition) string {
	for _, version := range crd.Spec.Versions {
		if version.Storage {
			return version.Name
		}
	}
	return ""
}

// DumpGVR fetches resources of a given GroupVersionResource
// and writes them to files in <outputPath>/<namespace>/<Kind>/<name>.yaml.
func DumpGVR(ctx context.Context, dynamicClient *dynamic.DynamicClient, gvr schema.GroupVersionResource, outputPath string) error {
	return dumpGVR(ctx, dynamicClient, gvr, outputPath, "")
}

// dumpGVR fetches resources of a given GroupVersionResource and writes them to
// files in <outputPath>/<namespace>/<kindDir>/<name>.yaml. If kindDir is empty,
// the Kind of each resource is used.
func dumpGVR(ctx context.Context, dynamicClient *dynamic.DynamicClient, gvr schema.GroupVersionResource, outputPath, kindDir string) error {
	resources, err := dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("could not get resources: %w", err)
	}
	if len(resources.Items) == 0 {
		return nil
	}

	// Write resource to file
	for _, resource := range resources.Items {
		dir := kindDir
		if dir == "" {
			dir = resource.GetKind()
		}
		filePath := filepath.Join(outputPath, resource.GetNamespace(), dir)
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
