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

import (
	"k8s.io/apimachinery/pkg/selection"
)

const (
	// UnhealthyAnnotation is the annotation that sets unhealthy status of BMH
	UnhealthyAnnotation = "capi.metal3.io/unhealthy"
)

// APIEndpoint represents a reachable Kubernetes API endpoint.
type APIEndpoint struct {
	// Host is the hostname on which the API server is serving.
	Host string `json:"host"`

	// Port is the port on which the API server is serving.
	Port int `json:"port"`
}

// HostSelector specifies matching criteria for labels on BareMetalHosts.
// This is used to limit the set of BareMetalHost objects considered for
// claiming for a Machine.
type HostSelector struct {
	// Key/value pairs of labels that must exist on a chosen BareMetalHost
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// Label match expressions that must be true on a chosen BareMetalHost
	// +optional
	MatchExpressions []HostSelectorRequirement `json:"matchExpressions,omitempty"`
}

type HostSelectorRequirement struct {
	Key      string             `json:"key"`
	Operator selection.Operator `json:"operator"`
	Values   []string           `json:"values"`
}

// Image holds the details of an image to use during provisioning.
type Image struct {
	// URL is a location of an image to deploy.
	URL string `json:"url"`

	// Checksum is a md5sum value or a URL to retrieve one.
	Checksum string `json:"checksum"`

	// ChecksumType is the checksum algorithm for the image.
	// e.g md5, sha256, sha512
	// +kubebuilder:validation:Enum=md5;sha256;sha512
	// +optional
	ChecksumType *string `json:"checksumType,omitempty"`

	//DiskFormat contains the image disk format
	// +kubebuilder:validation:Enum=raw;qcow2;vdi;vmdk;live-iso
	// +optional
	DiskFormat *string `json:"format,omitempty"`
}
