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
				ControlPlaneEndpoint: APIEndpoint{
					Host: "foo.bar",
					Port: 6443,
				},
			},
			ErrorExpected: false,
			Name:          "Correct spec",
		},
		{
			Spec: Metal3ClusterSpec{
				ControlPlaneEndpoint: APIEndpoint{
					Host: "",
					Port: 0,
				},
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no host and port",
		},
		{
			Spec: Metal3ClusterSpec{
				ControlPlaneEndpoint: APIEndpoint{
					Host: "",
					Port: 6443,
				},
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no host",
		},
		{
			Spec: Metal3ClusterSpec{
				ControlPlaneEndpoint: APIEndpoint{
					Host: "foo.bar",
					Port: 0,
				},
			},
			ErrorExpected: true,
			Name:          "Incorrect spec, no port",
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
