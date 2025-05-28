package e2e

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CertRotationInput struct {
	E2EConfig         *clusterctl.E2EConfig
	ManagementCluster framework.ClusterProxy
	SpecName          string
}

func certRotation(ctx context.Context, inputGetter func() CertRotationInput) {
	Logf("Start the certificate rotation test")
	input := inputGetter()
	clientSet := input.ManagementCluster.GetClientSet()
	clusterClient := input.ManagementCluster.GetClient()
	mariadbEnabled := input.E2EConfig.GetVariable(ironicMariadb) == "true"
	By("Check if Ironic pod is running")
	ironicNamespace := input.E2EConfig.GetVariable("NAMEPREFIX") + "-system"
	ironicDeploymentName := input.E2EConfig.GetVariable("NAMEPREFIX") + ironicSuffix
	ironicDeployment, err := getDeployment(ctx, clusterClient, ironicDeploymentName, ironicNamespace)
	Expect(err).ToNot(HaveOccurred(), "failed to get ironic deployment")
	Eventually(func() error {
		ironicPod, err := getPodFromDeployment(ctx, clientSet, ironicDeployment, ironicNamespace)
		if err != nil {
			return err
		}
		if ironicPod.Status.Phase == corev1.PodRunning {
			return nil
		}

		return errors.New("ironic pod is not in running state")
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-deployment")...).Should(Succeed())

	time.Sleep(5 * time.Minute)

	By("Force the cert-manager to regenerate the certificate by deleting the secrets")
	secretList := []string{
		"ironic-cert",
	}
	if mariadbEnabled {
		secretList = append(secretList, "mariadb-cert")
	}
	for _, secretName := range secretList {
		err := clientSet.CoreV1().Secrets(ironicNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred(), "cannot delete this secret: %s", secretName)
	}
	By("Check if the secrets are deleted")
	for _, secretName := range secretList {
		_, err := clientSet.CoreV1().Secrets(ironicNamespace).Get(ctx, secretName, metav1.GetOptions{})
		Expect(err).To(HaveOccurred(), secretName)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}
	By("Wait until secrets are recreated")
	Eventually(func(g Gomega) {
		_, err := clientSet.CoreV1().Secrets(ironicNamespace).Get(ctx, secretList[0], metav1.GetOptions{})
		g.Expect(err).ToNot(HaveOccurred())
		if mariadbEnabled {
			_, err := clientSet.CoreV1().Secrets(ironicNamespace).Get(ctx, secretList[1], metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
		}
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-pod-restart")...).Should(Succeed())

	// TODO(Sunnatillo): Further extend the test with querying ironic endpoint with new certificates
	By("Check if all containers are running in ironic pod")
	Consistently(func() error {
		ironicPod, err := getPodFromDeployment(ctx, clientSet, ironicDeployment, ironicNamespace)
		Expect(err).ToNot(HaveOccurred(), "cannot get ironic from Deployment")

		if areAllContainersRunning(ironicPod) {
			return nil
		}
		return errors.New("not all containers are running")
	}, 200*time.Second, 20*time.Second).Should(Succeed(), "not all containers are in running state in ironic pod")
	By("CERTIFICATE ROTATION TESTS PASSED!")
}

func getDeployment(ctx context.Context, clusterClient client.Client, deploymentName string, namespace string) (*appv1.Deployment, error) {
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

func getPodFromDeployment(ctx context.Context, clientSet *kubernetes.Clientset, deployment *appv1.Deployment, namespace string) (*corev1.Pod, error) {
	labelMap, err := metav1.LabelSelectorAsMap(deployment.Spec.Selector)
	Expect(err).ToNot(HaveOccurred())
	option := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelMap).String(),
	}
	podList, err := clientSet.CoreV1().Pods(namespace).List(ctx, option)
	if err != nil {
		return nil, err
	}
	Expect(podList.Items).To(HaveLen(1), "The number of ironic pod is not equal to 1, but %v\n", len(podList.Items))
	return &podList.Items[0], nil
}

func areAllContainersRunning(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running == nil {
			return false
		}
	}
	return true
}
