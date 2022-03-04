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
	"context"
	"reflect"

	"github.com/go-logr/logr"

	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientfake "k8s.io/client-go/kubernetes/fake"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
			Expect(reflect.DeepEqual(got, tc.ExpectedResult)).To(Equal(true),
				"Expected %v but got %v", tc.ExpectedResult, got)
		},
		Entry("Single label case", TestCaseBuildLabelSyncSet{
			PrefixSet: map[string]struct{}{
				"foo.metal3.io": {},
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
				"foo.metal3.io": {},
				"boo.metal3.io": {},
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
				"foo.metal3.io": {},
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
		Entry("Empty prefix set and empty labels", TestCaseBuildLabelSyncSet{
			PrefixSet:      map[string]struct{}{},
			Labels:         map[string]string{},
			ExpectedResult: map[string]string{},
		}),
		Entry("Empty labels", TestCaseBuildLabelSyncSet{
			PrefixSet: map[string]struct{}{
				"foo.metal3.io": {},
			},
			Labels:         map[string]string{},
			ExpectedResult: map[string]string{},
		}),
	)

	type TestCaseParsePrefixAnnotation struct {
		PrefixStr      string
		ExpectedErr    bool
		ExpectedResult map[string]struct{}
	}

	DescribeTable("Parse Prefix Annotation",
		func(tc TestCaseParsePrefixAnnotation) {
			prefixSet, err := parsePrefixAnnotation(tc.PrefixStr)
			if tc.ExpectedErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(reflect.DeepEqual(prefixSet, tc.ExpectedResult)).To(Equal(true),
					"Expected %v but got %v", tc.ExpectedResult, prefixSet)
			}
		},
		Entry("Parse single prefix", TestCaseParsePrefixAnnotation{
			PrefixStr:   "foo.metal3.io",
			ExpectedErr: false,
			ExpectedResult: map[string]struct{}{
				"foo.metal3.io": {},
			},
		}),
		Entry("Parse multiple prefixes", TestCaseParsePrefixAnnotation{
			PrefixStr:   "foo.metal3.io, moo.myprefix,,bar",
			ExpectedErr: false,
			ExpectedResult: map[string]struct{}{
				"foo.metal3.io": {},
				"moo.myprefix":  {},
				"bar":           {},
			},
		}),
		Entry("Parse empty prefix string", TestCaseParsePrefixAnnotation{
			PrefixStr:      "",
			ExpectedErr:    false,
			ExpectedResult: map[string]struct{}{},
		}),
		Entry("Parse empty prefix string with commas", TestCaseParsePrefixAnnotation{
			PrefixStr:      ",, ,,",
			ExpectedErr:    false,
			ExpectedResult: map[string]struct{}{},
		}),
		Entry("Invalid prefix does not meet DNS (RFC 1123)", TestCaseParsePrefixAnnotation{
			PrefixStr:      "foo.io, @bar.io",
			ExpectedErr:    true,
			ExpectedResult: nil,
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
			Expect(reflect.DeepEqual(tc.Node.Labels, tc.ExpectedResult)).To(Equal(true),
				"Expected %v but got %v", tc.ExpectedResult, tc.Node.Labels)
		},
		Entry("Label exists, do nothing", TestCaseSynchronizeLabelSyncSetsOnNode{
			PrefixSet: map[string]struct{}{
				"foo.metal3.io": {},
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
				"foo.metal3.io": {},
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
				"foo.metal3.io": {},
				"boo.metal3.io": {},
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
				"foo.metal3.io": {},
				"boo.metal3.io": {},
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
		Entry("Empty prefix set, do nothing", TestCaseSynchronizeLabelSyncSetsOnNode{
			PrefixSet: map[string]struct{}{},
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"some.bmh-label.io/blah": "gray", // ignore
					},
				},
			},
			Node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"some.node-label.io/bar": "orange", // ignore
					},
				},
			},
			ExpectedResult: map[string]string{
				"some.node-label.io/bar": "orange",
			},
		}),
	)
	type TestCaseMetal3ClusterToBMHs struct {
		Cluster        *clusterv1.Cluster
		M3Cluster      *capm3.Metal3Cluster
		Machine        *clusterv1.Machine
		M3Machine      *capm3.Metal3Machine
		ExpectRequests []ctrl.Request
	}

	DescribeTable("Metal3Cluster To BareMetalHosts tests",
		func(tc TestCaseMetal3ClusterToBMHs) {
			objects := []client.Object{
				tc.Cluster,
				tc.M3Cluster,
				tc.Machine,
				tc.M3Machine,
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			r := Metal3LabelSyncReconciler{
				Client: fakeClient,
				Log:    logr.Discard(),
			}
			obj := client.Object(tc.M3Cluster)
			reqs := r.Metal3ClusterToBareMetalHosts(obj)
			Expect(reflect.DeepEqual(reqs, tc.ExpectRequests)).To(Equal(true),
				"Expected %v but got %v", tc.ExpectRequests, reqs)
		},
		Entry("Metal3Cluster To BareMetalHost",
			TestCaseMetal3ClusterToBMHs{
				Cluster:   newCluster(clusterName, nil, nil),
				M3Cluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, nil, false),
				Machine:   newMachine(clusterName, machineName, metal3machineName, ""),
				M3Machine: newMetal3Machine(metal3machineName, m3mObjectMeta(), nil, nil, false),
				ExpectRequests: []ctrl.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      "myhost",
							Namespace: namespaceName,
						},
					},
				},
			},
		),
	)
	Describe("Test labelsync Reconcile functions", func() {
		Labels := map[string]string{
			"foo.metal3.io/bar": "blue",
		}
		metal3MachineSpec := bmh.BareMetalHostSpec{
			ConsumerRef: &corev1.ObjectReference{
				Name:       metal3machineName,
				Namespace:  namespaceName,
				Kind:       "Metal3Machine",
				APIVersion: capm3.GroupVersion.String(),
			},
		}
		notMetal3MachineSpec := bmh.BareMetalHostSpec{
			ConsumerRef: &corev1.ObjectReference{
				Name:       metal3machineName,
				Namespace:  namespaceName,
				Kind:       "notMetal3Machine",
				APIVersion: "not" + capm3.GroupVersion.String(),
			},
		}
		annotation := map[string]string{
			"metal3.io/metal3-label-sync-prefixes": "foo.metal3.io",
		}
		incorrectAnnotation := map[string]string{
			"metal3.io/incorrect-metal3-label-sync-prefixes": "incorrect",
		}
		nodeName := "testNode"
		cluserCapiSpec := clusterv1.ClusterSpec{
			Paused: true,
			InfrastructureRef: &corev1.ObjectReference{
				Name:       metal3ClusterName,
				Namespace:  namespaceName,
				Kind:       "Metal3Cluster",
				APIVersion: capm3.GroupVersion.String(),
			},
		}
		type testCaseReconcile struct {
			host            *bmh.BareMetalHost
			machine         *clusterv1.Machine
			metal3Machine   *capm3.Metal3Machine
			cluster         *clusterv1.Cluster
			metal3Cluster   *capm3.Metal3Cluster
			expectError     bool
			expectRequeue   bool
			expectLabelsync map[string]string
			debug           bool
		}
		DescribeTable("Test reconcile",

			func(tc testCaseReconcile) {

				objects := []client.Object{}
				if tc.host != nil {
					objects = append(objects, tc.host)
				}
				if tc.cluster != nil {
					objects = append(objects, tc.cluster)
				}
				if tc.metal3Cluster != nil {
					objects = append(objects, tc.metal3Cluster)
				}
				if tc.machine != nil {
					objects = append(objects, tc.machine)
				}
				if tc.metal3Machine != nil {
					objects = append(objects, tc.metal3Machine)
				}

				fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
				corev1Client := clientfake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				}}).CoreV1()
				r := &Metal3LabelSyncReconciler{
					Client:         fakeClient,
					ManagerFactory: baremetal.NewManagerFactory(fakeClient),
					Log:            logr.Discard(),
					CapiClientGetter: func(ctx context.Context, client client.Client, cluster *clusterv1.Cluster) (
						clientcorev1.CoreV1Interface, error,
					) {
						return corev1Client, nil
					},
					WatchFilterValue: "",
				}
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "bmh-0",
						Namespace: namespaceName,
					},
				}
				result, err := r.Reconcile(context.TODO(), req)

				if tc.expectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
				if tc.expectRequeue {
					Expect(result.Requeue || result.RequeueAfter > 0).To(BeTrue())
				} else {
					Expect(result.Requeue || result.RequeueAfter > 0).To(BeFalse())
				}

				node, _ := corev1Client.Nodes().Get(context.TODO(), "testNode", metav1.GetOptions{})
				Expect(node.Labels).To(Equal(tc.expectLabelsync))
			},
			Entry("Baremetal host not found", testCaseReconcile{
				expectError:   false,
				expectRequeue: false,
			}),
			Entry("Paused Baremetal", testCaseReconcile{
				host:          newBareMetalHost(nil, nil, nil, true),
				expectRequeue: true,
			}),
			Entry("Baremetal host with no ConsumerRef", testCaseReconcile{
				host: newBareMetalHost(nil, nil, nil, false),
			}),
			Entry("Unknown API version in BareMetalHost ConsumerRef", testCaseReconcile{
				host: newBareMetalHost(&notMetal3MachineSpec, nil, Labels, false),
			}),
			Entry("Could not find associated Metal3Machine", testCaseReconcile{
				host:          newBareMetalHost(&metal3MachineSpec, nil, Labels, false),
				expectRequeue: true,
			}),
			Entry("Could not find Machine object", testCaseReconcile{
				host:          newBareMetalHost(&metal3MachineSpec, nil, Labels, false),
				metal3Machine: newMetal3Machine(metal3machineName, m3mObjectMetaWithOwnerRef(), nil, nil, false),
				expectError:   true,
			}),
			Entry("Could not find Node Ref", testCaseReconcile{
				host:          newBareMetalHost(&metal3MachineSpec, nil, Labels, false),
				metal3Machine: newMetal3Machine(metal3machineName, m3mObjectMetaWithOwnerRef(), nil, nil, false),
				machine:       newMachine(clusterName, machineName, metal3machineName, ""),
				expectRequeue: true,
			}),
			Entry("Error fetching cluster", testCaseReconcile{
				host:          newBareMetalHost(&metal3MachineSpec, nil, Labels, false),
				metal3Machine: newMetal3Machine(metal3machineName, m3mObjectMetaWithOwnerRef(), nil, nil, false),
				machine:       newMachine(clusterName, machineName, metal3machineName, nodeName),
				expectError:   true,
				expectRequeue: true,
			}),
			Entry("Error fetching Metal3Cluster", testCaseReconcile{
				host:          newBareMetalHost(&metal3MachineSpec, nil, Labels, false),
				metal3Machine: newMetal3Machine(metal3machineName, m3mObjectMetaWithOwnerRef(), nil, nil, false),
				machine:       newMachine(clusterName, machineName, metal3machineName, nodeName),
				cluster:       newCluster(clusterName, nil, nil),
				expectError:   true,
				expectRequeue: true,
			}),
			Entry("Cluster is paused", testCaseReconcile{
				host:          newBareMetalHost(&metal3MachineSpec, nil, Labels, false),
				metal3Machine: newMetal3Machine(metal3machineName, m3mObjectMetaWithOwnerRef(), nil, nil, false),
				machine:       newMachine(clusterName, machineName, metal3machineName, nodeName),
				cluster:       newCluster(clusterName, &cluserCapiSpec, nil),
				metal3Cluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, annotation, false),
				expectRequeue: true,
			}),
			Entry("Nil annotations", testCaseReconcile{
				host:          newBareMetalHost(&metal3MachineSpec, nil, Labels, false),
				metal3Machine: newMetal3Machine(metal3machineName, m3mObjectMetaWithOwnerRef(), nil, nil, false),
				machine:       newMachine(clusterName, machineName, metal3machineName, nodeName),
				cluster:       newCluster(clusterName, nil, nil),
				metal3Cluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, nil, false),
			}),
			Entry("No annotation for the prefixes found on Metal3Cluster", testCaseReconcile{
				host:          newBareMetalHost(&metal3MachineSpec, nil, Labels, false),
				metal3Machine: newMetal3Machine(metal3machineName, m3mObjectMetaWithOwnerRef(), nil, nil, false),
				machine:       newMachine(clusterName, machineName, metal3machineName, nodeName),
				cluster:       newCluster(clusterName, nil, nil),
				metal3Cluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, incorrectAnnotation, false),
			}),
			Entry("No errors", testCaseReconcile{
				host:          newBareMetalHost(&metal3MachineSpec, nil, Labels, false),
				machine:       newMachine(clusterName, machineName, metal3machineName, nodeName),
				debug:         true,
				metal3Machine: newMetal3Machine(metal3machineName, m3mObjectMetaWithOwnerRef(), nil, nil, false),
				cluster:       newCluster(clusterName, nil, nil),
				metal3Cluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, annotation, false),
				expectRequeue: true,
				expectLabelsync: map[string]string{
					"foo.metal3.io/bar": "blue",
				},
			}),
		)
		type TestCaseReconcileBMHLabels struct {
			PrefixSet   map[string]struct{}
			Host        *bmh.BareMetalHost
			Machine     *clusterv1.Machine
			Cluster     *clusterv1.Cluster
			ExpectError bool
		}

		DescribeTable("Test reconcileBMHLabels",
			func(tc TestCaseReconcileBMHLabels) {
				objects := []client.Object{
					tc.Host,
					tc.Cluster,
					tc.Machine,
				}
				fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
				corev1Client := clientfake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				}}).CoreV1()
				r := &Metal3LabelSyncReconciler{
					Client:         fakeClient,
					ManagerFactory: baremetal.NewManagerFactory(fakeClient),
					Log:            logr.Discard(),
					CapiClientGetter: func(ctx context.Context, client client.Client, cluster *clusterv1.Cluster) (
						clientcorev1.CoreV1Interface, error,
					) {
						return corev1Client, nil
					},
					WatchFilterValue: "",
				}
				err := r.reconcileBMHLabels(context.TODO(),
					tc.Host, tc.Machine, tc.Cluster, tc.PrefixSet)

				if tc.ExpectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},
			Entry("No errors", TestCaseReconcileBMHLabels{
				PrefixSet: map[string]struct{}{
					"foo.metal3.io": {},
				},
				Host:    newBareMetalHost(nil, nil, Labels, false),
				Machine: newMachine(clusterName, machineName, metal3machineName, nodeName),
				Cluster: newCluster(clusterName, nil, nil),
			}),
		)
	})
})

func m3mObjectMeta() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            metal3machineName,
		Namespace:       namespaceName,
		OwnerReferences: m3mOwnerRefs(),
		Labels: map[string]string{
			clusterv1.ClusterLabelName: clusterName,
		},
		Annotations: map[string]string{
			baremetal.HostAnnotation: namespaceName + "/myhost",
		},
	}
}
