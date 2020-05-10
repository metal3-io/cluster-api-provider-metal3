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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
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
		func(ipPool *infrav1.Metal3IPPool) {
			ipPoolMgr, err := NewIPPoolManager(nil, ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			ipPoolMgr.SetFinalizer()

			Expect(ipPool.ObjectMeta.Finalizers).To(ContainElement(
				infrav1.IPPoolFinalizer,
			))

			ipPoolMgr.UnsetFinalizer()

			Expect(ipPool.ObjectMeta.Finalizers).NotTo(ContainElement(
				infrav1.IPPoolFinalizer,
			))
		},
		Entry("No finalizers", &infrav1.Metal3IPPool{}),
		Entry("Additional Finalizers", &infrav1.Metal3IPPool{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"foo"},
			},
		}),
	)

	type testCaseSetClusterOwnerRef struct {
		cluster     *capi.Cluster
		ipPool      *infrav1.Metal3IPPool
		expectError bool
	}

	DescribeTable("Test SetClusterOwnerRef",
		func(tc testCaseSetClusterOwnerRef) {
			ipPoolMgr, err := NewIPPoolManager(nil, tc.ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())
			err = ipPoolMgr.SetClusterOwnerRef(tc.cluster)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				_, err := findOwnerRefFromList(tc.ipPool.OwnerReferences,
					tc.cluster.TypeMeta, tc.cluster.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Cluster missing", testCaseSetClusterOwnerRef{
			expectError: true,
		}),
		Entry("no previous ownerref", testCaseSetClusterOwnerRef{
			ipPool: &infrav1.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc-cluster",
				},
			},
		}),
		Entry("previous ownerref", testCaseSetClusterOwnerRef{
			ipPool: &infrav1.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name: "def",
						},
					},
				},
			},
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc-cluster",
				},
			},
		}),
		Entry("ownerref present", testCaseSetClusterOwnerRef{
			ipPool: &infrav1.Metal3IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name: "def",
						},
						metav1.OwnerReference{
							Name: "abc-cluster",
						},
					},
				},
			},
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc-cluster",
				},
			},
		}),
	)

	type testGetIndexes struct {
		ipPool              *infrav1.Metal3IPPool
		addresses           []*infrav1.Metal3IPAddress
		expectError         bool
		expectedAddresses   map[infrav1.IPAddress]string
		expectedAllocations map[string]infrav1.IPAddress
	}

	DescribeTable("Test getIndexes",
		func(tc testGetIndexes) {
			objects := []runtime.Object{}
			for _, address := range tc.addresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			addressMap, err := ipPoolMgr.getIndexes(context.TODO())
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(addressMap).To(Equal(tc.expectedAddresses))
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))
			Expect(tc.ipPool.Status.LastUpdated.IsZero()).To(BeFalse())
		},
		Entry("No addresses", testGetIndexes{
			ipPool:              &infrav1.Metal3IPPool{},
			expectedAddresses:   map[infrav1.IPAddress]string{},
			expectedAllocations: map[string]infrav1.IPAddress{},
		}),
		Entry("addresses", testGetIndexes{
			ipPool: &infrav1.Metal3IPPool{
				ObjectMeta: testObjectMeta,
				Spec: infrav1.Metal3IPPoolSpec{
					PreAllocations: map[string]infrav1.IPAddress{
						"bcd": infrav1.IPAddress("bcde"),
					},
				},
			},
			addresses: []*infrav1.Metal3IPAddress{
				&infrav1.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3IPAddressSpec{
						Address: "abcd1",
						Pool:    *testObjectReference,
						Claim:   *testObjectReference,
					},
				},
				&infrav1.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bbc-1",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3IPAddressSpec{
						Address: "abcd2",
						Pool: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
					},
				},
				&infrav1.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-2",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3IPAddressSpec{
						Address: "abcd3",
						Pool:    corev1.ObjectReference{},
						Claim:   *testObjectReference,
					},
				},
				&infrav1.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3IPAddressSpec{
						Address: "abcd4",
						Pool: corev1.ObjectReference{
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{},
					},
				},
			},
			expectedAddresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("abcd1"): "abc",
				infrav1.IPAddress("bcde"):  "",
			},
			expectedAllocations: map[string]infrav1.IPAddress{
				"abc": infrav1.IPAddress("abcd1"),
			},
		}),
	)

	var ipPoolMeta = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: "myns",
	}

	type testCaseUpdateAddresses struct {
		ipPool                *infrav1.Metal3IPPool
		ipClaims              []*infrav1.Metal3IPClaim
		ipAddresses           []*infrav1.Metal3IPAddress
		expectRequeue         bool
		expectError           bool
		expectedNbAllocations int
		expectedAllocations   map[string]infrav1.IPAddress
	}

	DescribeTable("Test UpdateAddresses",
		func(tc testCaseUpdateAddresses) {
			objects := []runtime.Object{}
			for _, address := range tc.ipAddresses {
				objects = append(objects, address)
			}
			for _, claim := range tc.ipClaims {
				objects = append(objects, claim)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			nbAllocations, err := ipPoolMgr.UpdateAddresses(context.TODO())
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
			Expect(nbAllocations).To(Equal(tc.expectedNbAllocations))
			Expect(tc.ipPool.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))

			// get list of Metal3IPAddress objects
			addressObjects := infrav1.Metal3IPClaimList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &addressObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			// Iterate over the Metal3IPAddress objects to find all indexes and objects
			for _, claim := range addressObjects.Items {
				if claim.DeletionTimestamp.IsZero() {
					fmt.Printf("%#v", claim)
					Expect(claim.Status.Address).NotTo(BeNil())
				}
			}

		},
		Entry("No Claims", testCaseUpdateAddresses{
			ipPool: &infrav1.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
			},
			expectedAllocations: map[string]infrav1.IPAddress{},
		}),
		Entry("Claim and IP exist", testCaseUpdateAddresses{
			ipPool: &infrav1.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: infrav1.Metal3IPPoolSpec{
					NamePrefix: "abcpref",
				},
			},
			ipClaims: []*infrav1.Metal3IPClaim{
				&infrav1.Metal3IPClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
				},
				&infrav1.Metal3IPClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcd",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abcd",
							Namespace: "myns",
						},
					},
					Status: infrav1.Metal3IPClaimStatus{
						Address: &corev1.ObjectReference{
							Name:      "abcpref-192-168-1-12",
							Namespace: "myns",
						},
					},
				},
				&infrav1.Metal3IPClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abce",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
					Status: infrav1.Metal3IPClaimStatus{
						Address: &corev1.ObjectReference{
							Name:      "abcpref-192-168-1-12",
							Namespace: "myns",
						},
					},
				},
				&infrav1.Metal3IPClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "abcf",
						Namespace:         "myns",
						DeletionTimestamp: &timeNow,
					},
					Spec: infrav1.Metal3IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
					Status: infrav1.Metal3IPClaimStatus{
						Address: &corev1.ObjectReference{
							Name:      "abcpref-192-168-1-13",
							Namespace: "myns",
						},
					},
				},
			},
			ipAddresses: []*infrav1.Metal3IPAddress{
				&infrav1.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-11",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Address: infrav1.IPAddress("192.168.1.11"),
						Gateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.1")),
						Prefix:  24,
					},
				},
				&infrav1.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-12",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abce",
							Namespace: "myns",
						},
						Address: infrav1.IPAddress("192.168.1.12"),
					},
				},
				&infrav1.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-13",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abcf",
							Namespace: "myns",
						},
						Address: infrav1.IPAddress("192.168.1.13"),
					},
				},
			},
			expectedAllocations: map[string]infrav1.IPAddress{
				"abc":  infrav1.IPAddress("192.168.1.11"),
				"abce": infrav1.IPAddress("192.168.1.12"),
			},
			expectedNbAllocations: 2,
		}),
	)

	type testCaseCreateAddresses struct {
		ipPool              *infrav1.Metal3IPPool
		ipClaim             *infrav1.Metal3IPClaim
		ipAddresses         []*infrav1.Metal3IPAddress
		addresses           map[infrav1.IPAddress]string
		expectRequeue       bool
		expectError         bool
		expectedIPAddresses []string
		expectedAddresses   map[infrav1.IPAddress]string
		expectedAllocations map[string]infrav1.IPAddress
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

			allocatedMap, err := ipPoolMgr.createAddress(context.TODO(), tc.ipClaim,
				tc.addresses,
			)
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
			addressObjects := infrav1.Metal3IPAddressList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &addressObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(tc.expectedIPAddresses)).To(Equal(len(addressObjects.Items)))
			// Iterate over the Metal3IPAddress objects to find all indexes and objects
			for _, address := range addressObjects.Items {
				Expect(tc.expectedIPAddresses).To(ContainElement(address.Name))
				// TODO add further testing later
			}
			Expect(len(tc.ipClaim.Finalizers)).To(Equal(1))

			Expect(allocatedMap).To(Equal(tc.expectedAddresses))
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))
		},
		Entry("Already exists", testCaseCreateAddresses{
			ipPool: &infrav1.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Status: infrav1.Metal3IPPoolStatus{
					Allocations: map[string]infrav1.IPAddress{
						"abc": infrav1.IPAddress("foo-0"),
					},
				},
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]infrav1.IPAddress{
				"abc": infrav1.IPAddress("foo-0"),
			},
		}),
		Entry("Not allocated yet, pre-allocated", testCaseCreateAddresses{
			ipPool: &infrav1.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: infrav1.Metal3IPPoolSpec{
					Pools: []infrav1.IPPool{
						infrav1.IPPool{
							Start: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:   (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.20")),
						},
					},
					PreAllocations: map[string]infrav1.IPAddress{
						"abc": infrav1.IPAddress("192.168.0.15"),
					},
					NamePrefix: "abcpref",
				},
				Status: infrav1.Metal3IPPoolStatus{
					Allocations: map[string]infrav1.IPAddress{},
				},
			},
			addresses: map[infrav1.IPAddress]string{},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]infrav1.IPAddress{
				"abc": infrav1.IPAddress("192.168.0.15"),
			},
			expectedAddresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.15"): "abc",
			},
			expectedIPAddresses: []string{"abcpref-192-168-0-15"},
		}),
		Entry("Not allocated yet", testCaseCreateAddresses{
			ipPool: &infrav1.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: infrav1.Metal3IPPoolSpec{
					Pools: []infrav1.IPPool{
						infrav1.IPPool{
							Start: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:   (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.20")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: infrav1.Metal3IPPoolStatus{
					Allocations: map[string]infrav1.IPAddress{},
				},
			},
			addresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.11"): "bcd",
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]infrav1.IPAddress{
				"abc": infrav1.IPAddress("192.168.0.12"),
			},
			expectedAddresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.12"): "abc",
				infrav1.IPAddress("192.168.0.11"): "bcd",
			},
			expectedIPAddresses: []string{"abcpref-192-168-0-12"},
		}),
		Entry("Not allocated yet, conflict", testCaseCreateAddresses{
			ipPool: &infrav1.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: infrav1.Metal3IPPoolSpec{
					Pools: []infrav1.IPPool{
						infrav1.IPPool{
							Start: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:   (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.20")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: infrav1.Metal3IPPoolStatus{
					Allocations: map[string]infrav1.IPAddress{},
				},
			},
			addresses: map[infrav1.IPAddress]string{},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			ipAddresses: []*infrav1.Metal3IPAddress{
				&infrav1.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-0-11",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3IPAddressSpec{
						Address: "192.168.0.11",
						Pool: corev1.ObjectReference{
							Name: "abc",
						},
						Claim: corev1.ObjectReference{
							Name: "bcd",
						},
					},
				},
			},
			expectedAllocations: map[string]infrav1.IPAddress{},
			expectedAddresses:   map[infrav1.IPAddress]string{},
			expectedIPAddresses: []string{"abcpref-192-168-0-11"},
			expectRequeue:       true,
		}),
		Entry("Not allocated yet, exhausted pool", testCaseCreateAddresses{
			ipPool: &infrav1.Metal3IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: infrav1.Metal3IPPoolSpec{
					Pools: []infrav1.IPPool{
						infrav1.IPPool{
							Start: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:   (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.11")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: infrav1.Metal3IPPoolStatus{
					Allocations: map[string]infrav1.IPAddress{},
				},
			},
			addresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.11"): "bcd",
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]infrav1.IPAddress{},
			expectedAddresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.11"): "bcd",
			},
			expectedIPAddresses: []string{},
			expectError:         true,
		}),
	)

	type testCaseAllocateAddress struct {
		ipPool          *infrav1.Metal3IPPool
		ipClaim         *infrav1.Metal3IPClaim
		addresses       map[infrav1.IPAddress]string
		expectedAddress infrav1.IPAddress
		expectedPrefix  int
		expectedGateway *infrav1.IPAddress
		expectError     bool
	}

	DescribeTable("Test AllocateAddress",
		func(tc testCaseAllocateAddress) {
			ipPoolMgr, err := NewIPPoolManager(nil, tc.ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())
			allocatedAddress, prefix, gateway, err := ipPoolMgr.allocateAddress(
				tc.ipClaim, tc.addresses,
			)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(allocatedAddress).To(Equal(tc.expectedAddress))
			Expect(prefix).To(Equal(tc.expectedPrefix))
			Expect(*gateway).To(Equal(*tc.expectedGateway))
		},
		Entry("Empty pools", testCaseAllocateAddress{
			ipPool: &infrav1.Metal3IPPool{
				Spec: infrav1.Metal3IPPoolSpec{},
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectError: true,
		}),
		Entry("One pool, pre-allocated", testCaseAllocateAddress{
			ipPool: &infrav1.Metal3IPPool{
				Spec: infrav1.Metal3IPPoolSpec{
					Pools: []infrav1.IPPool{
						infrav1.IPPool{
							Start:   (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:     (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.20")),
							Prefix:  26,
							Gateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.1.1")),
						},
					},
					PreAllocations: map[string]infrav1.IPAddress{
						"TestRef": infrav1.IPAddress("192.168.0.15"),
					},
					Prefix:  24,
					Gateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.1")),
				},
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			expectedAddress: infrav1.IPAddress("192.168.0.15"),
			expectedGateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.1")),
			expectedPrefix:  24,
		}),
		Entry("One pool, with start and existing address", testCaseAllocateAddress{
			ipPool: &infrav1.Metal3IPPool{
				Spec: infrav1.Metal3IPPoolSpec{
					Pools: []infrav1.IPPool{
						infrav1.IPPool{
							Start: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:   (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.20")),
						},
					},
					Prefix:  24,
					Gateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.1")),
				},
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.12"): "bcde",
				infrav1.IPAddress("192.168.0.11"): "abcd",
			},
			expectedAddress: infrav1.IPAddress("192.168.0.13"),
			expectedGateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.1")),
			expectedPrefix:  24,
		}),
		Entry("One pool, with subnet and override prefix", testCaseAllocateAddress{
			ipPool: &infrav1.Metal3IPPool{
				Spec: infrav1.Metal3IPPoolSpec{
					Pools: []infrav1.IPPool{
						infrav1.IPPool{
							Start:   (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.11")),
							End:     (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.20")),
							Prefix:  24,
							Gateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.1")),
						},
					},
					Prefix:  26,
					Gateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.1.1")),
				},
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.12"): "bcde",
				infrav1.IPAddress("192.168.0.11"): "abcd",
			},
			expectedAddress: infrav1.IPAddress("192.168.0.13"),
			expectedGateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.1")),
			expectedPrefix:  24,
		}),
		Entry("two pools, with subnet and override prefix", testCaseAllocateAddress{
			ipPool: &infrav1.Metal3IPPool{
				Spec: infrav1.Metal3IPPoolSpec{
					Pools: []infrav1.IPPool{
						infrav1.IPPool{
							Start: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.10")),
							End:   (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.10")),
						},
						infrav1.IPPool{
							Subnet:  (*infrav1.IPSubnet)(pointer.StringPtr("192.168.1.10/24")),
							Prefix:  24,
							Gateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.1.1")),
						},
					},
					Prefix:  26,
					Gateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.2.1")),
				},
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.1.11"): "bcde",
				infrav1.IPAddress("192.168.0.10"): "abcd",
			},
			expectedAddress: infrav1.IPAddress("192.168.1.12"),
			expectedGateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.1.1")),
			expectedPrefix:  24,
		}),
		Entry("Exhausted pools start", testCaseAllocateAddress{
			ipPool: &infrav1.Metal3IPPool{
				Spec: infrav1.Metal3IPPoolSpec{
					Pools: []infrav1.IPPool{
						infrav1.IPPool{
							Start: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.10")),
							End:   (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.10")),
						},
					},
					Prefix:  24,
					Gateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.1")),
				},
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.10"): "abcd",
			},
			expectError: true,
		}),
		Entry("Exhausted pools subnet", testCaseAllocateAddress{
			ipPool: &infrav1.Metal3IPPool{
				Spec: infrav1.Metal3IPPoolSpec{
					Pools: []infrav1.IPPool{
						infrav1.IPPool{
							Subnet: (*infrav1.IPSubnet)(pointer.StringPtr("192.168.0.0/30")),
						},
					},
					Prefix:  24,
					Gateway: (*infrav1.IPAddress)(pointer.StringPtr("192.168.0.1")),
				},
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.1"): "abcd",
				infrav1.IPAddress("192.168.0.2"): "abcd",
				infrav1.IPAddress("192.168.0.3"): "abcd",
			},
			expectError: true,
		}),
	)

	type testCaseDeleteAddresses struct {
		ipPool              *infrav1.Metal3IPPool
		ipClaim             *infrav1.Metal3IPClaim
		m3addresses         []*infrav1.Metal3IPAddress
		addresses           map[infrav1.IPAddress]string
		expectedAddresses   map[infrav1.IPAddress]string
		expectedAllocations map[string]infrav1.IPAddress
		expectError         bool
	}

	DescribeTable("Test DeleteAddresses",
		func(tc testCaseDeleteAddresses) {
			objects := []runtime.Object{}
			for _, address := range tc.m3addresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			allocatedMap, err := ipPoolMgr.deleteAddress(context.TODO(), tc.ipClaim, tc.addresses)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			// get list of Metal3IPAddress objects
			addressObjects := infrav1.Metal3IPAddressList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &addressObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(addressObjects.Items)).To(Equal(0))

			Expect(tc.ipPool.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(allocatedMap).To(Equal(tc.expectedAddresses))
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))
			Expect(len(tc.ipClaim.Finalizers)).To(Equal(0))
		},
		Entry("Empty IPPool", testCaseDeleteAddresses{
			ipPool: &infrav1.Metal3IPPool{},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
		}),
		Entry("No Deletion needed", testCaseDeleteAddresses{
			ipPool: &infrav1.Metal3IPPool{},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			expectedAddresses: map[infrav1.IPAddress]string{infrav1.IPAddress("192.168.0.1"): "abcd"},
			addresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.1"): "abcd",
			},
		}),
		Entry("Deletion needed, not found", testCaseDeleteAddresses{
			ipPool: &infrav1.Metal3IPPool{
				Status: infrav1.Metal3IPPoolStatus{
					Allocations: map[string]infrav1.IPAddress{
						"TestRef": infrav1.IPAddress("192.168.0.1"),
					},
				},
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.1"): "TestRef",
			},
			expectedAllocations: map[string]infrav1.IPAddress{},
			expectedAddresses:   map[infrav1.IPAddress]string{},
		}),
		Entry("Deletion needed", testCaseDeleteAddresses{
			ipPool: &infrav1.Metal3IPPool{
				Spec: infrav1.Metal3IPPoolSpec{
					NamePrefix: "abc",
				},
				Status: infrav1.Metal3IPPoolStatus{
					Allocations: map[string]infrav1.IPAddress{
						"TestRef": infrav1.IPAddress("192.168.0.1"),
					},
				},
			},
			ipClaim: &infrav1.Metal3IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
					Finalizers: []string{
						infrav1.IPClaimFinalizer,
					},
				},
			},
			addresses: map[infrav1.IPAddress]string{
				infrav1.IPAddress("192.168.0.1"): "TestRef",
			},
			expectedAddresses:   map[infrav1.IPAddress]string{},
			expectedAllocations: map[string]infrav1.IPAddress{},
			m3addresses: []*infrav1.Metal3IPAddress{
				&infrav1.Metal3IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-192-168-0-1",
					},
				},
			},
		}),
	)

})
