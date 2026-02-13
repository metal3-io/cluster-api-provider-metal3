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
	"reflect"
	"testing"

	_ "github.com/go-logr/logr"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capipamv1beta1 "sigs.k8s.io/cluster-api/api/ipam/v1beta1"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

const (
	clusterName            = "baremetal-testcluster"
	machineName            = "baremetal-testmachine"
	metal3ClusterName      = "baremetal-testmetal3Cluster"
	metal3machineName      = "baremetal-testmetal3machine"
	baremetalhostName      = "baremetal-testbaremetalhost"
	metal3DataTemplateName = "baremetal-testmetal3datatemplate"
	testPoolName           = "baremetal-testpoolname"
	metal3DataName         = "baremetal-testmetal3dataname"
	metal3DataClaimName    = "baremetal-testmetal3dataclaim"
	namespaceName          = "baremetalns-testns"
	muid                   = "902b9bf0-42c2-42ef-8315-ab23f07e009a"
	m3muid                 = "11111111-9845-4321-1234-c74be387f57c"
	bmhuid                 = "22222222-9845-4c48-9e49-c74be387f57c"
	m3duid                 = "4f3223fb-1ac1-482c-a4d4-e09f8e6c08f1"
	m3dcuid                = "d184c4f7-2a64-4537-bf74-f6abd08cb992"
	m3dtuid                = "9c8facc6-c9e3-4b1c-a038-d8416717fab3"
)

var providerid = fmt.Sprintf("%s/%s/%s", namespaceName, baremetalhostName, metal3machineName)

func TestManagers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Manager Suite")
}

var _ = BeforeSuite(func() {
	done := make(chan any)

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

		err = infrav1.AddToScheme(scheme.Scheme)
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
	if err := infrav1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := ipamv1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := capipamv1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := capipamv1beta1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := bmov1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
}

func testObjectMeta(name string, namespace string, uid string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		UID:       types.UID(uid),
	}
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
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				Name:     metal3ClusterName,
				Kind:     "InfrastructureConfig",
				APIGroup: "infrastructure.cluster.x-k8s.io/v1beta1",
			},
		},
		Status: clusterv1.ClusterStatus{
			Deprecated: &clusterv1.ClusterDeprecatedStatus{
				V1Beta1: &clusterv1.ClusterV1Beta1DeprecatedStatus{
					Conditions: clusterv1.Conditions{
						clusterv1.Condition{
							Type:   clusterv1.InfrastructureReadyV1Beta1Condition,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
		},
	}
}

func newMetal3Cluster(metal3ClusterName string, ownerRef *metav1.OwnerReference,
	spec *infrav1.Metal3ClusterSpec, status *infrav1.Metal3ClusterStatus) *infrav1.Metal3Cluster {
	if spec == nil {
		spec = &infrav1.Metal3ClusterSpec{}
	}
	if status == nil {
		status = &infrav1.Metal3ClusterStatus{}
	}
	ownerRefs := []metav1.OwnerReference{}
	if ownerRef != nil {
		ownerRefs = []metav1.OwnerReference{*ownerRef}
	}

	return &infrav1.Metal3Cluster{
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

func testObjectMetaWithOR(name string, m3mName string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespaceName,

		OwnerReferences: []metav1.OwnerReference{
			{
				Name:       m3mName,
				Kind:       metal3MachineKind,
				APIVersion: infrav1.GroupVersion.String(),
				UID:        m3muid,
			},
		},
	}
}
func testObjectReference(name string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name: name,
	}
}

func fakeClient(objects ...client.Object) client.Client {
	objs := []client.Object{}
	for _, o := range objects {
		if o != nil && reflect.ValueOf(o).Kind() == reflect.Ptr && !reflect.ValueOf(o).IsNil() {
			objs = append(objs, o)
		}
	}
	return fake.NewClientBuilder().
		WithScheme(setupScheme()).
		WithObjects(objs...).
		Build()
}
