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

package controllers

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha5"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"

	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1alpha5 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha5"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var timestampNow = metav1.Now()

const (
	clusterName               = "controllerns_testCluster"
	metal3ClusterName         = "controllerns_testmetal3Cluster"
	machineName               = "controllerns_testMachine"
	metal3machineName         = "controllerns_testmetal3machine"
	namespaceName             = "controllerns"
	metal3machineTemplateName = "controllerns_testmetal3machinetemplate"
)

func init() {
	klog.InitFlags(nil)
	logf.SetLogger(klogr.New())

	// Register required object kinds with global scheme.
	_ = apiextensionsv1.AddToScheme(scheme.Scheme)
	_ = clusterv1.AddToScheme(scheme.Scheme)
	_ = infrav1alpha5.AddToScheme(scheme.Scheme)
	_ = ipamv1.AddToScheme(scheme.Scheme)
	_ = v1.AddToScheme(scheme.Scheme)
	_ = bmh.SchemeBuilder.AddToScheme(scheme.Scheme)
}

func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := clusterv1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := infrav1alpha5.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := ipamv1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := v1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := bmh.SchemeBuilder.AddToScheme(s); err != nil {
		panic(err)
	}

	return s
}
func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
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

		err = infrav1alpha5.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		err = apiextensionsv1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		// +kubebuilder:scaffold:scheme

		k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient).ToNot(BeNil())

		close(done)
	}()
	Eventually(done, 60).Should(BeClosed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

var deletionTimestamp = metav1.Now()

func clusterPauseSpec() *clusterv1.ClusterSpec {
	return &clusterv1.ClusterSpec{
		Paused: true,
		InfrastructureRef: &v1.ObjectReference{
			Name:       metal3ClusterName,
			Namespace:  namespaceName,
			Kind:       "Metal3Cluster",
			APIVersion: infrav1alpha5.GroupVersion.String(),
		},
	}
}

func m3mObjectMetaWithOwnerRef() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            metal3machineName,
		Namespace:       namespaceName,
		OwnerReferences: m3mOwnerRefs(),
		Labels: map[string]string{
			clusterv1.ClusterLabelName: clusterName,
		},
	}
}

func bmcSpec() *infrav1alpha5.Metal3ClusterSpec {
	return &infrav1alpha5.Metal3ClusterSpec{
		ControlPlaneEndpoint: infrav1alpha5.APIEndpoint{
			Host: "192.168.111.249",
			Port: 6443,
		},
		NoCloudProvider: true,
	}
}

func bmcOwnerRef() *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: clusterv1.GroupVersion.String(),
		Kind:       "Cluster",
		Name:       clusterName,
	}
}

func contains(haystack []string, needle string) bool {
	for _, straw := range haystack {
		if straw == needle {
			return true
		}
	}
	return false
}

func getKey(objectName string) *client.ObjectKey {
	return &client.ObjectKey{
		Name:      objectName,
		Namespace: namespaceName,
	}
}

func newCluster(clusterName string, spec *clusterv1.ClusterSpec, status *clusterv1.ClusterStatus) *clusterv1.Cluster {
	if spec == nil {
		spec = &clusterv1.ClusterSpec{
			InfrastructureRef: &v1.ObjectReference{
				Name:       metal3ClusterName,
				Namespace:  namespaceName,
				Kind:       "Metal3Cluster",
				APIVersion: infrav1alpha5.GroupVersion.String(),
			},
		}
	}
	if status == nil {
		status = &clusterv1.ClusterStatus{
			InfrastructureReady: true,
		}
	}
	return &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespaceName,
		},
		Spec:   *spec,
		Status: *status,
	}
}

func newMetal3Cluster(baremetalName string, ownerRef *metav1.OwnerReference, spec *infrav1alpha5.Metal3ClusterSpec, status *infrav1alpha5.Metal3ClusterStatus, pausedAnnotation bool) *infrav1alpha5.Metal3Cluster {
	if spec == nil {
		spec = &infrav1alpha5.Metal3ClusterSpec{}
	}
	if status == nil {
		status = &infrav1alpha5.Metal3ClusterStatus{}
	}
	ownerRefs := []metav1.OwnerReference{}
	if ownerRef != nil {
		ownerRefs = []metav1.OwnerReference{*ownerRef}
	}
	objMeta := &metav1.ObjectMeta{
		Name:            metal3ClusterName,
		Namespace:       namespaceName,
		OwnerReferences: ownerRefs,
	}
	if pausedAnnotation == true {
		objMeta = &metav1.ObjectMeta{
			Name:      metal3ClusterName,
			Namespace: namespaceName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       clusterName,
				},
			},
			Annotations: map[string]string{
				clusterv1.PausedAnnotation: "true",
			},
		}
	}

	return &infrav1alpha5.Metal3Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Metal3Cluster",
			APIVersion: infrav1alpha5.GroupVersion.String(),
		},
		ObjectMeta: *objMeta,
		Spec:       *spec,
		Status:     *status,
	}
}

func newMachine(clusterName, machineName string, metal3machineName string) *clusterv1.Machine {
	machine := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Machine",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: namespaceName,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
		},
	}
	if metal3machineName != "" {
		machine.Spec.InfrastructureRef = v1.ObjectReference{
			Name:       metal3machineName,
			Namespace:  namespaceName,
			Kind:       "Metal3Machine",
			APIVersion: infrav1alpha5.GroupVersion.String(),
		}
	}
	return machine
}

func newMetal3MachineTemplate(m3mTemplateName string, namespace string, annotations map[string]string) *capm3.Metal3MachineTemplate {
	return &capm3.Metal3MachineTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Metal3MachineTemplate",
			APIVersion: capm3.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        m3mTemplateName,
			Namespace:   namespace,
			Annotations: annotations,
		},
	}
}

func newMetal3Machine(name string, meta *metav1.ObjectMeta,
	spec *infrav1alpha5.Metal3MachineSpec, status *infrav1alpha5.Metal3MachineStatus,
	pausedAnnotation bool,
) *infrav1alpha5.Metal3Machine {

	if meta == nil {
		meta = &metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespaceName,
			OwnerReferences: []metav1.OwnerReference{},
			Annotations:     map[string]string{},
		}
	}

	if pausedAnnotation == true {
		meta = &metav1.ObjectMeta{
			Name:      name,
			Namespace: namespaceName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       machineName,
				},
			},
			Annotations: map[string]string{
				clusterv1.PausedAnnotation: "true",
			},
		}
	}

	meta.Name = name
	if spec == nil {
		spec = &infrav1alpha5.Metal3MachineSpec{}
	}
	if status == nil {
		status = &infrav1alpha5.Metal3MachineStatus{}
	}

	return &infrav1alpha5.Metal3Machine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Metal3Machine",
			APIVersion: infrav1alpha5.GroupVersion.String(),
		},
		ObjectMeta: *meta,
		Spec:       *spec,
		Status:     *status,
	}
}

func newBareMetalHost(bmhName string, spec *bmov1alpha1.BareMetalHostSpec,
	status *bmov1alpha1.BareMetalHostStatus, labels map[string]string, paused bool,
) *bmov1alpha1.BareMetalHost {
	if spec == nil {
		spec = &bmh.BareMetalHostSpec{}
	}
	if status == nil {
		status = &bmh.BareMetalHostStatus{
			Provisioning: bmh.ProvisionStatus{
				State: bmh.StateProvisioned,
			},
		}
	}
	return &bmh.BareMetalHost{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BareMetalHost",
			APIVersion: bmh.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bmhName,
			Namespace: namespaceName,
			UID:       bmhuid,
		},
		Spec:   *spec,
		Status: *status,
	}

}

func testObjectMeta(name string, namespace string, uid string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
}
