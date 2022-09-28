package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func upgradeIronic(clientSet *kubernetes.Clientset) {
	Logf("Starting ironic containers upgrade tests")
	var (
		ironicNamespace   = e2eConfig.GetVariable("IRONIC_NAMESPACE")
		ironicDeployName  = "ironic"
		containerRegistry = e2eConfig.GetVariable("CONTAINER_REGISTRY")
		ironicImageTag    = e2eConfig.GetVariable("IRONIC_IMAGE_TAG")
		mariadbImageTag   = e2eConfig.GetVariable("MARIADB_IMAGE_TAG")
	)

	Logf("ironicNamespace %v", ironicNamespace)
	Logf("ironicDeployName %v", ironicDeployName)
	Logf("containerRegistry %v", containerRegistry)
	Logf("ironicImageTag %v", ironicImageTag)
	Logf("mariadbImageTag %v", mariadbImageTag)

	By("Upgrading ironic image based containers")
	deploy, err := clientSet.AppsV1().Deployments(ironicNamespace).Get(ctx, ironicDeployName, metav1.GetOptions{})
	Expect(err).To(BeNil())
	for i, container := range deploy.Spec.Template.Spec.Containers {
		switch container.Name {
		case
			"ironic",
			"ironic-dnsmasq",
			"ironic-log-watch",
			"ironic-inspector":
			deploy.Spec.Template.Spec.Containers[i].Image = containerRegistry + "/metal3-io/ironic:" + ironicImageTag
		case
			"mariadb":
			deploy.Spec.Template.Spec.Containers[i].Image = containerRegistry + "/metal3-io/mariadb:" + mariadbImageTag
		}
	}

	_, err = clientSet.AppsV1().Deployments(ironicNamespace).Update(ctx, deploy, metav1.UpdateOptions{})
	Expect(err).To(BeNil())

	By("Waiting for ironic update to rollout")
	Eventually(func() bool {
		return deploymentRolledOut(ctx, clientSet, ironicDeployName, ironicNamespace, deploy.Status.ObservedGeneration+1)
	},
		e2eConfig.GetIntervals(specName, "wait-deployment")...,
	).Should(Equal(true))

	By("IRONIC CONTAINERS UPGRADE TESTS PASSED!")
}
