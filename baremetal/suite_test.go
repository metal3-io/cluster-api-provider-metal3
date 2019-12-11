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

package baremetal

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	_ "github.com/go-logr/logr"
	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
)

const (
	clusterName          = "testCluster"
	baremetalClusterName = "testBaremetalCluster"
	namespaceName        = "testNameSpace"
)

func TestManagers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Manager Suite")
}

var bmcOwnerRef = &metav1.OwnerReference{
	APIVersion: clusterv1.GroupVersion.String(),
	Kind:       "Cluster",
	Name:       clusterName,
}

//-----------------------------------
//------ Helper functions -----------
//-----------------------------------
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
			InfrastructureRef: &v1.ObjectReference{
				Name:       baremetalClusterName,
				Namespace:  namespaceName,
				Kind:       "InfrastructureConfig",
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha2",
			},
		},
		Status: clusterv1.ClusterStatus{
			InfrastructureReady: true,
		},
	}
}

func newBareMetalCluster(baremetalName string, ownerRef *metav1.OwnerReference,
	spec *infrav1.BareMetalClusterSpec, status *infrav1.BareMetalClusterStatus) *infrav1.BareMetalCluster {
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
			Kind: "BareMetalCluster",
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
