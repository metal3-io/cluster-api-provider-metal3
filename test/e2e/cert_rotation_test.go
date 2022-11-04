package e2e

import (
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func certRotation(clientSet *kubernetes.Clientset, clusterClient client.Client) {
	Logf("Start the certificate rotation test")
	By("Check if Ironic pod is running")
	ironicNamespace := e2eConfig.GetVariable("NAMEPREFIX") + "-system"
	ironicDeploymentName := e2eConfig.GetVariable("NAMEPREFIX") + "-ironic"
	ironicDeployment, err := getDeployment(clusterClient, ironicDeploymentName, ironicNamespace)
	Eventually(func() error {
		ironicPod, err := getPodFromDeployment(clientSet, ironicDeployment, ironicNamespace)
		if err != nil {
			return err
		}
		if ironicPod.Status.Phase == corev1.PodRunning {
			return nil
		}

		return errors.New("Ironic pod is not in running state")
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(BeNil())

	time.Sleep(5 * time.Minute)

	By("Get the current number of time containers were restarted")
	containerNumRestart := make(map[string]int32)
	containerNumRestart["ironic-httpd"] = 0
	containerNumRestart["mariadb"] = 0
	Expect(err).To(BeNil())
	ironicPod, err := getPodFromDeployment(clientSet, ironicDeployment, ironicNamespace)
	Expect(err).To(BeNil())
	for _, container := range ironicPod.Status.ContainerStatuses {
		if _, exist := containerNumRestart[container.Name]; exist {
			containerNumRestart[container.Name] = container.RestartCount
		}
	}

	By("Force the cert-manager to regenerate the certificate by deleting the secrets")
	secretList := []string{
		"ironic-cert",
		"ironic-inspector-cert",
		"mariadb-cert",
	}
	for _, secretName := range secretList {
		err := clientSet.CoreV1().Secrets(ironicNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		Expect(err).To(BeNil(), "Cannot detele this secret: %s", secretName)
	}

	By("Wait for containers in the ironic pod to be restarted")
	Eventually(func() error {
		ironicPod, err := getPodFromDeployment(clientSet, ironicDeployment, ironicNamespace)
		if err != nil {
			return err
		}
		// check for container in containerNumRestart list
		for container := range containerNumRestart {
			notFound := true
			for _, ironicContainer := range ironicPod.Status.ContainerStatuses {
				if ironicContainer.Name == container {
					notFound = false
					break
				}
			}
			if notFound {
				return fmt.Errorf("%s container does not exist in Ironic pod", container)
			}
		}
		if ironicPod.Status.Phase == corev1.PodRunning {
			for _, container := range ironicPod.Status.ContainerStatuses {
				if oldNumRestart, exist := containerNumRestart[container.Name]; exist {
					if !(oldNumRestart < container.RestartCount) {
						return fmt.Errorf("%s is not restarted", container.Name)
					}
				}
			}
			return nil
		}
		return errors.New("Ironic pod is not in running state")
	}, e2eConfig.GetIntervals(specName, "wait-pod-restart")...).Should(BeNil())
	By("CERTIFICATE ROTATION TESTS PASSED!")
}

func getDeployment(clusterClient client.Client, deploymentName string, namespace string) (*appv1.Deployment, error) {
	deployment := &appv1.Deployment{}
	namespaceName := client.ObjectKey{
		Name:      deploymentName,
		Namespace: namespace,
	}
	err := clusterClient.Get(ctx, namespaceName, deployment)
	if err != nil {
		return nil, err
	}
	return deployment, nil
}

func getPodFromDeployment(clientSet *kubernetes.Clientset, deployment *appv1.Deployment, namespace string) (*corev1.Pod, error) {
	labelMap, err := metav1.LabelSelectorAsMap(deployment.Spec.Selector)
	Expect(err).To(BeNil())
	option := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelMap).String(),
	}
	podList, err := clientSet.CoreV1().Pods(namespace).List(ctx, option)
	if err != nil {
		return nil, err
	}
	Expect(len(podList.Items) == 1).To(BeTrue(), "The number of ironic pod is not equal to 1, but %v\n", len(podList.Items))
	return &podList.Items[0], nil
}
