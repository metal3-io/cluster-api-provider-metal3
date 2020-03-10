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

package v1alpha3

import (
	"math/rand"
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	. "github.com/onsi/gomega"
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
		func(i *v1alpha4.APIEndpoint, c fuzz.Continue) {
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
	g.Expect(v1alpha4.AddToScheme(scheme)).To(Succeed())

	t.Run("for Metal3Cluster", utilconversion.FuzzTestFunc(scheme, &v1alpha4.Metal3Cluster{}, &Metal3Cluster{}, apiEndpointFuzzerFuncs))
	t.Run("for Metal3Machine", utilconversion.FuzzTestFunc(scheme, &v1alpha4.Metal3Machine{}, &Metal3Machine{}))
	t.Run("for Metal3Machine", utilconversion.FuzzTestFunc(scheme, &v1alpha4.Metal3MachineTemplate{}, &Metal3MachineTemplate{}))
}
