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
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Label sync helpers", func() {
	type testCaseIsReservedLabelPrefix struct {
		Prefix   string
		Expected bool
	}

	DescribeTable("IsReservedLabelPrefix",
		func(tc testCaseIsReservedLabelPrefix) {
			Expect(IsReservedLabelPrefix(tc.Prefix)).To(Equal(tc.Expected))
		},
		Entry("kubernetes.io is reserved", testCaseIsReservedLabelPrefix{
			Prefix:   "kubernetes.io",
			Expected: true,
		}),
		Entry("k8s.io is reserved", testCaseIsReservedLabelPrefix{
			Prefix:   "k8s.io",
			Expected: true,
		}),
		Entry("subdomain of kubernetes.io is reserved", testCaseIsReservedLabelPrefix{
			Prefix:   "node.kubernetes.io",
			Expected: true,
		}),
		Entry("subdomain of k8s.io is reserved", testCaseIsReservedLabelPrefix{
			Prefix:   "foo.k8s.io",
			Expected: true,
		}),
		Entry("metal3.io is not reserved", testCaseIsReservedLabelPrefix{
			Prefix:   "foo.metal3.io",
			Expected: false,
		}),
		Entry("lookalike suffix is not reserved", testCaseIsReservedLabelPrefix{
			Prefix:   "notkubernetes.io",
			Expected: false,
		}),
		Entry("empty prefix is not reserved", testCaseIsReservedLabelPrefix{
			Prefix:   "",
			Expected: false,
		}),
	)

	type testCaseParsePrefixAnnotation struct {
		PrefixStr      string
		ExpectedErr    bool
		ExpectedResult map[string]struct{}
	}

	DescribeTable("ParsePrefixAnnotation",
		func(tc testCaseParsePrefixAnnotation) {
			prefixSet, err := ParsePrefixAnnotation(tc.PrefixStr)
			if tc.ExpectedErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(reflect.DeepEqual(prefixSet, tc.ExpectedResult)).To(BeTrue(), "Expected %v but got %v", tc.ExpectedResult, prefixSet)
			}
		},
		Entry("single prefix", testCaseParsePrefixAnnotation{
			PrefixStr:   "foo.metal3.io",
			ExpectedErr: false,
			ExpectedResult: map[string]struct{}{
				"foo.metal3.io": {},
			},
		}),
		Entry("multiple prefixes with whitespace and empties", testCaseParsePrefixAnnotation{
			PrefixStr:   "foo.metal3.io, moo.myprefix,,bar",
			ExpectedErr: false,
			ExpectedResult: map[string]struct{}{
				"foo.metal3.io": {},
				"moo.myprefix":  {},
				"bar":           {},
			},
		}),
		Entry("empty string", testCaseParsePrefixAnnotation{
			PrefixStr:      "",
			ExpectedErr:    false,
			ExpectedResult: map[string]struct{}{},
		}),
		Entry("only commas", testCaseParsePrefixAnnotation{
			PrefixStr:      ",, ,,",
			ExpectedErr:    false,
			ExpectedResult: map[string]struct{}{},
		}),
		Entry("invalid DNS-1123 prefix", testCaseParsePrefixAnnotation{
			PrefixStr:      "foo.io, @bar.io",
			ExpectedErr:    true,
			ExpectedResult: nil,
		}),
	)
})
