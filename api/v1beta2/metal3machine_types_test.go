/*
Copyright 2026 The Kubernetes Authors.

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

package v1beta2

import (
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestStorageMetal3MachineSpec(t *testing.T) {
	key := types.NamespacedName{
		Name:      "foo",
		Namespace: "default",
	}

	created := &Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: Metal3MachineSpec{
			UserData: &corev1.SecretReference{
				Name: "foo",
			},
		},
	}

	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &Metal3Machine{}
	g.Expect(c.Create(t.Context(), created)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(t.Context(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(t.Context(), updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(t.Context(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(t.Context(), fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(t.Context(), key, fetched)).To(gomega.HaveOccurred())
}
