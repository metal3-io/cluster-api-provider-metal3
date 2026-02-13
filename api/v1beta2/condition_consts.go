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
	// BaremetalInfrastructureReadyCondition reports the current status of
	// cluster infrastructure. At the moment this is cosmetic since
	// Metal3Cluster does not contain any infra setup steps.
	BaremetalInfrastructureReadyCondition clusterv1.ConditionType = "BaremetalInfrastructureReady"
	// ControlPlaneEndpointFailedReason is used to indicate that provided ControlPlaneEndpoint is invalid.
	ControlPlaneEndpointFailedReason = "ControlPlaneEndpointFailed"
	// InternalFailureReason is used to indicate that an internal failure
	// occurred. The `Message` field of the Condition should be consulted for
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
	// WaitingForMetal3MachineOwnerRefReason is used when Metal3Machine is waiting for OwnerReference to be
	// set before proceeding.
	WaitingForMetal3MachineOwnerRefReason = "WaitingForM3MachineOwnerRef"
	// Metal3MachinePausedReason is used when Metal3Machine or Cluster is paused.
	Metal3MachinePausedReason = "Metal3MachinePaused"
	// WaitingforMetal3ClusterReason is used when Metal3Machine is waiting for Metal3Cluster.
	WaitingforMetal3ClusterReason = "WaitingforMetal3Cluster"
	// PauseAnnotationRemoveFailedReason is used when failed to remove/check pause annotation on associated bmh.
	PauseAnnotationRemoveFailedReason = "PauseAnnotationRemoveFailed"
	// PauseAnnotationSetFailedReason is used when failed to set pause annotation on associated bmh.
	PauseAnnotationSetFailedReason = "PauseAnnotationSetFailedReason"

	// KubernetesNodeReadyCondition documents the transition of a Metal3Machine into a Kubernetes Node.
	KubernetesNodeReadyCondition clusterv1.ConditionType = "KubernetesNodeReady"
	// Could not find the BMH associated with the Metal3Machine.
	MissingBMHReason = "MissingBMH"
	// Could not set the ProviderID on the target cluster's Node object.
	SettingProviderIDOnNodeFailedReason = "SettingProviderIDOnNodeFailed"
	// Metal3DataReadyCondition reports a summary of Metal3Data status.
	Metal3DataReadyCondition clusterv1.ConditionType = "Metal3DataReady"
	// WaitingForMetal3DataReason used when waiting for Metal3Data
	// to be ready before proceeding.
	WaitingForMetal3DataReason = "WaitingForMetal3Data"
	// AssociateM3MetaDataFailedReason is used when failed to associate Metadata to Metal3Machine.
	AssociateM3MetaDataFailedReason = "AssociateM3MetaDataFailed"
	// DisassociateM3MetaDataFailedReason is used when failed to remove OwnerReference of Meta3DataTemplate.
	DisassociateM3MetaDataFailedReason = "DisassociateM3MetaDataFailed"
	// DeletingReason (Severity=Info) documents a condition not in Status=True because the underlying object it is currently being deleted.
	DeletingReason = "Deleting"
	// DeletionFailedReason (Severity=Warning) documents a condition not in Status=True because the underlying object
	// encountered problems during deletion. This is a warning because the reconciler will retry deletion.
	DeletionFailedReason = "DeletionFailed"
)
