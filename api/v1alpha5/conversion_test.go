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

package v1alpha5

import (
	"math/rand"
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func apiEndpointFuzzerFuncs(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(i *v1beta1.APIEndpoint, c fuzz.Continue) {
			b := make([]byte, seededRand.Intn(264))
			for i := range b {
				b[i] = charset[seededRand.Intn(len(charset))]
			}
			i.Host = string(b)
			i.Port = seededRand.Intn(65535)
		},
	}
}

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1beta1.AddToScheme(scheme)).To(Succeed())

	t.Run("for Metal3Cluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta1.Metal3Cluster{},
		Spoke:       &Metal3Cluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{apiEndpointFuzzerFuncs},
	}))

	t.Run("for Metal3Machine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &v1beta1.Metal3Machine{},
		Spoke:  &Metal3Machine{},
	}))

	t.Run("for Metal3MachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &v1beta1.Metal3MachineTemplate{},
		Spoke:  &Metal3MachineTemplate{},
	}))

	t.Run("for Metal3Data", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &v1beta1.Metal3Data{},
		Spoke:  &Metal3Data{},
	}))

	t.Run("for Metal3DataTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &v1beta1.Metal3DataTemplate{},
		Spoke:  &Metal3DataTemplate{},
	}))

	t.Run("for Metal3DataClaim", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &v1beta1.Metal3DataClaim{},
		Spoke:  &Metal3DataClaim{},
	}))
}
