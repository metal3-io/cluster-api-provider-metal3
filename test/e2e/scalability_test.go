package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
 * Apply scaled number of BMHs per batches and wait for the them to become available
 * The test pass when all the BMHs become available
 */

var _ = Describe("When testing scalability with fakeIPA [scalability]", Label("scalability"), func() {
	scalabilityTest()
})

func scalabilityTest() {
	It("Should apply BMHs per batches and wait for them to become available", func() {
		numNodes, _ := strconv.Atoi(e2eConfig.GetVariable("NUM_NODES"))
		batch, _ := strconv.Atoi(e2eConfig.GetVariable("BMH_BATCH_SIZE"))
		Logf("Starting scalability test")
		bootstrapClient := bootstrapClusterProxy.GetClient()
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))
		applyBatchBmh := func(from int, to int) {
			Logf("Apply BMH batch from node_%d to node_%d", from, to)
			for i := from; i < to+1; i++ {
				resource, err := os.ReadFile(filepath.Join(workDir, fmt.Sprintf("bmhs/node_%d.yaml", i)))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(CreateOrUpdateWithNamespace(ctx, bootstrapClusterProxy, resource, namespace)).ShouldNot(HaveOccurred())
			}
			Logf("Wait for batch from node_%d to node_%d to become available", from, to)
			WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
				Client:    bootstrapClient,
				Options:   []client.ListOption{client.InNamespace(namespace)},
				Replicas:  to + 1,
				Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
			})
		}

		for i := 0; i < numNodes; i += batch {
			if i+batch > numNodes {
				applyBatchBmh(i, numNodes-1)
				break
			}
			applyBatchBmh(i, i+batch-1)
		}

		By("SCALABILITY TEST PASSED!")
	})
}
