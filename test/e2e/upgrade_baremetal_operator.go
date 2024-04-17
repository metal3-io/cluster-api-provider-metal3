package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bmo_e2e "github.com/metal3-io/baremetal-operator/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

type upgradeBMOInput struct {
	E2EConfig         *clusterctl.E2EConfig
	ManagementCluster framework.ClusterProxy
	SpecName          string
}

// upgradeBMO upgrades BMO image to the latest.
func upgradeBMO(ctx context.Context, inputGetter func() upgradeBMOInput) {
	bmo_e2e.Logf("Starting BMO containers upgrade tests")
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

	bmo_e2e.Logf("namePrefix %v", namePrefix)
	bmo_e2e.Logf("bmoNamespace %v", bmoNamespace)
	bmo_e2e.Logf("bmoDeployName %v", bmoDeployName)
	bmo_e2e.Logf("containerRegistry %v", containerRegistry)
	bmo_e2e.Logf("bmoImageTag %v", bmoImageTag)

	By("Upgrading BMO deployment")
	deploy, err := clientSet.AppsV1().Deployments(bmoNamespace).Get(ctx, bmoDeployName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	for i, container := range deploy.Spec.Template.Spec.Containers {
		if container.Name == "manager" {
			bmo_e2e.Logf("Old image: %v", deploy.Spec.Template.Spec.Containers[i].Image)
			bmo_e2e.Logf("New image: %v", bmoImage)
			deploy.Spec.Template.Spec.Containers[i].Image = bmoImage
		}
	}

	_, err = clientSet.AppsV1().Deployments(bmoNamespace).Update(ctx, deploy, metav1.UpdateOptions{})
	Expect(err).ToNot(HaveOccurred())

	By("Waiting for BMO update to rollout")
	Eventually(func() bool {
		return DeploymentRolledOut(ctx, clientSet, bmoDeployName, bmoNamespace, deploy.Status.ObservedGeneration+1)
	},
		input.E2EConfig.GetIntervals(input.SpecName, "wait-deployment")...,
	).Should(BeTrue())

	By("BMO UPGRADE TESTS PASSED!")
}
