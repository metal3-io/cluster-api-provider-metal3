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

package clusterfilter

import (
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// IsMetal3Cluster returns true if the Cluster's InfrastructureRef points to a Metal3Cluster.
func IsMetal3Cluster(cluster *clusterv1.Cluster) bool {
	if cluster == nil {
		return false
	}

	infra := cluster.Spec.InfrastructureRef

	return infra.Kind == "Metal3Cluster" &&
		infra.APIGroup == infrav1.GroupVersion.Group
}
