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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	_ "github.com/go-logr/logr"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func bmcSpec() *infrav1.Metal3ClusterSpec {
	return &infrav1.Metal3ClusterSpec{
		ControlPlaneEndpoint: infrav1.APIEndpoint{
			Host: "192.168.111.249",
			Port: 6443,
		},
	}
}

func bmcSpecAPIEmpty() *infrav1.Metal3ClusterSpec {
	return &infrav1.Metal3ClusterSpec{
		ControlPlaneEndpoint: infrav1.APIEndpoint{
			Host: "",
			Port: 0,
		},
	}
}

type testCaseBMClusterManager struct {
	BMCluster     *infrav1.Metal3Cluster
	Cluster       *clusterv1.Cluster
	ExpectSuccess bool
}

type descendantsTestCase struct {
	Machines            []*clusterv1.Machine
	ExpectError         bool
	ExpectedDescendants int
}

var _ = Describe("Metal3Cluster manager", func() {

	Describe("Test New Cluster Manager", func() {

		var fakeClient client.Client

		BeforeEach(func() {
			fakeClient = fakeclient.NewFakeClientWithScheme(setupScheme())
		})

		DescribeTable("Test NewClusterManager",
			func(tc testCaseBMClusterManager) {
				_, err := NewClusterManager(fakeClient, tc.Cluster, tc.BMCluster,
					klogr.New(),
				)
				if tc.ExpectSuccess {
					Expect(err).NotTo(HaveOccurred())
				} else {
					Expect(err).To(HaveOccurred())
				}
			},
			Entry("Cluster and BMCluster Defined", testCaseBMClusterManager{
				Cluster:       &clusterv1.Cluster{},
				BMCluster:     &infrav1.Metal3Cluster{},
				ExpectSuccess: true,
			}),
			Entry("BMCluster undefined", testCaseBMClusterManager{
				Cluster:       &clusterv1.Cluster{},
				BMCluster:     nil,
				ExpectSuccess: false,
			}),
			Entry("Cluster undefined", testCaseBMClusterManager{
				Cluster:       nil,
				BMCluster:     &infrav1.Metal3Cluster{},
				ExpectSuccess: false,
			}),
		)
	})

	DescribeTable("Test Finalizers",
		func(tc testCaseBMClusterManager) {
			clusterMgr, err := newBMClusterSetup(tc)
			Expect(err).NotTo(HaveOccurred())

			clusterMgr.SetFinalizer()

			Expect(tc.BMCluster.ObjectMeta.Finalizers).To(ContainElement(
				infrav1.ClusterFinalizer,
			))

			clusterMgr.UnsetFinalizer()

			Expect(tc.BMCluster.ObjectMeta.Finalizers).NotTo(ContainElement(
				infrav1.ClusterFinalizer,
			))
		},
		Entry("No finalizers", testCaseBMClusterManager{
			Cluster: nil,
			BMCluster: newMetal3Cluster(metal3ClusterName,
				bmcOwnerRef, nil, nil,
			),
		}),
		Entry("Finalizers", testCaseBMClusterManager{
			Cluster: nil,
			BMCluster: &infrav1.Metal3Cluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "Metal3Cluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            metal3ClusterName,
					Namespace:       namespaceName,
					OwnerReferences: []metav1.OwnerReference{*bmcOwnerRef},
					Finalizers:      []string{infrav1.ClusterFinalizer},
				},
				Spec:   infrav1.Metal3ClusterSpec{},
				Status: infrav1.Metal3ClusterStatus{},
			},
		}),
	)

	DescribeTable("Test setting and clearing errors",
		func(tc testCaseBMClusterManager) {
			clusterMgr, err := newBMClusterSetup(tc)
			Expect(err).NotTo(HaveOccurred())

			clusterMgr.setError("abc", capierrors.InvalidConfigurationClusterError)

			Expect(*tc.BMCluster.Status.FailureReason).To(Equal(
				capierrors.InvalidConfigurationClusterError,
			))
			Expect(*tc.BMCluster.Status.FailureMessage).To(Equal("abc"))

			clusterMgr.clearError()

			Expect(tc.BMCluster.Status.FailureReason).To(BeNil())
			Expect(tc.BMCluster.Status.FailureMessage).To(BeNil())
		},
		Entry("No pre-existing errors", testCaseBMClusterManager{
			Cluster: newCluster(clusterName),
			BMCluster: newMetal3Cluster(metal3ClusterName,
				bmcOwnerRef, nil, nil,
			),
		}),
		Entry("Pre-existing error message overriden", testCaseBMClusterManager{
			Cluster: newCluster(clusterName),
			BMCluster: newMetal3Cluster(metal3ClusterName,
				bmcOwnerRef, nil, &infrav1.Metal3ClusterStatus{
					FailureMessage: pointer.StringPtr("cba"),
				},
			),
		}),
	)

	DescribeTable("Test BM cluster Delete",
		func(tc testCaseBMClusterManager) {
			clusterMgr, err := newBMClusterSetup(tc)
			Expect(err).NotTo(HaveOccurred())
			err = clusterMgr.Delete()

			if tc.ExpectSuccess {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},
		Entry("deleting BMCluster", testCaseBMClusterManager{
			Cluster:       &clusterv1.Cluster{},
			BMCluster:     &infrav1.Metal3Cluster{},
			ExpectSuccess: true,
		}),
	)

	DescribeTable("Test BMCluster Create",
		func(tc testCaseBMClusterManager) {
			clusterMgr, err := newBMClusterSetup(tc)
			Expect(err).NotTo(HaveOccurred())
			Expect(clusterMgr).NotTo(BeNil())

			err = clusterMgr.Create(context.TODO())

			if tc.ExpectSuccess {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},
		Entry("Cluster and BMCluster exist", testCaseBMClusterManager{
			Cluster: newCluster(clusterName),
			BMCluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef,
				bmcSpec(), nil,
			),
			ExpectSuccess: true,
		}),
		Entry("Cluster exists, BMCluster empty", testCaseBMClusterManager{
			Cluster:       newCluster(clusterName),
			BMCluster:     &infrav1.Metal3Cluster{},
			ExpectSuccess: false,
		}),
		Entry("Cluster empty, BMCluster exists", testCaseBMClusterManager{
			Cluster: &clusterv1.Cluster{},
			BMCluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef,
				bmcSpec(), nil,
			),
			ExpectSuccess: true,
		}),
		Entry("Cluster empty, BMCluster exists without owner",
			testCaseBMClusterManager{
				Cluster: &clusterv1.Cluster{},
				BMCluster: newMetal3Cluster(metal3ClusterName, nil,
					bmcSpec(), nil,
				),
				ExpectSuccess: true,
			},
		),
		Entry("Cluster and BMCluster exist, BMC spec API empty",
			testCaseBMClusterManager{
				Cluster: newCluster(clusterName),
				BMCluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef,
					bmcSpecAPIEmpty(), nil,
				),
				ExpectSuccess: false,
			},
		),
	)

	DescribeTable("Test BMCluster Update",
		func(tc testCaseBMClusterManager) {
			clusterMgr, err := newBMClusterSetup(tc)
			Expect(err).NotTo(HaveOccurred())
			Expect(clusterMgr).NotTo(BeNil())

			err = clusterMgr.UpdateClusterStatus()
			Expect(err).NotTo(HaveOccurred())

			//apiEndPoints := tc.BMCluster.Status.APIEndpoints
			//if tc.ExpectSuccess {
			//	Expect(apiEndPoints[0].Host).To(Equal("192.168.111.249"))
			//	Expect(apiEndPoints[0].Port).To(Equal(6443))
			//} else {
			//	Expect(apiEndPoints[0].Host).To(Equal(""))
			//}
		},
		Entry("Cluster and BMCluster exist", testCaseBMClusterManager{
			Cluster: newCluster(clusterName),
			BMCluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef,
				bmcSpec(), nil,
			),
			ExpectSuccess: true,
		}),
		Entry("Cluster exists, BMCluster empty", testCaseBMClusterManager{
			Cluster:       newCluster(clusterName),
			BMCluster:     &infrav1.Metal3Cluster{},
			ExpectSuccess: false,
		}),
		Entry("Cluster empty, BMCluster exists", testCaseBMClusterManager{
			Cluster: &clusterv1.Cluster{},
			BMCluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef,
				bmcSpec(), nil,
			),
			ExpectSuccess: true,
		}),
		Entry("Cluster empty, BMCluster exists without owner",
			testCaseBMClusterManager{
				Cluster: &clusterv1.Cluster{},
				BMCluster: newMetal3Cluster(metal3ClusterName, nil, bmcSpec(),
					nil,
				),
				ExpectSuccess: true,
			},
		),
		Entry("Cluster and BMCluster exist, BMC spec API empty",
			testCaseBMClusterManager{
				Cluster: newCluster(clusterName),
				BMCluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef,
					bmcSpecAPIEmpty(), nil,
				),
				ExpectSuccess: false,
			},
		),
	)

	var descendantsTestCases = []TableEntry{
		Entry("No Cluster Descendants", descendantsTestCase{
			Machines:            []*clusterv1.Machine{},
			ExpectError:         false,
			ExpectedDescendants: 0,
		}),
		Entry("One Cluster Descendant", descendantsTestCase{
			Machines: []*clusterv1.Machine{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Labels: map[string]string{
							clusterv1.ClusterLabelName: clusterName,
						},
					},
				},
			},
			ExpectError:         false,
			ExpectedDescendants: 1,
		}),
	}

	DescribeTable("Test List Descendants",
		func(tc descendantsTestCase) {
			clusterMgr := descendantsSetup(tc)

			descendants, err := clusterMgr.listDescendants(context.TODO())
			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(len(descendants.Items)).To(Equal(tc.ExpectedDescendants))
		},
		descendantsTestCases...,
	)

	DescribeTable("Test Count Descendants",
		func(tc descendantsTestCase) {
			clusterMgr := descendantsSetup(tc)
			nbDescendants, err := clusterMgr.CountDescendants(context.TODO())

			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(nbDescendants).To(Equal(tc.ExpectedDescendants))
		},
		descendantsTestCases...,
	)
})

func newBMClusterSetup(tc testCaseBMClusterManager) (*ClusterManager, error) {
	objects := []runtime.Object{}

	if tc.Cluster != nil {
		objects = append(objects, tc.Cluster)
	}
	if tc.BMCluster != nil {
		objects = append(objects, tc.BMCluster)
	}
	c := fakeclient.NewFakeClientWithScheme(setupScheme(), objects...)

	return &ClusterManager{
		client:        c,
		Metal3Cluster: tc.BMCluster,
		Cluster:       tc.Cluster,
		Log:           klogr.New(),
	}, nil
}

func descendantsSetup(tc descendantsTestCase) *ClusterManager {
	cluster := newCluster(clusterName)
	bmCluster := newMetal3Cluster(metal3ClusterName, bmcOwnerRef,
		nil, nil,
	)
	objects := []runtime.Object{
		cluster,
		bmCluster,
	}
	for _, machine := range tc.Machines {
		objects = append(objects, machine)
	}
	c := fakeclient.NewFakeClientWithScheme(setupScheme(), objects...)

	return &ClusterManager{
		client:        c,
		Metal3Cluster: bmCluster,
		Cluster:       cluster,
		Log:           klogr.New(),
	}
}
