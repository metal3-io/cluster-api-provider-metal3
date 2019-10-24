/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha2

import (
	"testing"
)

func TestClusterSpecIsValid(t *testing.T) {
	cases := []struct {
		Spec          BareMetalClusterSpec
		ErrorExpected bool
		Name          string
	}{
		{
			Spec:          BareMetalClusterSpec{},
			ErrorExpected: true,
			Name:          "empty spec",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: ":/sadfc7:/:",
			},
			ErrorExpected: true,
			Name:          "Incorrect spec",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: "http://foo.bar:6443",
			},
			ErrorExpected: false,
			Name:          "Correct spec",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: "foo.bar:6443",
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no scheme",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: "foo.bar",
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no scheme or port",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: "http://foo.bar",
			},
			ErrorExpected: false,
			Name:          "Correct spec, no port",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: "//foo.bar",
			},
			ErrorExpected: false,
			Name:          "Correct spec, hostname only",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: "http://192.168.1.1:6443",
			},
			ErrorExpected: false,
			Name:          "Correct spec",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: "192.168.1.1:6443",
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no scheme",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: "192.168.1.1.bar",
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no scheme or port",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: "http://192.168.1.1",
			},
			ErrorExpected: false,
			Name:          "Correct spec, no port",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: "//192.168.1.1",
			},
			ErrorExpected: false,
			Name:          "Correct spec, hostname only",
		},
		{
			Spec: BareMetalClusterSpec{
				APIEndpoint: "",
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no endpoint",
		},
	}

	for _, tc := range cases {
		err := tc.Spec.IsValid()
		if tc.ErrorExpected && err == nil {
			t.Errorf("Did not get error from case \"%v\"", tc.Name)
		}
		if !tc.ErrorExpected && err != nil {
			t.Errorf("Got unexpected error from case \"%v\": %v", tc.Name, err)
		}
	}
}
