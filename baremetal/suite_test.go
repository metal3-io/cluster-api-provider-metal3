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

package baremetal

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	_ "github.com/go-logr/logr"
	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

const (
	clusterName       = "testCluster"
	metal3ClusterName = "testmetal3Cluster"
	namespaceName     = "baremetalns"
	m3muid            = "11111111-9845-4321-1234-c74be387f57c"
	bmhuid            = "22222222-9845-4c48-9e49-c74be387f57c"
)

var providerid = fmt.Sprintf("%s/%s/%s", namespaceName, bmhuid, m3muid)

func TestManagers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Manager Suite")
}

var _ = BeforeSuite(func() {
	done := make(chan interface{})

	go func() {
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
		By("bootstrapping test environment")
		testEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
		}

		var err error
		cfg, err = testEnv.Start()
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg).ToNot(BeNil())

		err = capm3.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		err = ipamv1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		err = apiextensionsv1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		// +kubebuilder:scaffold:scheme

		k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient).ToNot(BeNil())
		err = k8sClient.Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
		})
		Expect(err).NotTo(HaveOccurred())

		close(done)
	}()
	Eventually(done, 60).Should(BeClosed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

var bmcOwnerRef = &metav1.OwnerReference{
	APIVersion: clusterv1.GroupVersion.String(),
	Kind:       "Cluster",
	Name:       clusterName,
}

/*-----------------------------------
---------Helper functions------------
------------------------------------*/

func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := clusterv1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := capm3.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := ipamv1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := bmh.SchemeBuilder.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
}

func newCluster(clusterName string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespaceName,
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				Name:       metal3ClusterName,
				Namespace:  namespaceName,
				Kind:       "InfrastructureConfig",
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			},
		},
		Status: clusterv1.ClusterStatus{
			InfrastructureReady: true,
		},
	}
}

func newMetal3Cluster(baremetalName string, ownerRef *metav1.OwnerReference,
	spec *capm3.Metal3ClusterSpec, status *capm3.Metal3ClusterStatus) *capm3.Metal3Cluster {
	if spec == nil {
		spec = &capm3.Metal3ClusterSpec{}
	}
	if status == nil {
		status = &capm3.Metal3ClusterStatus{}
	}
	ownerRefs := []metav1.OwnerReference{}
	if ownerRef != nil {
		ownerRefs = []metav1.OwnerReference{*ownerRef}
	}

	return &capm3.Metal3Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Metal3Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            metal3ClusterName,
			Namespace:       namespaceName,
			OwnerReferences: ownerRefs,
		},
		Spec:   *spec,
		Status: *status,
	}
}
