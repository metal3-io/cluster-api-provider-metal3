package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func upgradeBMO() {
	Logf("Starting BMO containers upgrade tests")
	var (
		namePrefix = e2eConfig.GetVariable("NAMEPREFIX")
		clientSet  = targetCluster.GetClientSet()
		// BMO and Ironic share namespace
		bmoNamespace      = e2eConfig.GetVariable("IRONIC_NAMESPACE")
		bmoDeployName     = namePrefix + "-controller-manager"
		containerRegistry = e2eConfig.GetVariable("CONTAINER_REGISTRY")
		bmoImageTag       = e2eConfig.GetVariable("UPGRADED_BMO_IMAGE_TAG")
		bmoImage          = containerRegistry + "/metal3-io/baremetal-operator:" + bmoImageTag
	)

	Logf("namePrefix %v", namePrefix)
	Logf("bmoNamespace %v", bmoNamespace)
	Logf("bmoDeployName %v", bmoDeployName)
	Logf("containerRegistry %v", containerRegistry)
	Logf("bmoImageTag %v", bmoImageTag)

	By("Upgrading BMO deployment")
	deploy, err := clientSet.AppsV1().Deployments(bmoNamespace).Get(ctx, bmoDeployName, metav1.GetOptions{})
	Expect(err).To(BeNil())
	for i, container := range deploy.Spec.Template.Spec.Containers {
		if container.Name == "manager" {
			Logf("Old image: %v", deploy.Spec.Template.Spec.Containers[i].Image)
			Logf("New image: %v", bmoImage)
			deploy.Spec.Template.Spec.Containers[i].Image = bmoImage
		}
	}

	_, err = clientSet.AppsV1().Deployments(bmoNamespace).Update(ctx, deploy, metav1.UpdateOptions{})
	Expect(err).To(BeNil())

	By("Waiting for BMO update to rollout")
	Eventually(func() bool {
		return deploymentRolledOut(ctx, clientSet, bmoDeployName, bmoNamespace, deploy.Status.ObservedGeneration+1)
	},
		e2eConfig.GetIntervals(specName, "wait-deployment")...,
	).Should(Equal(true))

	By("BMO UPGRADE TESTS PASSED!")
}
