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

import (
	"net/url"
	"strings"

	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	// UnhealthyAnnotationDeprecated is deprecated in favor of UnhealthyAnnotation and kept for migration period.
	UnhealthyAnnotationDeprecated = "capi.metal3.io/unhealthy"

	// UnhealthyAnnotation is the annotation that sets unhealthy status of BMH.
	UnhealthyAnnotation = "capm3.metal3.io/unhealthy"

	// liveISODiskFormat is the disk format for live-iso images.
	LiveISODiskFormat = "live-iso"
)

// APIEndpoint represents a reachable Kubernetes API endpoint.
// +kubebuilder:validation:MinProperties=1
type APIEndpoint struct {
	// host is the hostname on which the API server is serving.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	Host string `json:"host,omitempty"`

	// port is the port on which the API server is serving.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

// HostSelector specifies matching criteria for labels on BareMetalHosts.
// This is used to limit the set of BareMetalHost objects considered for
// claiming for a Machine.
type HostSelector struct {
	// matchLabels specifies key/value pairs of labels that must exist on a chosen BareMetalHost
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// matchExpressions specifies match expressions that must be true on a chosen BareMetalHost
	// +optional
	MatchExpressions []HostSelectorRequirement `json:"matchExpressions,omitempty"`
}

type HostSelectorRequirement struct {
	// key is the label key that the selector applies to.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	Key string `json:"key,omitempty"`

	// operator represents a key's relationship to a set of values.
	// +required
	Operator selection.Operator `json:"operator,omitempty"`

	// values is an array of string of required values.
	// +required
	// +kubebuilder:validation:MaxItems=512
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=512
	Values []string `json:"values,omitempty"`
}

// Image holds the details of an image to use during provisioning.
type Image struct {
	// url is a location of an image to deploy.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:Format=uri
	URL string `json:"url,omitempty"`

	// checksum is a md5sum, sha256sum or sha512sum value or a URL to retrieve one.
	// When diskFormat is live-iso, empty string is also a valid value.
	// +required
	// +kubebuilder:validation:MinLength=0
	// +kubebuilder:validation:MaxLength=512
	Checksum *string `json:"checksum,omitempty"`

	// checksumType is the checksum algorithm for the image.
	// e.g md5, sha256, sha512
	// +kubebuilder:validation:Enum=md5;sha256;sha512
	// +optional
	ChecksumType string `json:"checksumType,omitempty"`

	// diskFormat contains the image disk format.
	// +kubebuilder:validation:Enum=raw;qcow2;vdi;vmdk;live-iso
	// +optional
	DiskFormat string `json:"diskFormat,omitempty"`
}

// Custom deploy is a description of a customized deploy process.
type CustomDeploy struct {
	// method is the name of the deploy method.
	// This name is specific to the deploy ramdisk used. If you don't have
	// a custom deploy ramdisk, you shouldn't use CustomDeploy.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	Method string `json:"method,omitempty"`
}

// Metal3ObjectRef is a reference to a Metal3 resource by name and namespace.
// +structType=atomic
type Metal3ObjectRef struct {
	// name of the resource.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name,omitempty"`

	// namespace of the resource.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Namespace string `json:"namespace,omitempty"`
}

// Validate performs validation on [Image], returning a list of field errors using the provided base path.
// It is intended to be used in the validation webhooks of resources containing [Image].
func (i *Image) Validate(base field.Path) field.ErrorList {
	var errors field.ErrorList

	if i.URL == "" && i.Checksum == nil {
		errors = append(errors, field.Required(&base, "either image or customDeploy is required"))
		return errors // not possible to validate further
	}

	if i.URL == "" {
		errors = append(errors, field.Required(base.Child("URL"), "cannot be empty"))
	} else {
		_, err := url.ParseRequestURI(i.URL)
		if err != nil {
			errors = append(errors, field.Invalid(base.Child("URL"), i.URL, "not a valid URL"))
		}
	}
	// Checksum is not required for live-iso.
	if i.DiskFormat != LiveISODiskFormat {
		if i.Checksum == nil || *i.Checksum == "" {
			errors = append(errors, field.Required(base.Child("Checksum"), "cannot be empty"))
		} else if strings.HasPrefix(*i.Checksum, "http://") || strings.HasPrefix(*i.Checksum, "https://") {
			_, err := url.ParseRequestURI(*i.Checksum)
			if err != nil {
				errors = append(errors, field.Invalid(base.Child("Checksum"), *i.Checksum, "not a valid URL"))
			}
		}
	}
	return errors
}
