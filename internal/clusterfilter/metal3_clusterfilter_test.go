package clusterfilter_test

import (
	"testing"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	"github.com/metal3-io/cluster-api-provider-metal3/internal/clusterfilter"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestIsMetal3Cluster(t *testing.T) {
	tests := []struct {
		name    string
		cluster *clusterv1.Cluster
		want    bool
	}{
		{
			name:    "nil cluster",
			cluster: nil,
			want:    false,
		},
		{
			name: "metal3 infrastructure ref",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind:     "Metal3Cluster",
						APIGroup: infrav1.GroupVersion.Group,
					},
				},
			},
			want: true,
		},
		{
			name: "wrong api group",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind:     "Metal3Cluster",
						APIGroup: "infrastructure.example.io",
					},
				},
			},
			want: false,
		},
		{
			name: "wrong kind",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind:     "OtherCluster",
						APIGroup: infrav1.GroupVersion.Group,
					},
				},
			},
			want: false,
		},
		{
			name: "empty infrastructure ref",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clusterfilter.IsMetal3Cluster(tt.cluster); got != tt.want {
				t.Fatalf("clusterfilter.IsMetal3Cluster() = %v, want %v", got, tt.want)
			}
		})
	}
}
