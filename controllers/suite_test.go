/*

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

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	infrastructurev1alpha2 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
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

const (
	clusterName          = "testCluster"
	baremetalClusterName = "testBaremetalCluster"
	machineName          = "testMachine"
	bareMetalMachineName = "testBaremetalMachine"
	namespaceName        = "testNameSpace"
)

func init() {
	klog.InitFlags(nil)
}
func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := clusterv1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := infrav1.AddToScheme(s); err != nil {
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
func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{envtest.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = infrastructurev1alpha2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = infrastructurev1alpha2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

var deletionTimestamp = metav1.Now()

func bmcSpec() *infrav1.BareMetalClusterSpec {
	return &infrav1.BareMetalClusterSpec{
		APIEndpoint:     "http://192.168.111.249:6443",
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
				Name:       baremetalClusterName,
				Namespace:  namespaceName,
				Kind:       "BareMetalCluster",
				APIVersion: infrav1.GroupVersion.String(),
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

func newBareMetalCluster(baremetalName string, ownerRef *metav1.OwnerReference, spec *infrav1.BareMetalClusterSpec, status *infrav1.BareMetalClusterStatus) *infrav1.BareMetalCluster {
	if spec == nil {
		spec = &infrav1.BareMetalClusterSpec{}
	}
	if status == nil {
		status = &infrav1.BareMetalClusterStatus{}
	}
	ownerRefs := []metav1.OwnerReference{}
	if ownerRef != nil {
		ownerRefs = []metav1.OwnerReference{*ownerRef}
	}

	return &infrav1.BareMetalCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BareMetalCluster",
			APIVersion: infrav1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            baremetalClusterName,
			Namespace:       namespaceName,
			OwnerReferences: ownerRefs,
		},
		Spec:   *spec,
		Status: *status,
	}
}

func newMachine(clusterName, machineName string, bareMetalMachineName string) *clusterv1.Machine {
	machine := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Machine",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: namespaceName,
			Labels: map[string]string{
				clusterv1.MachineClusterLabelName: clusterName,
			},
		},
	}
	if bareMetalMachineName != "" {
		machine.Spec.InfrastructureRef = v1.ObjectReference{
			Name:       bareMetalMachineName,
			Namespace:  namespaceName,
			Kind:       "BareMetalMachine",
			APIVersion: infrav1.GroupVersion.String(),
		}
	}
	return machine
}

func newBareMetalMachine(name string, meta *metav1.ObjectMeta,
	spec *infrav1.BareMetalMachineSpec, status *infrav1.BareMetalMachineStatus,
) *infrav1.BareMetalMachine {

	if meta == nil {
		meta = &metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespaceName,
			OwnerReferences: []metav1.OwnerReference{},
			Annotations:     map[string]string{},
		}
	}
	meta.Name = name
	if spec == nil {
		spec = &infrav1.BareMetalMachineSpec{}
	}
	if status == nil {
		status = &infrav1.BareMetalMachineStatus{}
	}

	return &infrav1.BareMetalMachine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BareMetalMachine",
			APIVersion: infrav1.GroupVersion.String(),
		},
		ObjectMeta: *meta,
		Spec:       *spec,
		Status:     *status,
	}
}

func newBareMetalHost(spec *bmh.BareMetalHostSpec,
	status *bmh.BareMetalHostStatus,
) *bmh.BareMetalHost {

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
			APIVersion: bmh.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bmh-0",
			Namespace: namespaceName,
			UID:       "54db7dd5-269a-4d94-a12a-c4eafcecb8e7",
		},
		Spec:   *spec,
		Status: *status,
	}

}
