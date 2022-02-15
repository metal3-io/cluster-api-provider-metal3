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

package remote

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Metal3 baremetal remote", func() {
	Describe("NewClusterClient", func() {

		var (
			clusterWithValidKubeConfig   *clusterv1.Cluster
			clusterWithInvalidKubeConfig *clusterv1.Cluster
			clusterWithNoKubeConfig      *clusterv1.Cluster
			validKubeConfig              string
			validSecret                  *corev1.Secret
			invalidSecret                *corev1.Secret
		)
		BeforeEach(func() {
			clusterWithValidKubeConfig = &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
				},
			}
			clusterWithInvalidKubeConfig = &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test2-invalid",
					Namespace: "test",
				},
			}
			clusterWithNoKubeConfig = &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test3",
					Namespace: "test",
				},
			}
			validKubeConfig = `
clusters:
- cluster:
    server: https://test-cluster-api:6443
  name: test-cluster-api
contexts:
- context:
    cluster: test-cluster-api
    user: kubernetes-admin
  name: kubernetes-admin@test-cluster-api
current-context: kubernetes-admin@test-cluster-api
kind: Config
preferences: {}
users:
- name: kubernetes-admin
`
			validSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1-kubeconfig",
					Namespace: "test",
				},
				Data: map[string][]byte{
					secret.KubeconfigDataName: []byte(validKubeConfig),
				},
			}
			invalidSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test2-invalid-kubeconfig",
					Namespace: "test",
				},
				Data: map[string][]byte{
					secret.KubeconfigDataName: []byte("Not valid!!1"),
				},
			}
		})

		It("should create a non-nil client when given valid kubeconfig", func() {
			client := fake.NewClientBuilder().WithRuntimeObjects(validSecret).Build()
			c, err := NewClusterClient(context.TODO(), client, clusterWithValidKubeConfig)
			Expect(err).To(BeNil())
			Expect(c).To(Not(BeNil()))
		})
		It("should error with not found message with no cluster kubeconfig", func() {
			client := fake.NewClientBuilder().Build()
			_, err := NewClusterClient(context.TODO(), client, clusterWithNoKubeConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
		It("should error with some other error with invalid kubeconfig", func() {
			client := fake.NewClientBuilder().WithRuntimeObjects(invalidSecret).Build()
			_, err := NewClusterClient(context.TODO(), client, clusterWithInvalidKubeConfig)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeFalse())
		})
	})
})
