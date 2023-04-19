package e2e

import (
	"context"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

type IPReuseInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
}

func IPReuse(ctx context.Context, inputGetter func() IPReuseInput) {
	Logf("Starting IP reuse tests")
	input := inputGetter()
	targetClusterClient := input.TargetCluster.GetClient()
	baremetalv4Pool, provisioningPool := GetIPPools(ctx, targetClusterClient, input.ClusterName, input.Namespace)
	Expect(baremetalv4Pool).To(HaveLen(1))
	Expect(provisioningPool).To(HaveLen(1))
}
