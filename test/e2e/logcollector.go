package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/describe"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
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
	if err := os.MkdirAll(qemuFolder, 0o750); err != nil {
		fmt.Fprintf(GinkgoWriter, "couldn't create directory %q : %s\n", qemuFolder, err)
	}

	serialLog := fmt.Sprintf("/var/log/libvirt/qemu/%s-serial0.log", VMName)
	if _, err := os.Stat(serialLog); os.IsNotExist(err) {
		return fmt.Errorf("error finding the serial log: %w", err)
	}

	copyCmd := fmt.Sprintf("sudo cp %s %s", serialLog, qemuFolder)
	cmd := exec.Command("/bin/sh", "-c", copyCmd) // #nosec G204:gosec
	if output, err := cmd.Output(); err != nil {
		return fmt.Errorf("something went wrong when executing '%s': %w, output: %s", cmd.String(), err, output)
	}
	setPermsCmd := fmt.Sprintf("sudo chmod -v 777 %s", path.Join(qemuFolder, filepath.Base(serialLog)))
	cmd = exec.Command("/bin/sh", "-c", setPermsCmd) // #nosec G204:gosec
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error changing file permissions after copying: %w, output: %s", err, output)
	}

	kubeadmCP := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      cli,
		ClusterName: m.Spec.ClusterName,
		Namespace:   m.Namespace,
	})

	if len(kubeadmCP.Spec.KubeadmConfigSpec.Users) < 1 {
		return fmt.Errorf("no valid credentials found: KubeadmConfigSpec.Users is empty")
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
	return fmt.Errorf("CollectInfrastructureLogs not implemented")
}

func (Metal3LogCollector) CollectMachinePoolLog(_ context.Context, _ client.Client, _ *expv1.MachinePool, _ string) error {
	return fmt.Errorf("CollectMachinePoolLog not implemented")
}

func FetchClusterLogs(clusterProxy framework.ClusterProxy, outputPath string) error {
	dirName := "/tmp/" + outputPath
	err := os.MkdirAll(dirName, 0775)
	if err != nil {
		return fmt.Errorf("couldn't list to containers: %v", err)
	}

	kubeconfigPath := clusterProxy.GetKubeconfigPath()
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fmt.Errorf("couldn't build kubeconfig: %v", err)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("couldn't create clientset: %v", err)
	}

	// Print the Pods' information to file
	// This does the same thing as:
	// kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" get pods -A
	out, err := exec.Command("kubectl", "get", "pods", "-A").Output()
	if err != nil {
		return fmt.Errorf("couldn't get pods: %v", err)
	}
	file := filepath.Join(dirName, "pods.log")
	err = os.WriteFile(file, out, 0644)
	if err != nil {
		return fmt.Errorf("couldn't write to file: %v", err)
	}

	// Get all namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("couldn't get namespaces: %v", err)
	}
	for _, namespace := range namespaces.Items {
		namespaceDir := filepath.Join(dirName, namespace.Name)

		// Get all pods in the namespace
		pods, err := clientset.CoreV1().Pods(namespace.Name).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("couldn't list pods in namespace %s: %v", namespace.Name, err)
		}
		for _, pod := range pods.Items {
			// Create a directory for each pod and the path to it if
			// it does not exist
			podDir := filepath.Join(namespaceDir, pod.Name)
			err = os.MkdirAll(podDir, 0775)
			if err != nil {
				return fmt.Errorf("couldn't write to file: %v", err)
			}

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
				return fmt.Errorf("couldn't describe pod %s in namespace %s: %v", pod.Name, namespace.Name, err)
			}

			// Print the Pod information to file
			file := filepath.Join(podDir, "stdout_describe.log")
			err = os.WriteFile(file, []byte(podDescription), 0644)
			if err != nil {
				return fmt.Errorf("couldn't write to file: %v", err)
			}

			// Get containers of the Pod
			for _, container := range pod.Spec.Containers {
				// Create a directory for each container
				containerDir := filepath.Join(podDir, container.Name)

				err := CollectContainerLogs(namespace.Name, pod.Name, container.Name, clientset, containerDir)
				if err != nil {
					return err
				}
			}
		}
	}
	fmt.Println("Successfully collected cluster logs.")
	return nil
}

func CollectContainerLogs(namespace string, podName string, containerName string, clientset *kubernetes.Clientset, outputPath string) error {
	err := os.MkdirAll(outputPath, 0775)
	if err != nil {
		return fmt.Errorf("couldn't list to containers: %v", err)
	}

	// Get logs of a container
	// Does the same thing as:
	// kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" logs -n "${NAMESPACE}" "${POD}" "${CONTAINER}"
	podLogOptions := corev1.PodLogOptions{
		Container: containerName,
	}
	podLogs, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOptions).Stream(context.Background())
	if err != nil {
		return fmt.Errorf("couldn't get container logs: %v", err)
	}
	defer podLogs.Close()

	// Read the logs into a string
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return fmt.Errorf("couldn't buffer container logs: %v", err)
	}
	podStr := buf.String()

	// Print the Pod information to file
	file := filepath.Join(outputPath, "stdout.log")
	err = os.WriteFile(file, []byte(podStr), 0644)
	if err != nil {
		return fmt.Errorf("couldn't write to file: %v", err)
	}

	return nil
}

func FetchManifests(_ framework.ClusterProxy) error {
	return fmt.Errorf("FetchManifests not implemented")
}
