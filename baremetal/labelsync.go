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
	"errors"
	"fmt"
	"regexp"
	"strings"
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
		} else if err := isDNS1123Subdomain(prefix); err != nil {
			return nil, fmt.Errorf("invalid prefix (%v): %w", prefix, err)
		}
		prefixSet[prefix] = struct{}{}
	}
	return prefixSet, nil
}

// The following code mirrors kubectl label/prefix validation.
// Reference: https://github.com/kubernetes/apimachinery/blob/master/pkg/util/validation/validation.go
const dns1123LabelFmt string = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"
const dns1123SubdomainFmt string = dns1123LabelFmt + "(\\." + dns1123LabelFmt + ")*"
const dns1123SubdomainErrorMsg string = "a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"

// dns1123SubdomainMaxLength is a subdomain's max length in DNS (RFC 1123).
const dns1123SubdomainMaxLength int = 253

var dns1123SubdomainRegexp = regexp.MustCompile("^" + dns1123SubdomainFmt + "$")

// isDNS1123Subdomain tests for a string that conforms to the definition of a
// subdomain in DNS (RFC 1123).
func isDNS1123Subdomain(value string) error {
	if len(value) > dns1123SubdomainMaxLength {
		return fmt.Errorf("%v must be no more than %d characters", value, dns1123SubdomainMaxLength)
	}
	if !dns1123SubdomainRegexp.MatchString(value) {
		return errors.New(regexError(dns1123SubdomainErrorMsg, dns1123SubdomainFmt, "example.com"))
	}
	return nil
}

// regexError returns a string explanation of a regex validation failure.
func regexError(msg string, format string, examples ...string) string {
	if len(examples) == 0 {
		return msg + " (regex used for validation is '" + format + "')"
	}
	return msg + " (e.g. '" + strings.Join(examples, "' or '") + "', regex used for validation is '" + format + "')"
}
