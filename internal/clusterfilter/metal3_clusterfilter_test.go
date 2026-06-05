/*
 Copyright 2026 The Kubernetes Authors.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

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
