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

	"github.com/go-logr/logr"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	Cluster       *clusterv1beta1.Cluster
	ExpectSuccess bool
}

type descendantsTestCase struct {
	Machines            []*clusterv1beta1.Machine
	OwnerRef            *metav1.OwnerReference
	ExpectError         bool
	ExpectedDescendants int
}

var _ = Describe("Metal3Cluster manager", func() {

	Describe("Test New Cluster Manager", func() {

		var fakeClient client.Client

		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().WithScheme(setupScheme()).Build()
		})

		DescribeTable("Test NewClusterManager",
			func(tc testCaseBMClusterManager) {
				_, err := NewClusterManager(fakeClient, tc.Cluster, tc.BMCluster,
					logr.Discard(),
				)
				if tc.ExpectSuccess {
					Expect(err).NotTo(HaveOccurred())
				} else {
					Expect(err).To(HaveOccurred())
				}
			},
			Entry("Cluster and BMCluster Defined", testCaseBMClusterManager{
				Cluster:       &clusterv1beta1.Cluster{},
				BMCluster:     &infrav1.Metal3Cluster{},
				ExpectSuccess: true,
			}),
			Entry("BMCluster undefined", testCaseBMClusterManager{
				Cluster:       &clusterv1beta1.Cluster{},
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
			clusterMgr := newBMClusterSetup(tc)

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

	DescribeTable("Test setting errors",
		func(tc testCaseBMClusterManager) {
			clusterMgr := newBMClusterSetup(tc)
			clusterMgr.setError("abc", capierrors.InvalidConfigurationClusterError)

			Expect(*tc.BMCluster.Status.FailureReason).To(Equal(
				capierrors.InvalidConfigurationClusterError,
			))
			Expect(*tc.BMCluster.Status.FailureMessage).To(Equal("abc"))
		},
		Entry("No pre-existing errors", testCaseBMClusterManager{
			Cluster: newCluster(clusterName),
			BMCluster: newMetal3Cluster(metal3ClusterName,
				bmcOwnerRef, nil, nil,
			),
		}),
		Entry("Pre-existing error message overridden", testCaseBMClusterManager{
			Cluster: newCluster(clusterName),
			BMCluster: newMetal3Cluster(metal3ClusterName,
				bmcOwnerRef, nil, &infrav1.Metal3ClusterStatus{
					FailureMessage: ptr.To("cba"),
				},
			),
		}),
	)

	DescribeTable("Test BM cluster Delete",
		func(tc testCaseBMClusterManager) {
			clusterMgr := newBMClusterSetup(tc)
			err := clusterMgr.Delete()

			if tc.ExpectSuccess {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},
		Entry("deleting BMCluster", testCaseBMClusterManager{
			Cluster:       &clusterv1beta1.Cluster{},
			BMCluster:     &infrav1.Metal3Cluster{},
			ExpectSuccess: true,
		}),
	)

	DescribeTable("Test BMCluster Create",
		func(tc testCaseBMClusterManager) {
			clusterMgr := newBMClusterSetup(tc)
			Expect(clusterMgr).NotTo(BeNil())

			err := clusterMgr.Create(context.TODO())

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
			Cluster: &clusterv1beta1.Cluster{},
			BMCluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef,
				bmcSpec(), nil,
			),
			ExpectSuccess: true,
		}),
		Entry("Cluster empty, BMCluster exists without owner",
			testCaseBMClusterManager{
				Cluster: &clusterv1beta1.Cluster{},
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
			clusterMgr := newBMClusterSetup(tc)
			Expect(clusterMgr).NotTo(BeNil())

			err := clusterMgr.UpdateClusterStatus()
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
			Cluster: &clusterv1beta1.Cluster{},
			BMCluster: newMetal3Cluster(metal3ClusterName, bmcOwnerRef,
				bmcSpec(), nil,
			),
			ExpectSuccess: true,
		}),
		Entry("Cluster empty, BMCluster exists without owner",
			testCaseBMClusterManager{
				Cluster: &clusterv1beta1.Cluster{},
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
			Machines:            []*clusterv1beta1.Machine{},
			OwnerRef:            bmcOwnerRef,
			ExpectError:         false,
			ExpectedDescendants: 0,
		}),
		Entry("One Cluster Descendant with proper OwnerRef", descendantsTestCase{
			Machines: []*clusterv1beta1.Machine{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Labels: map[string]string{
							clusterv1beta1.ClusterNameLabel: clusterName,
						},
					},
				},
			},
			OwnerRef:            bmcOwnerRef,
			ExpectError:         false,
			ExpectedDescendants: 1,
		}),
		Entry("One Cluster Descendant with wrong OwnerRef", descendantsTestCase{
			Machines: []*clusterv1beta1.Machine{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Labels: map[string]string{
							clusterv1beta1.ClusterNameLabel: clusterName,
						},
					},
				},
			},
			OwnerRef: &metav1.OwnerReference{
				APIVersion: "test////",
				Kind:       "Cluster",
				Name:       clusterName,
			},
			ExpectError:         true,
			ExpectedDescendants: 0,
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

			Expect(descendants.Items).To(HaveLen(tc.ExpectedDescendants))
		},
		descendantsTestCases,
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
		descendantsTestCases,
	)
})

func newBMClusterSetup(tc testCaseBMClusterManager) *ClusterManager {
	objects := []client.Object{}

	if tc.Cluster != nil {
		objects = append(objects, tc.Cluster)
	}
	if tc.BMCluster != nil {
		objects = append(objects, tc.BMCluster)
	}
	fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()

	return &ClusterManager{
		client:        fakeClient,
		Metal3Cluster: tc.BMCluster,
		Cluster:       tc.Cluster,
		Log:           logr.Discard(),
	}
}

func descendantsSetup(tc descendantsTestCase) *ClusterManager {
	cluster := newCluster(clusterName)
	bmCluster := newMetal3Cluster(metal3ClusterName, tc.OwnerRef,
		nil, nil,
	)
	objects := []client.Object{
		cluster,
		bmCluster,
	}
	for _, machine := range tc.Machines {
		objects = append(objects, machine)
	}
	fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()

	return &ClusterManager{
		client:        fakeClient,
		Metal3Cluster: bmCluster,
		Cluster:       cluster,
		Log:           logr.Discard(),
	}
}
