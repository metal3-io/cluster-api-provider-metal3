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

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Metal3IPPool manager", func() {
	DescribeTable("Test Finalizers",
		func(ipPool *capm3.Metal3IPPool) {
			ipPoolMgr, err := NewIPPoolManager(nil, ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			ipPoolMgr.SetFinalizer()

			Expect(ipPool.ObjectMeta.Finalizers).To(ContainElement(
				capm3.IPPoolFinalizer,
			))

			ipPoolMgr.UnsetFinalizer()

			Expect(ipPool.ObjectMeta.Finalizers).NotTo(ContainElement(
				capm3.IPPoolFinalizer,
			))
		},
		Entry("No finalizers", &capm3.Metal3IPPool{}),
		Entry("Additional Finalizers", &capm3.Metal3IPPool{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"foo"},
			},
		}),
	)

	type testRecreateIPPoolStatus struct {
		ipPool              *capm3.Metal3IPPool
		addresses           []*capm3.Metal3IPAddress
		expectedAddresses   map[string]string
		expectedAllocations map[string]string
	}

	DescribeTable("Test RecreateStatus",
		func(tc testRecreateIPPoolStatus) {
			objects := []runtime.Object{}
			for _, address := range tc.addresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = ipPoolMgr.RecreateStatusConditionally(context.TODO())
			Expect(err).NotTo(HaveOccurred())
			Expect(tc.ipPool.Status.Addresses).To(Equal(tc.expectedAddresses))
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))
			Expect(tc.ipPool.Status.LastUpdated.IsZero()).To(BeFalse())
		},
		Entry("No data", testRecreateIPPoolStatus{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: testObjectMeta,
			},
			expectedAddresses:   map[string]string{},
			expectedAllocations: map[string]string{},
		}),
		Entry("data present", testRecreateIPPoolStatus{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3IPPoolSpec{
					Allocations: map[string]capm3.IPAddress{
						"bcd": capm3.IPAddress("bcde"),
					},
				},
			},
			addresses: []*capm3.Metal3IPAddress{
				&capm3.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: "myns",
					},
					Spec: capm3.Metal3IPAddressSpec{
						Address: "abcd1",
						IPPool:  testObjectReference,
						Owner:   testObjectReference,
					},
				},
				&capm3.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bbc-1",
						Namespace: "myns",
					},
					Spec: capm3.Metal3IPAddressSpec{
						Address: "abcd2",
						IPPool: &corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
						Owner: &corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
					},
				},
				&capm3.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-2",
						Namespace: "myns",
					},
					Spec: capm3.Metal3IPAddressSpec{
						Address: "abcd3",
						IPPool:  nil,
						Owner:   testObjectReference,
					},
				},
				&capm3.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: "myns",
					},
					Spec: capm3.Metal3IPAddressSpec{
						Address: "abcd4",
						IPPool: &corev1.ObjectReference{
							Namespace: "myns",
						},
						Owner: nil,
					},
				},
			},
			expectedAddresses: map[string]string{
				"abcd1": "abc",
				"bcde":  "bcd",
			},
			expectedAllocations: map[string]string{
				"abc": "abc-0",
			},
		}),
		Entry("No recreation of the status", testRecreateIPPoolStatus{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: testObjectMeta,
				Status: capm3.Metal3IPPoolStatus{
					LastUpdated: &timeNow,
					Addresses:   map[string]string{},
					Allocations: map[string]string{},
				},
			},
			addresses: []*capm3.Metal3IPAddress{
				&capm3.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: "myns",
					},
					Spec: capm3.Metal3IPAddressSpec{
						Address: "abcd",
						IPPool:  testObjectReference,
						Owner:   testObjectReference,
					},
				},
			},
			expectedAddresses:   map[string]string{},
			expectedAllocations: map[string]string{},
		}),
	)

	type testCaseCreateAddresses struct {
		ipPool              *capm3.Metal3IPPool
		ipAddresses         []*capm3.Metal3IPAddress
		expectRequeue       bool
		expectError         bool
		expectedIPAddresses []string
		expectedAddresses   map[string]string
		expectedAllocations map[string]string
	}

	var ipPoolMeta = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: "myns",
		Labels: map[string]string{
			capi.ClusterLabelName: clusterName,
		},
		OwnerReferences: []metav1.OwnerReference{
			metav1.OwnerReference{
				Name:       "abc",
				Kind:       "Metal3Data",
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
			},
		},
	}

	DescribeTable("Test CreateAddresses",
		func(tc testCaseCreateAddresses) {
			objects := []runtime.Object{}
			for _, address := range tc.ipAddresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = ipPoolMgr.CreateAddresses(context.TODO())
			if tc.expectRequeue || tc.expectError {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			// get list of Metal3IPAddress objects
			addressObjects := capm3.Metal3IPAddressList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &addressObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(tc.expectedIPAddresses)).To(Equal(len(addressObjects.Items)))
			// Iterate over the Metal3IPAddress objects to find all indexes and objects
			for _, address := range addressObjects.Items {
				Expect(tc.expectedIPAddresses).To(ContainElement(address.Name))
				// TODO add further testing later
			}

			if !tc.expectError {
				Expect(tc.ipPool.Status.LastUpdated.IsZero()).To(BeFalse())
			}
			Expect(tc.ipPool.Status.Addresses).To(Equal(tc.expectedAddresses))
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))
		},
		Entry("No cluster label", testCaseCreateAddresses{
			ipPool:      &capm3.Metal3IPPool{},
			expectError: true,
		}),
		Entry("No OwnerRefs", testCaseCreateAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
					Labels: map[string]string{
						capi.ClusterLabelName: clusterName,
					},
				},
			},
		}),
		Entry("Wrong OwnerRefs", testCaseCreateAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
					Labels: map[string]string{
						capi.ClusterLabelName: clusterName,
					},
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Cluster",
							APIVersion: "cluster.x-k8s.io/v1alpha3",
						},
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: "cluster.x-k8s.io/v1alpha3",
						},
					},
				},
			},
		}),
		Entry("Already exists", testCaseCreateAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Status: capm3.Metal3IPPoolStatus{
					Allocations: map[string]string{
						"abc": "foo-0",
					},
				},
			},
			expectedAllocations: map[string]string{"abc": "foo-0"},
		}),
		Entry("Not allocated yet, pre-allocated", testCaseCreateAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: capm3.Metal3IPPoolSpec{
					Pools: []capm3.IPPool{
						capm3.IPPool{
							Start: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:   (*capm3.IPAddress)(pointer.StringPtr("192.168.0.20")),
						},
					},
					Allocations: map[string]capm3.IPAddress{
						"abc": capm3.IPAddress("192.168.0.15"),
					},
					NamePrefix: "abcpref",
				},
				Status: capm3.Metal3IPPoolStatus{},
			},
			expectedAllocations: map[string]string{"abc": "abcpref-192-168-0-15"},
			expectedAddresses:   map[string]string{"192.168.0.15": "abc"},
			expectedIPAddresses: []string{"abcpref-192-168-0-15"},
		}),
		Entry("Not allocated yet", testCaseCreateAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: capm3.Metal3IPPoolSpec{
					Pools: []capm3.IPPool{
						capm3.IPPool{
							Start: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:   (*capm3.IPAddress)(pointer.StringPtr("192.168.0.20")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: capm3.Metal3IPPoolStatus{
					Addresses: map[string]string{
						"192.168.0.11": "bcd",
					},
				},
			},
			expectedAllocations: map[string]string{"abc": "abcpref-192-168-0-12"},
			expectedAddresses: map[string]string{
				"192.168.0.12": "abc",
				"192.168.0.11": "bcd",
			},
			expectedIPAddresses: []string{"abcpref-192-168-0-12"},
		}),
		Entry("Not allocated yet, after error", testCaseCreateAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: capm3.Metal3IPPoolSpec{
					Pools: []capm3.IPPool{
						capm3.IPPool{
							Start: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:   (*capm3.IPAddress)(pointer.StringPtr("192.168.0.20")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: capm3.Metal3IPPoolStatus{
					Addresses: map[string]string{
						"192.168.0.11": "bcd",
					},
					Allocations: map[string]string{
						"abc": "",
					},
				},
			},
			expectedAllocations: map[string]string{"abc": "abcpref-192-168-0-12"},
			expectedAddresses: map[string]string{
				"192.168.0.12": "abc",
				"192.168.0.11": "bcd",
			},
			expectedIPAddresses: []string{"abcpref-192-168-0-12"},
		}),
		Entry("Not allocated yet, conflict", testCaseCreateAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: capm3.Metal3IPPoolSpec{
					Pools: []capm3.IPPool{
						capm3.IPPool{
							Start: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:   (*capm3.IPAddress)(pointer.StringPtr("192.168.0.20")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: capm3.Metal3IPPoolStatus{},
			},
			ipAddresses: []*capm3.Metal3IPAddress{
				&capm3.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-0-11",
						Namespace: "myns",
					},
					Spec: capm3.Metal3IPAddressSpec{
						Address: "192.168.0.11",
						IPPool: &corev1.ObjectReference{
							Name: "abc",
						},
						Owner: &corev1.ObjectReference{
							Name: "abc",
						},
					},
				},
			},
			expectedAllocations: map[string]string{"abc": "abcpref-192-168-0-11"},
			expectedAddresses:   map[string]string{"192.168.0.11": "abc"},
			expectedIPAddresses: []string{"abcpref-192-168-0-11"},
			expectRequeue:       true,
		}),
		Entry("Not allocated yet, exhausted pool", testCaseCreateAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: capm3.Metal3IPPoolSpec{
					Pools: []capm3.IPPool{
						capm3.IPPool{
							Start: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:   (*capm3.IPAddress)(pointer.StringPtr("192.168.0.11")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: capm3.Metal3IPPoolStatus{
					Addresses: map[string]string{"192.168.0.11": "bcde"},
				},
			},
			expectedAllocations: map[string]string{"abc": ""},
			expectedAddresses:   map[string]string{"192.168.0.11": "bcde"},
			expectError:         true,
		}),
	)

	type testCaseAllocateAddress struct {
		ipPool          *capm3.Metal3IPPool
		expectedAddress string
		expectedPrefix  int
		expectedGateway string
		expectError     bool
	}

	DescribeTable("Test AllocateAddress",
		func(tc testCaseAllocateAddress) {
			ipPoolMgr, err := NewIPPoolManager(nil, tc.ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())
			allocatedAddress, prefix, gateway, err := ipPoolMgr.allocateAddress(
				metav1.OwnerReference{
					Name: "TestRef",
				},
			)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(allocatedAddress).To(Equal(tc.expectedAddress))
			Expect(prefix).To(Equal(tc.expectedPrefix))
			Expect(string(*gateway)).To(Equal(tc.expectedGateway))
		},
		Entry("Empty pools", testCaseAllocateAddress{
			ipPool: &capm3.Metal3IPPool{
				Spec: capm3.Metal3IPPoolSpec{},
			},
			expectError: true,
		}),
		Entry("One pool, with start and existing address", testCaseAllocateAddress{
			ipPool: &capm3.Metal3IPPool{
				Spec: capm3.Metal3IPPoolSpec{
					Pools: []capm3.IPPool{
						capm3.IPPool{
							Start:   (*capm3.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:     (*capm3.IPAddress)(pointer.StringPtr("192.168.0.20")),
							Prefix:  26,
							Gateway: (*capm3.IPAddress)(pointer.StringPtr("192.168.1.1")),
						},
					},
					Allocations: map[string]capm3.IPAddress{
						"TestRef": capm3.IPAddress("192.168.0.15"),
					},
					Prefix:  24,
					Gateway: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.1")),
				},
				Status: capm3.Metal3IPPoolStatus{
					Addresses: map[string]string{
						"192.168.0.12": "bcde",
						"192.168.0.11": "abcd",
						"192.168.0.15": "TestRef",
					},
				},
			},
			expectedAddress: "192.168.0.15",
			expectedGateway: "192.168.0.1",
			expectedPrefix:  24,
		}),
		Entry("One pool, with start and existing address", testCaseAllocateAddress{
			ipPool: &capm3.Metal3IPPool{
				Spec: capm3.Metal3IPPoolSpec{
					Pools: []capm3.IPPool{
						capm3.IPPool{
							Start: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:   (*capm3.IPAddress)(pointer.StringPtr("192.168.0.20")),
						},
					},
					Prefix:  24,
					Gateway: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.1")),
				},
				Status: capm3.Metal3IPPoolStatus{
					Addresses: map[string]string{
						"192.168.0.12": "bcde",
						"192.168.0.11": "abcd",
					},
				},
			},
			expectedAddress: "192.168.0.13",
			expectedGateway: "192.168.0.1",
			expectedPrefix:  24,
		}),
		Entry("One pool, with subnet and override prefix", testCaseAllocateAddress{
			ipPool: &capm3.Metal3IPPool{
				Spec: capm3.Metal3IPPoolSpec{
					Pools: []capm3.IPPool{
						capm3.IPPool{
							Subnet:  (*capm3.IPSubnet)(pointer.StringPtr("192.168.0.10/24")),
							Prefix:  24,
							Gateway: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.1")),
						},
					},
					Prefix:  26,
					Gateway: (*capm3.IPAddress)(pointer.StringPtr("192.168.1.1")),
				},
				Status: capm3.Metal3IPPoolStatus{
					Addresses: map[string]string{
						"192.168.0.12": "bcde",
						"192.168.0.11": "abcd",
					},
				},
			},
			expectedAddress: "192.168.0.13",
			expectedGateway: "192.168.0.1",
			expectedPrefix:  24,
		}),
		Entry("two pools, with subnet and override prefix", testCaseAllocateAddress{
			ipPool: &capm3.Metal3IPPool{
				Spec: capm3.Metal3IPPoolSpec{
					Pools: []capm3.IPPool{
						capm3.IPPool{
							Start: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.10")),
							End:   (*capm3.IPAddress)(pointer.StringPtr("192.168.0.10")),
						},
						capm3.IPPool{
							Subnet:  (*capm3.IPSubnet)(pointer.StringPtr("192.168.1.10/24")),
							Prefix:  24,
							Gateway: (*capm3.IPAddress)(pointer.StringPtr("192.168.1.1")),
						},
					},
					Prefix:  26,
					Gateway: (*capm3.IPAddress)(pointer.StringPtr("192.168.2.1")),
				},
				Status: capm3.Metal3IPPoolStatus{
					Addresses: map[string]string{
						"192.168.1.11": "bcde",
						"192.168.0.10": "abcd",
					},
				},
			},
			expectedAddress: "192.168.1.12",
			expectedGateway: "192.168.1.1",
			expectedPrefix:  24,
		}),
		Entry("Exhausted pools start", testCaseAllocateAddress{
			ipPool: &capm3.Metal3IPPool{
				Spec: capm3.Metal3IPPoolSpec{
					Pools: []capm3.IPPool{
						capm3.IPPool{
							Start: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.10")),
							End:   (*capm3.IPAddress)(pointer.StringPtr("192.168.0.10")),
						},
					},
					Prefix:  24,
					Gateway: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.1")),
				},
				Status: capm3.Metal3IPPoolStatus{
					Addresses: map[string]string{
						"192.168.0.10": "abcd",
					},
				},
			},
			expectError: true,
		}),
		Entry("Exhausted pools subnet", testCaseAllocateAddress{
			ipPool: &capm3.Metal3IPPool{
				Spec: capm3.Metal3IPPoolSpec{
					Pools: []capm3.IPPool{
						capm3.IPPool{
							Subnet: (*capm3.IPSubnet)(pointer.StringPtr("192.168.0.0/30")),
						},
					},
					Prefix:  24,
					Gateway: (*capm3.IPAddress)(pointer.StringPtr("192.168.0.1")),
				},
				Status: capm3.Metal3IPPoolStatus{
					Addresses: map[string]string{
						"192.168.0.1": "abcd",
						"192.168.0.2": "abcd",
						"192.168.0.3": "abcd",
					},
				},
			},
			expectError: true,
		}),
	)

	type testCaseDeleteAddresses struct {
		ipPool              *capm3.Metal3IPPool
		addresses           []*capm3.Metal3IPAddress
		expectedAddresses   map[string]string
		expectedAllocations map[string]string
	}

	DescribeTable("Test DeleteAddresses",
		func(tc testCaseDeleteAddresses) {
			objects := []runtime.Object{}
			for _, data := range tc.addresses {
				objects = append(objects, data)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = ipPoolMgr.DeleteAddresses(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			// get list of Metal3IPAddress objects
			addressObjects := capm3.Metal3IPAddressList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &addressObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(addressObjects.Items)).To(Equal(0))

			Expect(tc.ipPool.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(tc.ipPool.Status.Addresses).To(Equal(tc.expectedAddresses))
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))
		},
		Entry("Empty IPPool", testCaseDeleteAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: testObjectMeta,
			},
		}),
		Entry("No Deletion needed", testCaseDeleteAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: "cluster.x-k8s.io/v1alpha3",
						},
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Metal3Machine",
							APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
						},
					},
				},
				Status: capm3.Metal3IPPoolStatus{
					Allocations: map[string]string{
						"abc": "abc-0",
					},
					Addresses: map[string]string{
						"0": "abc",
					},
				},
			},
			expectedAddresses:   map[string]string{"0": "abc"},
			expectedAllocations: map[string]string{"abc": "abc-0"},
		}),
		Entry("Deletion needed, not found", testCaseDeleteAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: "cluster.x-k8s.io/v1alpha3",
						},
					},
				},
				Status: capm3.Metal3IPPoolStatus{
					Allocations: map[string]string{
						"abc": "abc-0",
					},
					Addresses: map[string]string{
						"0": "abc",
					},
				},
			},
			expectedAddresses:   map[string]string{},
			expectedAllocations: map[string]string{},
		}),
		Entry("Deletion needed", testCaseDeleteAddresses{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: "cluster.x-k8s.io/v1alpha3",
						},
					},
				},
				Status: capm3.Metal3IPPoolStatus{
					Allocations: map[string]string{
						"abc": "abc-0",
					},
					Addresses: map[string]string{
						"0": "abc",
					},
				},
			},
			expectedAddresses:   map[string]string{},
			expectedAllocations: map[string]string{},
			addresses: []*capm3.Metal3IPAddress{
				&capm3.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-0",
					},
				},
			},
		}),
	)

	type testCaseDeleteReady struct {
		ipPool      *capm3.Metal3IPPool
		expectReady bool
	}
	DescribeTable("Test DeleteReady",
		func(tc testCaseDeleteReady) {
			ipPoolMgr, err := NewIPPoolManager(nil, tc.ipPool, klogr.New())
			Expect(err).NotTo(HaveOccurred())

			ready, err := ipPoolMgr.DeleteReady()
			Expect(err).NotTo(HaveOccurred())
			if tc.expectReady {
				Expect(ready).To(BeTrue())
			} else {
				Expect(ready).To(BeFalse())
			}
		},
		Entry("Ready", testCaseDeleteReady{
			ipPool:      &capm3.Metal3IPPool{},
			expectReady: true,
		}),
		Entry("Ready with OwnerRefs", testCaseDeleteReady{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Cluster",
							APIVersion: "cluster.x-k8s.io/v1alpha3",
						},
					},
				},
			},
			expectReady: true,
		}),
		Entry("Not Ready with OwnerRefs", testCaseDeleteReady{
			ipPool: &capm3.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Metal3Machine",
							APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
						},
					},
				},
			},
			expectReady: false,
		}),
	)
})
