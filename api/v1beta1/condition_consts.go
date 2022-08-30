/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

// Metal3Cluster Conditions and Reasons.
const (
	// BaremetalInfrastructureReadyCondition reports the current status of
	// cluster infrastructure. At the moment this is cosmetic since
	// Metal3Cluster does not contain any infra setup steps.
	BaremetalInfrastructureReadyCondition clusterv1.ConditionType = "BaremetalInfrastructureReady"
	// ControlPlaneEndpointFailedReason is used to indicate that provided ControlPlaneEndpoint is invalid.
	ControlPlaneEndpointFailedReason = "ControlPlaneEndpointFailed"
	// InternalFailureReason is used to indicate that an internal failure
	// occurred. The `Message` field of the Condition should be consluted for
	// details on the failure.
	InternalFailureReason = "InternalFailureOccured"
)

// Metal3Machine Conditions and Reasons.
const (
	// AssociateBMHCondition documents the status of associated the Metal3Machine with a BaremetalHost.
	AssociateBMHCondition clusterv1.ConditionType = "AssociateBMH"

	// WaitingForClusterInfrastructureReason used when waiting for cluster
	// infrastructure to be ready before proceeding.
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	// WaitingForBootstrapReadyReason used when waiting for bootstrap to be ready before proceeding.
	WaitingForBootstrapReadyReason = "WaitingForBootstrapReady"
	// AssociateBMHFailedReason documents any errors while associating Metal3Machine with a BaremetalHost.
	AssociateBMHFailedReason = "AssociateBMHFailed"

	// KubernetesNodeReadyCondition documents the transition of a Metal3Machine into a Kubernetes Node.
	KubernetesNodeReadyCondition clusterv1.ConditionType = "KubernetesNodeReady"
	// Could not find the BMH associated with the Metal3Machine.
	MissingBMHReason = "MissingBMH"
	// Could not set the ProviderID on the target cluster's Node object.
	SettingProviderIDOnNodeFailedReason = "SettingProviderIDOnNodeFailed"
)
