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

// MachineStatusError defines errors states for Machine objects.
// These are duplicated from the deprecated sigs.k8s.io/cluster-api/api/deprecated/errors
// package to allow CAPM3 to eventually remove the dependency on that package.
type MachineStatusError string

const (
	// InvalidConfigurationMachineError represents that the combination
	// of configuration in the MachineSpec is not supported by this cluster.
	// This is not a transient error, but indicates a state that must be
	// fixed before progress can be made.
	InvalidConfigurationMachineError MachineStatusError = "InvalidConfiguration"

	// CreateMachineError indicates an error while trying to create a Node to match this
	// Machine. This may indicate a transient problem that will be fixed
	// automatically with time, such as a service outage, or a terminal
	// error during creation that doesn't match a more specific
	// MachineStatusError value.
	CreateMachineError MachineStatusError = "CreateError"

	// UpdateMachineError indicates an error while trying to update a Node that this
	// Machine represents. This may indicate a transient problem that will be
	// fixed automatically with time, such as a service outage.
	UpdateMachineError MachineStatusError = "UpdateError"

	// DeleteMachineError indicates an error was encountered while trying to delete the Node
	// that this Machine represents. This could be a transient or terminal error, but
	// will only be observable if the provider's Machine controller has
	// added a finalizer to the object to more gracefully handle deletions.
	DeleteMachineError MachineStatusError = "DeleteError"
)

// ClusterStatusError defines errors states for Cluster objects.
// These are duplicated from the deprecated sigs.k8s.io/cluster-api/api/deprecated/errors
// package to allow CAPM3 to eventually remove the dependency on that package.
type ClusterStatusError string

const (
	// InvalidConfigurationClusterError indicates that the cluster
	// configuration is invalid.
	InvalidConfigurationClusterError ClusterStatusError = "InvalidConfiguration"

	// CreateClusterError indicates that an error was encountered
	// when trying to create the cluster.
	CreateClusterError ClusterStatusError = "CreateError"

	// UpdateClusterError indicates that an error was encountered
	// when trying to update the cluster.
	UpdateClusterError ClusterStatusError = "UpdateError"

	// DeleteClusterError indicates that an error was encountered
	// when trying to delete the cluster.
	DeleteClusterError ClusterStatusError = "DeleteError"
)
