package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

type upgradeIronicInput struct {
	E2EConfig         *clusterctl.E2EConfig
	ManagementCluster framework.ClusterProxy
	SpecName          string
}

// upgradeIronic upgrades ironic image to the latest.
func upgradeIronic(ctx context.Context, inputGetter func() upgradeIronicInput) {
	Logf("Starting ironic containers upgrade tests")
	input := inputGetter()
	var (
		clientSet         = input.ManagementCluster.GetClientSet()
		namePrefix        = input.E2EConfig.GetVariable("NAMEPREFIX")
		ironicNamespace   = input.E2EConfig.GetVariable("IRONIC_NAMESPACE")
		ironicDeployName  = namePrefix + ironicSuffix
		containerRegistry = input.E2EConfig.GetVariable("CONTAINER_REGISTRY")
		ironicImageTag    = input.E2EConfig.GetVariable("IRONIC_IMAGE_TAG")
	)

	Logf("namePrefix %v", namePrefix)
	Logf("ironicNamespace %v", ironicNamespace)
	Logf("ironicDeployName %v", ironicDeployName)
	Logf("containerRegistry %v", containerRegistry)
	Logf("ironicImageTag %v", ironicImageTag)

	By("Upgrading ironic image based containers")
	deploy, err := clientSet.AppsV1().Deployments(ironicNamespace).Get(ctx, ironicDeployName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	for i, container := range deploy.Spec.Template.Spec.Containers {
		switch container.Name {
		case
			"ironic",
			"ironic-dnsmasq",
			"ironic-log-watch",
			"ironic-inspector":
			deploy.Spec.Template.Spec.Containers[i].Image = containerRegistry + "/metal3-io/ironic:" + ironicImageTag
		}
	}

	_, err = clientSet.AppsV1().Deployments(ironicNamespace).Update(ctx, deploy, metav1.UpdateOptions{})
	Expect(err).ToNot(HaveOccurred())

	By("Waiting for ironic update to rollout")
	Eventually(func() bool {
		return DeploymentRolledOut(ctx, clientSet, ironicDeployName, ironicNamespace, deploy.Status.ObservedGeneration+1)
	},
		input.E2EConfig.GetIntervals(input.SpecName, "wait-deployment")...,
	).Should(BeTrue())

	By("IRONIC CONTAINERS UPGRADE TESTS PASSED!")
}
