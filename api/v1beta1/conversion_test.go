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

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())

	t.Run("for Metal3Cluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3Cluster{},
		Spoke:       &Metal3Cluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for Metal3ClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3ClusterTemplate{},
		Spoke:       &Metal3ClusterTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for Metal3Machine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3Machine{},
		Spoke:       &Metal3Machine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for Metal3MachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3MachineTemplate{},
		Spoke:       &Metal3MachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for Metal3DataTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3DataTemplate{},
		Spoke:       &Metal3DataTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for Metal3Data", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3Data{},
		Spoke:       &Metal3Data{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for Metal3DataClaim", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3DataClaim{},
		Spoke:       &Metal3DataClaim{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for Metal3Remediation", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3Remediation{},
		Spoke:       &Metal3Remediation{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for Metal3RemediationTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3RemediationTemplate{},
		Spoke:       &Metal3RemediationTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
}
