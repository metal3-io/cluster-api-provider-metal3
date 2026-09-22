/*
Copyright 2025 The Metal3 Authors.

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

package baremetal

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// PrefixAnnotationKey is the annotation key on Metal3Cluster that lists the label prefixes to sync.
	PrefixAnnotationKey = "metal3.io/metal3-label-sync-prefixes"
)

// reservedLabelDomains are Kubernetes-reserved label domains that must never be used as sync prefixes.
var reservedLabelDomains = []string{
	"kubernetes.io",
	"k8s.io",
}

// IsReservedLabelPrefix reports whether the given label prefix belongs to a
// Kubernetes-reserved domain (kubernetes.io, k8s.io) or any of their subdomains.
func IsReservedLabelPrefix(prefix string) bool {
	for _, domain := range reservedLabelDomains {
		if prefix == domain || strings.HasSuffix(prefix, "."+domain) {
			return true
		}
	}
	return false
}

// ParsePrefixAnnotation parses a comma-separated list of DNS-1123 subdomain prefixes.
func ParsePrefixAnnotation(prefixStr string) (map[string]struct{}, error) {
	entries := strings.Split(prefixStr, ",")
	prefixSet := make(map[string]struct{})
	for _, prefix := range entries {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			// ignore empty prefix string (e.g. `, ,`)
			continue
		} else if errs := validation.IsDNS1123Subdomain(prefix); len(errs) > 0 {
			return nil, fmt.Errorf("invalid prefix (%v): %s", prefix, strings.Join(errs, ", "))
		}
		prefixSet[prefix] = struct{}{}
	}
	return prefixSet, nil
}
