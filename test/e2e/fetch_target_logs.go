package e2e

import (
	"bytes"
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/kubectl/pkg/describe"
)

func FetchTargetLogs() {
	dirName := "/tmp/target_cluster_logs"

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Printf("Error building kubeconfig: %v", err)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Error creating clientset: %v", err)
	}

	// Print the Pods' information to file
	// This does the same thing as:
	// kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" get pods -A
	out, err := exec.Command("kubectl", "get", "pods", "-A").Output()
	if err != nil {
		log.Printf("Error in getting pods: %v", err)
	}
	file := filepath.Join(dirName, "pods.log")
	err = os.WriteFile(file, out, 0644)
	if err != nil {
		log.Printf("Error in writing to file: %v", err)
	}

	// Get all namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error in getting namespaces: %v", err)
	}
	for _, namespace := range namespaces.Items {
		namespaceDir := filepath.Join(dirName, namespace.Name)

		// Get all pods in the namespace
		pods, err := clientset.CoreV1().Pods(namespace.Name).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			log.Printf("Error listing pods in namespace %s: %v", namespace.Name, err)
		}
		for _, pod := range pods.Items {
			// Create a directory for each pod and the path to it if
			// it does not exist
			podDir := filepath.Join(namespaceDir, pod.Name)
			err = os.MkdirAll(podDir, 0775)
			if err != nil {
				log.Printf("Error in writing to file: %v", err)
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
				log.Printf("Error in describing pod %s in namespace %s: %v", pod.Name, namespace.Name, err)
			}

			// Print the Pod information to file
			file := filepath.Join(podDir, "stdout_describe.log")
			err = os.WriteFile(file, []byte(podDescription), 0644)
			if err != nil {
				log.Printf("Error in writing to file: %v", err)
			}

			// Get containers of the Pod
			for _, container := range pod.Spec.Containers {
				// Create a directory for each container
				containerDir := filepath.Join(podDir, container.Name)
				err = os.MkdirAll(containerDir, 0775)
				if err != nil {
					log.Printf("Error listing to containers: %v", err)
				}

				containerLogs := GetContainerLogs(namespace.Name, pod.Name, container.Name, clientset)
				// Print the Pod information to file
				file := filepath.Join(containerDir, "stdout.log")
				err = os.WriteFile(file, []byte(containerLogs), 0644)
				if err != nil {
					log.Printf("Error in writing to file: %v", err)
				}
			}
		}
	}
}

func GetContainerLogs(namespace string, podName string, containerName string, clientset *kubernetes.Clientset) string {
	// Get logs of a container
	// Does the same thing as:
	// kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" logs -n "${NAMESPACE}" "${POD}" "${CONTAINER}"
	podLogOptions := corev1.PodLogOptions{
		Container: containerName,
	}
	podLogs, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOptions).Stream(context.Background())
	if err != nil {
		log.Printf("Error getting container logs: %v", err)
	}
	defer podLogs.Close()

	// Read the logs into a string
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		log.Printf("Error in buffering container logs: %v", err)
	}
	podStr := buf.String()

	return podStr
}
