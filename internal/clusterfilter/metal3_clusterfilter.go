package clusterfilter

import (
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// Checks if cluster belongs to Metal3.
func IsMetal3Cluster(cluster *clusterv1.Cluster) bool {
	if cluster == nil {
		return false
	}

	infra := cluster.Spec.InfrastructureRef

	return infra.Kind == "Metal3Cluster" &&
		infra.APIGroup == infrav1.GroupVersion.Group
}
