package e2e

import (
	"errors"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/labels"
	framework "sigs.k8s.io/cluster-api/test/framework"
)

func certRotation() {
	Logf("Start the certificate rotation test")
	By("Check if Ironic pod is running")
	targetClusterClientSet := targetCluster.GetClientSet()
	ironicNamespace := os.Getenv("NAMEPREFIX") + "-system"
	ironicDeploymentName := os.Getenv("NAMEPREFIX") + "-ironic"
	ironicDeployment, err := getDeployment(targetCluster, ironicDeploymentName, ironicNamespace)
	Eventually(func() error {
		ironicPod, err := getPodFromDeployment(targetCluster, ironicDeployment, ironicNamespace)
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
	containerNumRestart["ironic-api"] = 0
	containerNumRestart["ironic-conductor"] = 0
	containerNumRestart["mariadb"] = 0
	Expect(err).To(BeNil())
	ironicPod, err := getPodFromDeployment(targetCluster, ironicDeployment, ironicNamespace)
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
		err := targetClusterClientSet.CoreV1().Secrets(ironicNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		Expect(err).To(BeNil(), "Cannot detele this secret: %s", secretName)
	}

	By("Wait for containers in the ironic pod to be restarted")
	Eventually(func() error {
		ironicPod, err := getPodFromDeployment(targetCluster, ironicDeployment, ironicNamespace)
		if err != nil {
			return err
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

func getDeployment(targetCluster framework.ClusterProxy, deploymentName string, namespace string) (*appv1.Deployment, error) {
	deployment := &appv1.Deployment{}
	namespaceName := client.ObjectKey{
		Name:      deploymentName,
		Namespace: namespace,
	}
	err := targetCluster.GetClient().Get(ctx, namespaceName, deployment)
	if err != nil {
		return nil, err
	}
	return deployment, nil
}

func getPodFromDeployment(targetCluster framework.ClusterProxy, deployment *appv1.Deployment, namespace string) (*corev1.Pod, error) {
	labelMap, err := metav1.LabelSelectorAsMap(deployment.Spec.Selector)
	Expect(err).To(BeNil())
	option := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelMap).String(),
	}
	podList, err := targetCluster.GetClientSet().CoreV1().Pods(namespace).List(ctx, option)
	if err != nil {
		return nil, err
	}
	Expect(len(podList.Items) == 1).To(BeTrue(), "The number of ironic pod is not equal to 1, but %v\n", len(podList.Items))
	return &podList.Items[0], nil
}
