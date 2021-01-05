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
	"reflect"

	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

var _ = Describe("Metal3LabelSync controller", func() {

	type TestCaseBuildLabelSyncSet struct {
		PrefixSet      map[string]struct{}
		Labels         map[string]string
		ExpectedResult map[string]string
	}

	DescribeTable("Build Label Sync Set",
		func(tc TestCaseBuildLabelSyncSet) {
			got := buildLabelSyncSet(tc.PrefixSet, tc.Labels)
			Expect(reflect.DeepEqual(got, tc.ExpectedResult)).To(Equal(true), "Expected %v but got %v", tc.ExpectedResult, got)
		},
		Entry("Single label case", TestCaseBuildLabelSyncSet{
			PrefixSet: map[string]struct{}{
				"foo.metal3.io": struct{}{},
			},
			Labels: map[string]string{
				"foo.metal3.io/bar": "blue",
			},
			ExpectedResult: map[string]string{
				"foo.metal3.io/bar": "blue",
			},
		}),
		Entry("Multiple label case", TestCaseBuildLabelSyncSet{
			PrefixSet: map[string]struct{}{
				"foo.metal3.io": struct{}{},
				"boo.metal3.io": struct{}{},
			},
			Labels: map[string]string{
				"foo.metal3.io/bar":  "blue",
				"foo.metal3.io/car":  "red",
				"boo.metal3.io/bill": "green",
			},
			ExpectedResult: map[string]string{
				"foo.metal3.io/bar":  "blue",
				"foo.metal3.io/car":  "red",
				"boo.metal3.io/bill": "green",
			},
		}),
		Entry("Ignore labels not in prefix set", TestCaseBuildLabelSyncSet{
			PrefixSet: map[string]struct{}{
				"foo.metal3.io": struct{}{},
			},
			Labels: map[string]string{
				"boo.metal3.io/blah":     "red",
				"foo.metal4.io/blahblah": "green",
				"foo.metal3.io/bar":      "blue",
			},
			ExpectedResult: map[string]string{
				"foo.metal3.io/bar": "blue",
			},
		}),
		Entry("Empty prefix set", TestCaseBuildLabelSyncSet{
			PrefixSet: map[string]struct{}{},
			Labels: map[string]string{
				"foo.metal3.io/blah":     "red",
				"foo.metal3.io/blahblah": "green",
				"boo.metal3.io/bar":      "blue",
			},
			ExpectedResult: map[string]string{},
		}),
		Entry("Empty labels", TestCaseBuildLabelSyncSet{
			PrefixSet: map[string]struct{}{
				"foo.metal3.io": struct{}{},
			},
			Labels:         map[string]string{},
			ExpectedResult: map[string]string{},
		}),
	)

	type TestCaseSynchronizeLabelSyncSetsOnNode struct {
		PrefixSet      map[string]struct{}
		Host           *bmh.BareMetalHost
		Node           *corev1.Node
		ExpectedResult map[string]string
	}

	DescribeTable("Build Label Sync Set",
		func(tc TestCaseSynchronizeLabelSyncSetsOnNode) {
			hostLabelSyncSet := buildLabelSyncSet(tc.PrefixSet, tc.Host.Labels)
			nodeLabelSyncSet := buildLabelSyncSet(tc.PrefixSet, tc.Node.Labels)
			synchronizeLabelSyncSetsOnNode(hostLabelSyncSet, nodeLabelSyncSet, tc.Node)
			Expect(reflect.DeepEqual(tc.Node.Labels, tc.ExpectedResult)).To(Equal(true), "Expected %v but got %v", tc.ExpectedResult, tc.Node.Labels)
		},
		Entry("Label exists, do nothing", TestCaseSynchronizeLabelSyncSetsOnNode{
			PrefixSet: map[string]struct{}{
				"foo.metal3.io": struct{}{},
			},
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo.metal3.io/bar": "blue",
					},
				},
			},
			Node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo.metal3.io/bar": "blue",
					},
				},
			},
			ExpectedResult: map[string]string{
				"foo.metal3.io/bar": "blue",
			},
		}),
		Entry("Add label on Node", TestCaseSynchronizeLabelSyncSetsOnNode{
			PrefixSet: map[string]struct{}{
				"foo.metal3.io": struct{}{},
			},
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo.metal3.io/bar": "blue",
					},
				},
			},
			Node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			ExpectedResult: map[string]string{
				"foo.metal3.io/bar": "blue",
			},
		}),
		Entry("Add multiple labels on Node, ignore existing label", TestCaseSynchronizeLabelSyncSetsOnNode{
			PrefixSet: map[string]struct{}{
				"foo.metal3.io": struct{}{},
				"boo.metal3.io": struct{}{},
			},
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo.metal3.io/bar":  "blue",
						"boo.metal3.io/ball": "red",
					},
				},
			},
			Node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"some.metal4.io/blah": "pink", // ignore
					},
				},
			},
			ExpectedResult: map[string]string{
				"foo.metal3.io/bar":   "blue",
				"boo.metal3.io/ball":  "red",
				"some.metal4.io/blah": "pink",
			},
		}),
		Entry("Remove and update labels from Node", TestCaseSynchronizeLabelSyncSetsOnNode{
			PrefixSet: map[string]struct{}{
				"foo.metal3.io": struct{}{},
				"boo.metal3.io": struct{}{},
			},
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo.metal3.io/bar": "blue",
					},
				},
			},
			Node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo.metal3.io/bar":  "XXXX", // update
						"foo.metal3.io/blah": "YYYY", // remove
					},
				},
			},
			ExpectedResult: map[string]string{
				"foo.metal3.io/bar": "blue",
			},
		}),
	)
	type TestCaseMetal3ClusterToBMHs struct {
		Cluster        *capi.Cluster
		M3Cluster      *infrav1.Metal3Cluster
		Machine        *capi.Machine
		M3Machine      *infrav1.Metal3Machine
		ExpectRequests []ctrl.Request
	}

	DescribeTable("Metal3Cluster To BareMetalHosts tests",
		func(tc TestCaseMetal3ClusterToBMHs) {
			objects := []runtime.Object{
				tc.Cluster,
				tc.M3Cluster,
				tc.Machine,
				tc.M3Machine,
			}
			c := fake.NewFakeClientWithScheme(setupScheme(), objects...)
			r := Metal3LabelSyncReconciler{
				Client: c,
				Log:    klogr.New(),
			}
			obj := handler.MapObject{
				Object: tc.M3Cluster,
			}
			reqs := r.Metal3ClusterToBareMetalHosts(obj)
			Expect(reflect.DeepEqual(reqs, tc.ExpectRequests)).To(Equal(true), "Expected %v but got %v", tc.ExpectRequests, reqs)
		},
		Entry("Metal3Cluster To BareMetalHost",
			TestCaseMetal3ClusterToBMHs{
				Cluster:   newCluster(clusterName, nil, nil),
				M3Cluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, false),
				Machine:   newMachine(clusterName, machineName, metal3machineName),
				M3Machine: newMetal3Machine(metal3machineName, m3mObjectMeta(), nil, nil, false),
				ExpectRequests: []ctrl.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      "myhost",
							Namespace: "myns",
						},
					},
				},
			},
		),
	)
})

func m3mObjectMeta() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            metal3machineName,
		Namespace:       namespaceName,
		OwnerReferences: m3mOwnerRefs(),
		Labels: map[string]string{
			capi.ClusterLabelName: clusterName,
		},
		Annotations: map[string]string{
			baremetal.HostAnnotation: "myns/myhost",
		},
	}
}
