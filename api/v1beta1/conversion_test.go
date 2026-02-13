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
	"reflect"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/randfill"

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
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3ClusterFuzzFuncs},
	}))
	t.Run("for Metal3ClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3ClusterTemplate{},
		Spoke:       &Metal3ClusterTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3ClusterTemplateFuzzFuncs},
	}))
	t.Run("for Metal3Machine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.Metal3Machine{},
		Spoke:       &Metal3Machine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{Metal3MachineFuzzFuncs},
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

func Metal3ClusterFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubMetal3ClusterStatus,
		hubMetal3FailureDomain,
		spokeMetal3ClusterStatus,
	}
}

func hubMetal3ClusterStatus(in *infrav1.Metal3ClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta1 == nil || reflect.DeepEqual(in.Deprecated.V1Beta1, &infrav1.Metal3ClusterV1Beta1DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func hubMetal3FailureDomain(in *clusterv1.FailureDomain, c randfill.Continue) {
	c.FillNoCustom(in)

	// Normalize ControlPlane: &false to nil (omitempty semantic)
	if in.ControlPlane != nil && !*in.ControlPlane {
		in.ControlPlane = nil
	}
}

func spokeMetal3ClusterStatus(in *Metal3ClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &Metal3ClusterV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}

func Metal3MachineFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubMetal3MachineStatus,
		spokeMetal3MachineSpec,
		spokeMetal3MachineStatus,
	}
}

func hubMetal3MachineStatus(in *infrav1.Metal3MachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.Deprecated != nil {
		if in.Deprecated.V1Beta1 == nil || reflect.DeepEqual(in.Deprecated.V1Beta1, &infrav1.Metal3MachineV1Beta1DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}


func spokeMetal3MachineSpec(in *Metal3MachineSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.ProviderID != nil && *in.ProviderID == "" {
		in.ProviderID = nil
	}
}

func spokeMetal3MachineStatus(in *Metal3MachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if len(in.V1Beta2.Conditions) == 0 || reflect.DeepEqual(in.V1Beta2, &Metal3MachineV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}

func Metal3ClusterTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubMetal3FailureDomain,
	}
}
