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
	"net/url"
	"strings"

	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	// UnhealthyAnnotation is the annotation that sets unhealthy status of BMH.
	UnhealthyAnnotation = "capi.metal3.io/unhealthy"

	LiveISODiskFormat = "live-iso"
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

	// Namespace where to look for BareMetalHosts. If empty, look for
	// BareMetalHosts in the namespace of Metal3Machine
	// +optional
	Namespace string `json:"namespace,omitempty"`
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

	// DiskFormat contains the image disk format.
	// +kubebuilder:validation:Enum=raw;qcow2;vdi;vmdk;live-iso
	// +optional
	DiskFormat *string `json:"format,omitempty"`
}

// Validate performs validation on [Image], returning a list of field errors using the provided base path.
// It is intended to be used in the validation webhooks of resources containing [Image].
func (i *Image) Validate(base field.Path) field.ErrorList {
	var errors field.ErrorList

	if i.URL == "" {
		errors = append(errors, field.Required(base.Child("URL"), "cannot be empty"))
	} else {
		_, err := url.ParseRequestURI(i.URL)
		if err != nil {
			errors = append(errors, field.Invalid(base.Child("URL"), i.URL, "not a valid URL"))
		}
	}
	// Checksum is not required for live-iso.
	if i.DiskFormat == nil || *i.DiskFormat != LiveISODiskFormat {
		if i.Checksum == "" {
			errors = append(errors, field.Required(base.Child("Checksum"), "cannot be empty"))
		}

		if strings.HasPrefix(i.Checksum, "http://") || strings.HasPrefix(i.Checksum, "https://") {
			_, err := url.ParseRequestURI(i.Checksum)
			if err != nil {
				errors = append(errors, field.Invalid(base.Child("Checksum"), i.Checksum, "not a valid URL"))
			}
		}
	}
	return errors
}
