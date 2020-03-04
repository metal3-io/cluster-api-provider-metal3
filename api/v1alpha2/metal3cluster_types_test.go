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
		Spec          Metal3ClusterSpec
		ErrorExpected bool
		Name          string
	}{
		{
			Spec:          Metal3ClusterSpec{},
			ErrorExpected: true,
			Name:          "empty spec",
		},
		{
			Spec: Metal3ClusterSpec{
				APIEndpoint: ":/sadfc7:/:",
			},
			ErrorExpected: true,
			Name:          "Incorrect spec",
		},
		{
			Spec: Metal3ClusterSpec{
				APIEndpoint: "http://foo.bar:6443",
			},
			ErrorExpected: false,
			Name:          "Correct spec",
		},
		{
			Spec: Metal3ClusterSpec{
				APIEndpoint: "foo.bar:6443",
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no scheme",
		},
		{
			Spec: Metal3ClusterSpec{
				APIEndpoint: "foo.bar",
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no scheme or port",
		},
		{
			Spec: Metal3ClusterSpec{
				APIEndpoint: "http://foo.bar",
			},
			ErrorExpected: false,
			Name:          "Correct spec, no port",
		},
		{
			Spec: Metal3ClusterSpec{
				APIEndpoint: "//foo.bar",
			},
			ErrorExpected: false,
			Name:          "Correct spec, hostname only",
		},
		{
			Spec: Metal3ClusterSpec{
				APIEndpoint: "http://192.168.1.1:6443",
			},
			ErrorExpected: false,
			Name:          "Correct spec",
		},
		{
			Spec: Metal3ClusterSpec{
				APIEndpoint: "192.168.1.1:6443",
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no scheme",
		},
		{
			Spec: Metal3ClusterSpec{
				APIEndpoint: "192.168.1.1.bar",
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no scheme or port",
		},
		{
			Spec: Metal3ClusterSpec{
				APIEndpoint: "http://192.168.1.1",
			},
			ErrorExpected: false,
			Name:          "Correct spec, no port",
		},
		{
			Spec: Metal3ClusterSpec{
				APIEndpoint: "//192.168.1.1",
			},
			ErrorExpected: false,
			Name:          "Correct spec, hostname only",
		},
		{
			Spec: Metal3ClusterSpec{
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
