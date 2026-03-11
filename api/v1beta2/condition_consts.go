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

package v1beta2

import clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

// Metal3Cluster Conditions and Reasons.
const (
	// BaremetalInfrastructureReadyV1Beta1Condition reports the current status of
	// cluster infrastructure. At the moment this is cosmetic since
	// Metal3Cluster does not contain any infra setup steps.
	BaremetalInfrastructureReadyV1Beta1Condition clusterv1.ConditionType = "BaremetalInfrastructureReady"
	// ControlPlaneEndpointFailedV1Beta1Reason is used to indicate that provided ControlPlaneEndpoint is invalid.
	ControlPlaneEndpointFailedV1Beta1Reason = "ControlPlaneEndpointFailed"
	// InternalFailureV1Beta1Reason is used to indicate that an internal failure
	// occurred. The `Message` field of the Condition should be consulted for
	// details on the failure.
	InternalFailureV1Beta1Reason = "InternalFailureOccured"
)

// Metal3Machine Conditions and Reasons.
const (
	// AssociateBMHV1Beta1Condition documents the status of associated the Metal3Machine with a BaremetalHost.
	AssociateBMHV1Beta1Condition clusterv1.ConditionType = "AssociateBMH"

	// WaitingForClusterInfrastructureV1Beta1Reason used when waiting for cluster
	// infrastructure to be ready before proceeding.
	WaitingForClusterInfrastructureV1Beta1Reason = "WaitingForClusterInfrastructure"
	// WaitingForBootstrapReadyV1Beta1Reason used when waiting for bootstrap to be ready before proceeding.
	WaitingForBootstrapReadyV1Beta1Reason = "WaitingForBootstrapReady"
	// AssociateBMHFailedV1Beta1Reason documents any errors while associating Metal3Machine with a BaremetalHost.
	AssociateBMHFailedV1Beta1Reason = "AssociateBMHFailed"
	// WaitingForMetal3MachineOwnerRefV1Beta1Reason is used when Metal3Machine is waiting for OwnerReference to be
	// set before proceeding.
	WaitingForMetal3MachineOwnerRefV1Beta1Reason = "WaitingForM3MachineOwnerRef"
	// Metal3MachinePausedV1Beta1Reason is used when Metal3Machine or Cluster is paused.
	Metal3MachinePausedV1Beta1Reason = "Metal3MachinePaused"
	// WaitingforMetal3ClusterV1Beta1Reason is used when Metal3Machine is waiting for Metal3Cluster.
	WaitingforMetal3ClusterV1Beta1Reason = "WaitingforMetal3Cluster"
	// PauseAnnotationRemoveFailedV1Beta1Reason is used when failed to remove/check pause annotation on associated bmh.
	PauseAnnotationRemoveFailedV1Beta1Reason = "PauseAnnotationRemoveFailed"
	// PauseAnnotationSetFailedV1Beta1Reason is used when failed to set pause annotation on associated bmh.
	PauseAnnotationSetFailedV1Beta1Reason = "PauseAnnotationSetFailedReason"

	// KubernetesNodeReadyV1Beta1Condition documents the transition of a Metal3Machine into a Kubernetes Node.
	KubernetesNodeReadyV1Beta1Condition clusterv1.ConditionType = "KubernetesNodeReady"
	// Could not find the BMH associated with the Metal3Machine.
	MissingBMHV1Beta1Reason = "MissingBMH"
	// Could not set the ProviderID on the target cluster's Node object.
	SettingProviderIDOnNodeFailedV1Beta1Reason = "SettingProviderIDOnNodeFailed"
	// Metal3DataReadyV1Beta1Condition reports a summary of Metal3Data status.
	Metal3DataReadyV1Beta1Condition clusterv1.ConditionType = "Metal3DataReady"
	// WaitingForMetal3DataV1Beta1Reason used when waiting for Metal3Data
	// to be ready before proceeding.
	WaitingForMetal3DataV1Beta1Reason = "WaitingForMetal3Data"
	// AssociateM3MetaDataFailedV1Beta1Reason is used when failed to associate Metadata to Metal3Machine.
	AssociateM3MetaDataFailedV1Beta1Reason = "AssociateM3MetaDataFailed"
	// DisassociateM3MetaDataFailedV1Beta1Reason is used when failed to remove OwnerReference of Meta3DataTemplate.
	DisassociateM3MetaDataFailedV1Beta1Reason = "DisassociateM3MetaDataFailed"
	// DeletingV1Beta1Reason (Severity=Info) documents a condition not in Status=True because the underlying object it is currently being deleted.
	DeletingV1Beta1Reason = "Deleting"
	// DeletionFailedV1Beta1Reason (Severity=Warning) documents a condition not in Status=True because the underlying object
	// encountered problems during deletion. This is a warning because the reconciler will retry deletion.
	DeletionFailedV1Beta1Reason = "DeletionFailed"
)
