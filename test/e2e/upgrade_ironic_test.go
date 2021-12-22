package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func upgradeIronic() {
	Logf("Starting ironic containers upgrade tests")
	var (
		namePrefix        = e2eConfig.GetVariable("NAMEPREFIX")
		clientSet         = targetCluster.GetClientSet()
		ironicNamespace   = e2eConfig.GetVariable("IRONIC_NAMESPACE")
		ironicDeployName  = namePrefix + "-ironic"
		containerRegistry = e2eConfig.GetVariable("CONTAINER_REGISTRY")
		ironicImageTag    = e2eConfig.GetVariable("IRONIC_IMAGE_TAG")
		mariadbImageTag   = e2eConfig.GetVariable("MARIADB_IMAGE_TAG")
	)

	Logf("namePrefix %v", namePrefix)
	Logf("ironicNamespace %v", ironicNamespace)
	Logf("ironicDeployName %v", ironicDeployName)
	Logf("containerRegistry %v", containerRegistry)
	Logf("ironicImageTag %v", ironicImageTag)
	Logf("mariadbImageTag %v", mariadbImageTag)

	By("Upgrading ironic image based containers")
	deploy, err := getIronicDeployment(clientSet, ironicDeployName, ironicNamespace)
	Expect(err).To(BeNil())
	for i, container := range deploy.Spec.Template.Spec.Containers {
		switch container.Name {
		case
			"ironic-api",
			"ironic-dnsmasq",
			"ironic-conductor",
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
		return deploymentRolledOut(clientSet, ironicDeployName, ironicNamespace, deploy.Status.ObservedGeneration+1)
	},
		e2eConfig.GetIntervals(specName, "wait-deployment")...,
	).Should(Equal(true))

	By("IRONIC CONTAINERS UPGRADE TESTS PASSED!")
}

func getIronicDeployment(clientSet *kubernetes.Clientset, ironicDeployName string, ironicNamespace string) (*appsv1.Deployment, error) {
	return clientSet.AppsV1().Deployments(ironicNamespace).Get(ctx, ironicDeployName, metav1.GetOptions{})
}

func deploymentRolledOut(clientSet *kubernetes.Clientset, ironicDeployName string, ironicNamespace string, desiredGeneration int64) bool {
	deploy, err := getIronicDeployment(clientSet, ironicDeployName, ironicNamespace)
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
