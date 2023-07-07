package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type upgradeBMOInput struct {
	E2EConfig         *clusterctl.E2EConfig
	ManagementCluster framework.ClusterProxy
	SpecName          string
}

// upgradeBMO upgrades BMO image to the latest.
func upgradeBMO(ctx context.Context, inputGetter func() upgradeBMOInput) {
	Logf("Starting BMO containers upgrade tests")
	input := inputGetter()
	var (
		clientSet  = input.ManagementCluster.GetClientSet()
		namePrefix = input.E2EConfig.GetVariable("NAMEPREFIX")
		// BMO and Ironic share namespace
		bmoNamespace      = input.E2EConfig.GetVariable("IRONIC_NAMESPACE")
		bmoDeployName     = namePrefix + "-controller-manager"
		containerRegistry = input.E2EConfig.GetVariable("CONTAINER_REGISTRY")
		bmoImageTag       = input.E2EConfig.GetVariable("UPGRADED_BMO_IMAGE_TAG")
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
		return DeploymentRolledOut(ctx, clientSet, bmoDeployName, bmoNamespace, deploy.Status.ObservedGeneration+1)
	},
		input.E2EConfig.GetIntervals(input.SpecName, "wait-deployment")...,
	).Should(Equal(true))

	By("BMO UPGRADE TESTS PASSED!")
}
