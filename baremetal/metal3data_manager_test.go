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

	"github.com/go-logr/logr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	bmo "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testObjectMeta = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: namespaceName,
		UID:       bmhuid,
	}
	testObjectMetaWithOR = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: namespaceName,

		OwnerReferences: []metav1.OwnerReference{
			{
				Name:       "abc",
				Kind:       "Metal3Machine",
				APIVersion: capm3.GroupVersion.String(),
				UID:        m3muid,
			},
		},
	}
	testObjectReference = &corev1.ObjectReference{
		Name: "abc",
	}
)

var _ = Describe("Metal3Data manager", func() {
	DescribeTable("Test Finalizers",
		func(data *capm3.Metal3Data) {
			machineMgr, err := NewDataManager(nil, data,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			machineMgr.SetFinalizer()

			Expect(data.ObjectMeta.Finalizers).To(ContainElement(
				capm3.DataFinalizer,
			))

			machineMgr.UnsetFinalizer()

			Expect(data.ObjectMeta.Finalizers).NotTo(ContainElement(
				capm3.DataFinalizer,
			))
		},
		Entry("No finalizers", &capm3.Metal3Data{}),
		Entry("Additional Finalizers", &capm3.Metal3Data{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"foo"},
			},
		}),
	)

	It("Test error handling", func() {
		data := &capm3.Metal3Data{}
		dataMgr, err := NewDataManager(nil, data,
			logr.Discard(),
		)
		Expect(err).NotTo(HaveOccurred())
		dataMgr.setError(context.TODO(), "This is an error")
		Expect(*data.Status.ErrorMessage).To(Equal("This is an error"))

		dataMgr.clearError(context.TODO())
		Expect(data.Status.ErrorMessage).To(BeNil())
	})

	type testCaseReconcile struct {
		m3d              *capm3.Metal3Data
		m3dt             *capm3.Metal3DataTemplate
		m3m              *capm3.Metal3Machine
		expectError      bool
		expectRequeue    bool
		expectedErrorSet bool
	}

	DescribeTable("Test CreateSecret",
		func(tc testCaseReconcile) {
			objects := []client.Object{}
			if tc.m3dt != nil {
				objects = append(objects, tc.m3dt)
			}
			if tc.m3m != nil {
				objects = append(objects, tc.m3m)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			dataMgr, err := NewDataManager(fakeClient, tc.m3d,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			err = dataMgr.Reconcile(context.TODO())
			if tc.expectError || tc.expectRequeue {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.expectedErrorSet {
				Expect(tc.m3d.Status.ErrorMessage).NotTo(BeNil())
			} else {
				Expect(tc.m3d.Status.ErrorMessage).To(BeNil())
			}
		},
		Entry("Clear Error", testCaseReconcile{
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{},
				Status: capm3.Metal3DataStatus{
					ErrorMessage: pointer.StringPtr("Error Happened"),
				},
			},
		}),
		Entry("requeue error", testCaseReconcile{
			m3d: &capm3.Metal3Data{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3DataSpec{
					Template: *testObjectReference,
				},
			},
			expectRequeue: true,
		}),
	)

	type testCaseCreateSecrets struct {
		m3d                 *capm3.Metal3Data
		m3dt                *capm3.Metal3DataTemplate
		m3m                 *capm3.Metal3Machine
		dataClaim           *capm3.Metal3DataClaim
		machine             *clusterv1.Machine
		bmh                 *bmo.BareMetalHost
		metadataSecret      *corev1.Secret
		networkdataSecret   *corev1.Secret
		expectError         bool
		expectRequeue       bool
		expectReady         bool
		expectedMetadata    *string
		expectedNetworkData *string
	}

	DescribeTable("Test CreateSecret",
		func(tc testCaseCreateSecrets) {
			objects := []client.Object{}
			if tc.m3dt != nil {
				objects = append(objects, tc.m3dt)
			}
			if tc.m3m != nil {
				objects = append(objects, tc.m3m)
			}
			if tc.dataClaim != nil {
				objects = append(objects, tc.dataClaim)
			}
			if tc.machine != nil {
				objects = append(objects, tc.machine)
			}
			if tc.bmh != nil {
				objects = append(objects, tc.bmh)
			}
			if tc.metadataSecret != nil {
				objects = append(objects, tc.metadataSecret)
			}
			if tc.networkdataSecret != nil {
				objects = append(objects, tc.networkdataSecret)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			dataMgr, err := NewDataManager(fakeClient, tc.m3d,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			err = dataMgr.createSecrets(context.TODO())
			if tc.expectError || tc.expectRequeue {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
				}
				return
			}
			Expect(err).NotTo(HaveOccurred())
			if tc.expectReady {
				Expect(tc.m3d.Status.Ready).To(BeTrue())
			} else {
				Expect(tc.m3d.Status.Ready).To(BeFalse())
			}
			if tc.expectedMetadata != nil {
				tmpSecret := corev1.Secret{}
				err = fakeClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      "abc-metadata",
						Namespace: namespaceName,
					},
					&tmpSecret,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(tmpSecret.Data["metaData"])).To(Equal(*tc.expectedMetadata))
			}
			if tc.expectedNetworkData != nil {
				tmpSecret := corev1.Secret{}
				err = fakeClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      "abc-networkdata",
						Namespace: namespaceName,
					},
					&tmpSecret,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(tmpSecret.Data["networkData"])).To(Equal(*tc.expectedNetworkData))
			}
		},
		Entry("Empty", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{},
			},
		}),
		Entry("No Metal3DataTemplate", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3DataSpec{
					Template: *testObjectReference,
				},
			},
			expectRequeue: true,
		}),
		Entry("No Metal3Machine in owner refs", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3DataSpec{
					Template: *testObjectReference,
					Claim:    *testObjectReference,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMeta,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			expectError: true,
		}),
		Entry("No Metal3Machine", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Template: *testObjectReference,
					Claim:    *testObjectReference,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			expectRequeue: true,
		}),
		Entry("No Secret needed", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Template: *testObjectReference,
					Claim:    *testObjectReference,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
			},
			m3m: &capm3.Metal3Machine{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: testObjectReference,
				},
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			expectReady: true,
		}),
		Entry("Machine without datatemplate", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Template: *testObjectReference,
					Claim:    *testObjectReference,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
			},
			m3m: &capm3.Metal3Machine{
				ObjectMeta: testObjectMeta,
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			expectError: true,
		}),
		Entry("secrets exist", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Template: *testObjectReference,
					Claim:    *testObjectReference,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						Strings: []capm3.MetaDataString{
							{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &capm3.NetworkData{
						Links: capm3.NetworkDataLink{
							Ethernets: []capm3.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &capm3.NetworkLinkEthernetMac{
										String: pointer.StringPtr("XX:XX:XX:XX:XX:XX"),
									},
								},
							},
						},
					},
				},
			},
			m3m: &capm3.Metal3Machine{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: testObjectReference,
				},
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			metadataSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc-metadata",
					Namespace: namespaceName,
				},
				Data: map[string][]byte{
					"metaData": []byte("Hello"),
				},
			},
			networkdataSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc-networkdata",
					Namespace: namespaceName,
				},
				Data: map[string][]byte{
					"networkData": []byte("Bye"),
				},
			},
			expectReady:         true,
			expectedMetadata:    pointer.StringPtr("Hello"),
			expectedNetworkData: pointer.StringPtr("Bye"),
		}),
		Entry("secrets do not exist", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Template: *testObjectReference,
					Claim:    *testObjectReference,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						Strings: []capm3.MetaDataString{
							{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &capm3.NetworkData{
						Links: capm3.NetworkDataLink{
							Ethernets: []capm3.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &capm3.NetworkLinkEthernetMac{
										String: pointer.StringPtr("XX:XX:XX:XX:XX:XX"),
									},
								},
							},
						},
					},
				},
			},
			m3m: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
					UID:       m3muid,
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: clusterv1.GroupVersion.String(),
						},
					},
					Annotations: map[string]string{
						"metal3.io/BareMetalHost": namespaceName + "/abc",
					},
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: testObjectReference,
				},
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			machine: &clusterv1.Machine{
				ObjectMeta: testObjectMeta,
			},
			bmh: &bmo.BareMetalHost{
				ObjectMeta: testObjectMeta,
			},
			expectReady:         true,
			expectedMetadata:    pointer.StringPtr(fmt.Sprintf("String-1: String-1\nproviderid: %s\n", providerid)),
			expectedNetworkData: pointer.StringPtr("links:\n- ethernet_mac_address: XX:XX:XX:XX:XX:XX\n  id: eth0\n  mtu: 1500\n  type: phy\nnetworks: []\nservices: []\n"),
		}),
		Entry("No Machine OwnerRef on M3M", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Template: *testObjectReference,
					Claim:    *testObjectReference,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						Strings: []capm3.MetaDataString{
							{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &capm3.NetworkData{
						Links: capm3.NetworkDataLink{
							Ethernets: []capm3.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &capm3.NetworkLinkEthernetMac{
										String: pointer.StringPtr("XX:XX:XX:XX:XX:XX"),
									},
								},
							},
						},
					},
				},
			},
			m3m: &capm3.Metal3Machine{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: testObjectReference,
				},
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			expectRequeue: true,
		}),
		Entry("secrets do not exist", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Template: *testObjectReference,
					Claim:    *testObjectReference,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						Strings: []capm3.MetaDataString{
							{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &capm3.NetworkData{
						Links: capm3.NetworkDataLink{
							Ethernets: []capm3.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &capm3.NetworkLinkEthernetMac{
										String: pointer.StringPtr("XX:XX:XX:XX:XX:XX"),
									},
								},
							},
						},
					},
				},
			},
			m3m: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: clusterv1.GroupVersion.String(),
						},
					},
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: testObjectReference,
				},
			},
			machine: &clusterv1.Machine{
				ObjectMeta: testObjectMeta,
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			expectRequeue: true,
		}),
	)

	type testCaseReleaseLeases struct {
		m3d           *capm3.Metal3Data
		m3dt          *capm3.Metal3DataTemplate
		expectError   bool
		expectRequeue bool
	}

	DescribeTable("Test ReleaseLeases",
		func(tc testCaseReleaseLeases) {
			objects := []client.Object{}
			if tc.m3dt != nil {
				objects = append(objects, tc.m3dt)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			dataMgr, err := NewDataManager(fakeClient, tc.m3d,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			err = dataMgr.ReleaseLeases(context.TODO())
			if tc.expectError || tc.expectRequeue {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Empty spec", testCaseReleaseLeases{
			m3d: &capm3.Metal3Data{},
		}),
		Entry("M3dt not found", testCaseReleaseLeases{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataSpec{
					Template: corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			expectRequeue: true,
		}),
		Entry("M3dt found", testCaseReleaseLeases{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataSpec{
					Template: corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
			},
		}),
	)

	type testCaseGetAddressesFromPool struct {
		m3dtSpec      capm3.Metal3DataTemplateSpec
		ipClaims      []string
		expectError   bool
		expectRequeue bool
	}

	DescribeTable("Test GetAddressesFromPool",
		func(tc testCaseGetAddressesFromPool) {
			objects := []client.Object{}
			for _, poolName := range tc.ipClaims {
				pool := &ipamv1.IPClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-" + poolName,
						Namespace: namespaceName,
					},
					Spec: ipamv1.IPClaimSpec{
						Pool: *testObjectReference,
					},
				}
				objects = append(objects, pool)
			}
			m3d := &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: capm3.GroupVersion.String(),
				},
			}
			m3dt := capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
				Spec: tc.m3dtSpec,
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			dataMgr, err := NewDataManager(fakeClient, m3d,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			poolAddresses, err := dataMgr.getAddressesFromPool(context.TODO(), m3dt)
			if tc.expectError || tc.expectRequeue {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			expectedPoolAddress := make(map[string]addressFromPool)
			for _, poolName := range tc.ipClaims {
				expectedPoolAddress[poolName] = addressFromPool{}
			}
			Expect(expectedPoolAddress).To(Equal(poolAddresses))
		},
		Entry("Metadata ok", testCaseGetAddressesFromPool{
			m3dtSpec: capm3.Metal3DataTemplateSpec{
				MetaData: &capm3.MetaData{
					IPAddressesFromPool: []capm3.FromPool{
						{
							Key:  "Address-1",
							Name: "abcd-1",
						},
					},
					PrefixesFromPool: []capm3.FromPool{
						{
							Key:  "Prefix-1",
							Name: "abcd-2",
						},
					},
					GatewaysFromPool: []capm3.FromPool{
						{
							Key:  "Gateway-1",
							Name: "abcd-3",
						},
					},
				},
				NetworkData: &capm3.NetworkData{
					Networks: capm3.NetworkDataNetwork{
						IPv4: []capm3.NetworkDataIPv4{
							{
								IPAddressFromIPPool: "abcd-4",
								Routes: []capm3.NetworkDataRoutev4{
									{
										Gateway: capm3.NetworkGatewayv4{
											FromIPPool: pointer.StringPtr("abcd-5"),
										},
									},
								},
							},
						},
						IPv6: []capm3.NetworkDataIPv6{
							{
								IPAddressFromIPPool: "abcd-6",
								Routes: []capm3.NetworkDataRoutev6{
									{
										Gateway: capm3.NetworkGatewayv6{
											FromIPPool: pointer.StringPtr("abcd-7"),
										},
									},
								},
							},
						},
						IPv4DHCP: []capm3.NetworkDataIPv4DHCP{
							{
								Routes: []capm3.NetworkDataRoutev4{
									{
										Gateway: capm3.NetworkGatewayv4{
											FromIPPool: pointer.StringPtr("abcd-8"),
										},
									},
								},
							},
						},
						IPv6DHCP: []capm3.NetworkDataIPv6DHCP{
							{
								Routes: []capm3.NetworkDataRoutev6{
									{
										Gateway: capm3.NetworkGatewayv6{
											FromIPPool: pointer.StringPtr("abcd-9"),
										},
									},
								},
							},
						},
						IPv6SLAAC: []capm3.NetworkDataIPv6DHCP{
							{
								Routes: []capm3.NetworkDataRoutev6{
									{
										Gateway: capm3.NetworkGatewayv6{
											FromIPPool: pointer.StringPtr("abcd-10"),
										},
									},
								},
							},
						},
					},
				},
			},
			ipClaims: []string{
				"abcd-1",
				"abcd-2",
				"abcd-3",
				"abcd-4",
				"abcd-5",
				"abcd-6",
				"abcd-7",
				"abcd-8",
				"abcd-9",
				"abcd-10",
			},
			expectRequeue: true,
		}),
		Entry("IPAddressesFromPool", testCaseGetAddressesFromPool{
			m3dtSpec: capm3.Metal3DataTemplateSpec{
				MetaData: &capm3.MetaData{
					IPAddressesFromPool: []capm3.FromPool{
						{
							Key:  "Address-1",
							Name: "abcd",
						},
					},
				},
				NetworkData: &capm3.NetworkData{},
			},
			ipClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
		Entry("PrefixesFromPool", testCaseGetAddressesFromPool{
			m3dtSpec: capm3.Metal3DataTemplateSpec{
				MetaData: &capm3.MetaData{
					PrefixesFromPool: []capm3.FromPool{
						{
							Key:  "Prefix-1",
							Name: "abcd",
						},
					},
				},
				NetworkData: &capm3.NetworkData{},
			},
			ipClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
		Entry("GatewaysFromPool", testCaseGetAddressesFromPool{
			m3dtSpec: capm3.Metal3DataTemplateSpec{
				MetaData: &capm3.MetaData{
					GatewaysFromPool: []capm3.FromPool{
						{
							Key:  "Gateway-1",
							Name: "abcd",
						},
					},
				},
				NetworkData: &capm3.NetworkData{},
			},
			ipClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
		Entry("IPv4", testCaseGetAddressesFromPool{
			m3dtSpec: capm3.Metal3DataTemplateSpec{
				MetaData: &capm3.MetaData{},
				NetworkData: &capm3.NetworkData{
					Networks: capm3.NetworkDataNetwork{
						IPv4: []capm3.NetworkDataIPv4{
							{
								IPAddressFromIPPool: "abcd-1",
								Routes: []capm3.NetworkDataRoutev4{
									{
										Gateway: capm3.NetworkGatewayv4{
											FromIPPool: pointer.StringPtr("abcd-2"),
										},
									},
								},
							},
						},
					},
				},
			},
			ipClaims: []string{
				"abcd-1",
				"abcd-2",
			},
			expectRequeue: true,
		}),
		Entry("IPv6", testCaseGetAddressesFromPool{
			m3dtSpec: capm3.Metal3DataTemplateSpec{
				MetaData: &capm3.MetaData{},
				NetworkData: &capm3.NetworkData{
					Networks: capm3.NetworkDataNetwork{
						IPv6: []capm3.NetworkDataIPv6{
							{
								IPAddressFromIPPool: "abcd-1",
								Routes: []capm3.NetworkDataRoutev6{
									{
										Gateway: capm3.NetworkGatewayv6{
											FromIPPool: pointer.StringPtr("abcd-2"),
										},
									},
								},
							},
						},
					},
				},
			},
			ipClaims: []string{
				"abcd-1",
				"abcd-2",
			},
			expectRequeue: true,
		}),
		Entry("IPv4DHCP", testCaseGetAddressesFromPool{
			m3dtSpec: capm3.Metal3DataTemplateSpec{
				MetaData: &capm3.MetaData{},
				NetworkData: &capm3.NetworkData{
					Networks: capm3.NetworkDataNetwork{
						IPv4DHCP: []capm3.NetworkDataIPv4DHCP{
							{
								Routes: []capm3.NetworkDataRoutev4{
									{
										Gateway: capm3.NetworkGatewayv4{
											FromIPPool: pointer.StringPtr("abcd"),
										},
									},
								},
							},
						},
					},
				},
			},
			ipClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
		Entry("IPv6DHCP", testCaseGetAddressesFromPool{
			m3dtSpec: capm3.Metal3DataTemplateSpec{
				MetaData: &capm3.MetaData{},
				NetworkData: &capm3.NetworkData{
					Networks: capm3.NetworkDataNetwork{
						IPv6DHCP: []capm3.NetworkDataIPv6DHCP{
							{
								Routes: []capm3.NetworkDataRoutev6{
									{
										Gateway: capm3.NetworkGatewayv6{
											FromIPPool: pointer.StringPtr("abcd"),
										},
									},
								},
							},
						},
					},
				},
			},
			ipClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
		Entry("IPv6SLAAC", testCaseGetAddressesFromPool{
			m3dtSpec: capm3.Metal3DataTemplateSpec{
				MetaData: &capm3.MetaData{},
				NetworkData: &capm3.NetworkData{
					Networks: capm3.NetworkDataNetwork{
						IPv6SLAAC: []capm3.NetworkDataIPv6DHCP{
							{
								Routes: []capm3.NetworkDataRoutev6{
									{
										Gateway: capm3.NetworkGatewayv6{
											FromIPPool: pointer.StringPtr("abcd"),
										},
									},
								},
							},
						},
					},
				},
			},
			ipClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
	)

	type testCaseReleaseAddressesFromPool struct {
		m3dtSpec      capm3.Metal3DataTemplateSpec
		ipClaims      []string
		expectError   bool
		expectRequeue bool
	}

	DescribeTable("Test ReleaseAddressesFromPool",
		func(tc testCaseReleaseAddressesFromPool) {
			objects := []client.Object{}
			for _, poolName := range tc.ipClaims {
				pool := &ipamv1.IPClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-" + poolName,
						Namespace: namespaceName,
					},
					Spec: ipamv1.IPClaimSpec{
						Pool: *testObjectReference,
					},
				}
				objects = append(objects, pool)
			}
			m3d := &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: capm3.GroupVersion.String(),
				},
			}
			m3dt := capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "datatemplate-abc",
				},
				Spec: tc.m3dtSpec,
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			dataMgr, err := NewDataManager(fakeClient, m3d,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = dataMgr.releaseAddressesFromPool(context.TODO(), m3dt)
			if tc.expectError || tc.expectRequeue {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			for _, poolName := range tc.ipClaims {
				capm3IPPool := &ipamv1.IPClaim{}
				poolNamespacedName := types.NamespacedName{
					Name:      "abc-" + poolName,
					Namespace: m3d.Namespace,
				}

				err = dataMgr.client.Get(context.TODO(), poolNamespacedName, capm3IPPool)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		},
		Entry("Metadata ok", testCaseReleaseAddressesFromPool{
			m3dtSpec: capm3.Metal3DataTemplateSpec{
				MetaData: &capm3.MetaData{
					IPAddressesFromPool: []capm3.FromPool{
						{
							Key:  "Address-1",
							Name: "abcd-1",
						},
					},
					PrefixesFromPool: []capm3.FromPool{
						{
							Key:  "Prefix-1",
							Name: "abcd-2",
						},
					},
					GatewaysFromPool: []capm3.FromPool{
						{
							Key:  "Gateway-1",
							Name: "abcd-3",
						},
					},
				},
				NetworkData: &capm3.NetworkData{
					Networks: capm3.NetworkDataNetwork{
						IPv4: []capm3.NetworkDataIPv4{
							{
								IPAddressFromIPPool: "abcd-4",
								Routes: []capm3.NetworkDataRoutev4{
									{
										Gateway: capm3.NetworkGatewayv4{
											FromIPPool: pointer.StringPtr("abcd-5"),
										},
									},
								},
							},
						},
						IPv6: []capm3.NetworkDataIPv6{
							{
								IPAddressFromIPPool: "abcd-6",
								Routes: []capm3.NetworkDataRoutev6{
									{
										Gateway: capm3.NetworkGatewayv6{
											FromIPPool: pointer.StringPtr("abcd-7"),
										},
									},
								},
							},
						},
						IPv4DHCP: []capm3.NetworkDataIPv4DHCP{
							{
								Routes: []capm3.NetworkDataRoutev4{
									{
										Gateway: capm3.NetworkGatewayv4{
											FromIPPool: pointer.StringPtr("abcd-8"),
										},
									},
								},
							},
						},
						IPv6DHCP: []capm3.NetworkDataIPv6DHCP{
							{
								Routes: []capm3.NetworkDataRoutev6{
									{
										Gateway: capm3.NetworkGatewayv6{
											FromIPPool: pointer.StringPtr("abcd-9"),
										},
									},
								},
							},
						},
						IPv6SLAAC: []capm3.NetworkDataIPv6DHCP{
							{
								Routes: []capm3.NetworkDataRoutev6{
									{
										Gateway: capm3.NetworkGatewayv6{
											FromIPPool: pointer.StringPtr("abcd-10"),
										},
									},
								},
							},
						},
					},
				},
			},
			ipClaims: []string{
				"abcd-1",
				"abcd-2",
				"abcd-3",
				"abcd-4",
				"abcd-5",
				"abcd-6",
				"abcd-7",
				"abcd-8",
				"abcd-9",
				"abcd-10",
			},
		}),
	)

	type testCaseGetAddressFromPool struct {
		m3d               *capm3.Metal3Data
		poolName          string
		poolAddresses     map[string]addressFromPool
		ipClaim           *ipamv1.IPClaim
		ipAddress         *ipamv1.IPAddress
		expectError       bool
		expectRequeue     bool
		expectedAddresses map[string]addressFromPool
		expectDataError   bool
		expectClaim       bool
	}

	DescribeTable("Test GetAddressFromPool",
		func(tc testCaseGetAddressFromPool) {
			objects := []client.Object{}
			if tc.ipAddress != nil {
				objects = append(objects, tc.ipAddress)
			}
			if tc.ipClaim != nil {
				objects = append(objects, tc.ipClaim)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			dataMgr, err := NewDataManager(fakeClient, tc.m3d,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			poolAddresses, requeue, err := dataMgr.getAddressFromPool(
				context.TODO(), tc.poolName, tc.poolAddresses,
			)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.expectRequeue {
				Expect(requeue).To(BeTrue())
			} else {
				Expect(requeue).To(BeFalse())
			}
			if tc.expectDataError {
				Expect(tc.m3d.Status.ErrorMessage).NotTo(BeNil())
			} else {
				Expect(tc.m3d.Status.ErrorMessage).To(BeNil())
			}
			Expect(poolAddresses).To(Equal(tc.expectedAddresses))
			if tc.expectClaim {
				capm3IPClaim := &ipamv1.IPClaim{}
				claimNamespacedName := types.NamespacedName{
					Name:      tc.m3d.Name + "-" + tc.poolName,
					Namespace: tc.m3d.Namespace,
				}

				err = dataMgr.client.Get(context.TODO(), claimNamespacedName, capm3IPClaim)
				Expect(err).NotTo(HaveOccurred())
				_, err := findOwnerRefFromList(capm3IPClaim.OwnerReferences,
					tc.m3d.TypeMeta, tc.m3d.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Already processed", testCaseGetAddressFromPool{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
				},
			},
			poolName: "abc",
			poolAddresses: map[string]addressFromPool{
				"abc": {address: "addr"},
			},
			expectedAddresses: map[string]addressFromPool{
				"abc": {address: "addr"},
			},
		}),
		Entry("IPClaim not found", testCaseGetAddressFromPool{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
			},
			poolName: "abc",
			expectedAddresses: map[string]addressFromPool{
				"abc": {},
			},
			expectRequeue: true,
			expectClaim:   true,
		}),
		Entry("IPClaim without allocation", testCaseGetAddressFromPool{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
			},
			poolName: "abc",
			expectedAddresses: map[string]addressFromPool{
				"abc": {},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc-abc",
					Namespace: namespaceName,
				},
			},
			expectRequeue: true,
		}),
		Entry("IPPool with allocation error", testCaseGetAddressFromPool{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
			},
			poolName: "abc",
			expectedAddresses: map[string]addressFromPool{
				"abc": {},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc-abc",
					Namespace: namespaceName,
				},
				Status: ipamv1.IPClaimStatus{
					ErrorMessage: pointer.StringPtr("Error happened"),
				},
			},
			expectError:     true,
			expectDataError: true,
		}),
		Entry("IPAddress not found", testCaseGetAddressFromPool{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
			},
			poolName: "abc",
			expectedAddresses: map[string]addressFromPool{
				"abc": {},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc-abc",
					Namespace: namespaceName,
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{
						Name:      "abc-192.168.0.11",
						Namespace: namespaceName,
					},
				},
			},
			expectRequeue: true,
		}),
		Entry("IPAddress found", testCaseGetAddressFromPool{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
			},
			poolName: "abc",
			expectedAddresses: map[string]addressFromPool{
				"abc": {
					address: ipamv1.IPAddressStr("192.168.0.10"),
					prefix:  26,
					gateway: ipamv1.IPAddressStr("192.168.0.1"),
					dnsServers: []ipamv1.IPAddressStr{
						"8.8.8.8",
					},
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc-abc",
					Namespace: namespaceName,
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{
						Name:      "abc-192.168.0.10",
						Namespace: namespaceName,
					},
				},
			},

			ipAddress: &ipamv1.IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc-192.168.0.10",
					Namespace: namespaceName,
				},
				Spec: ipamv1.IPAddressSpec{
					Address: ipamv1.IPAddressStr("192.168.0.10"),
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.1")),
					DNSServers: []ipamv1.IPAddressStr{
						"8.8.8.8",
					},
				},
			},
		}),
	)

	type testCaseReleaseAddressFromPool struct {
		m3d               *capm3.Metal3Data
		poolName          string
		poolAddresses     map[string]bool
		ipClaim           *ipamv1.IPClaim
		expectError       bool
		expectRequeue     bool
		expectedAddresses map[string]bool
	}

	DescribeTable("Test releaseAddressFromPool",
		func(tc testCaseReleaseAddressFromPool) {
			objects := []client.Object{}
			if tc.ipClaim != nil {
				objects = append(objects, tc.ipClaim)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			dataMgr, err := NewDataManager(fakeClient, tc.m3d,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			poolAddresses, requeue, err := dataMgr.releaseAddressFromPool(
				context.TODO(), tc.poolName, tc.poolAddresses,
			)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.expectRequeue {
				Expect(requeue).To(BeTrue())
			} else {
				Expect(requeue).To(BeFalse())
			}
			Expect(poolAddresses).To(Equal(tc.expectedAddresses))
			if tc.ipClaim != nil {
				capm3IPClaim := &ipamv1.IPClaim{}
				poolNamespacedName := types.NamespacedName{
					Name:      tc.m3d.Name,
					Namespace: tc.m3d.Namespace,
				}

				err = dataMgr.client.Get(context.TODO(), poolNamespacedName, capm3IPClaim)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		},
		Entry("Already processed", testCaseReleaseAddressFromPool{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
			},
			poolName: "abc",
			poolAddresses: map[string]bool{
				"abc": true,
			},
			expectedAddresses: map[string]bool{
				"abc": true,
			},
		}),
		Entry("Deletion already attempted", testCaseReleaseAddressFromPool{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
			},
			poolName: "abc",
			poolAddresses: map[string]bool{
				"abc": false,
			},
			expectedAddresses: map[string]bool{
				"abc": false,
			},
		}),
		Entry("IPClaim not found", testCaseReleaseAddressFromPool{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
			},
			poolName:      "abc",
			poolAddresses: map[string]bool{},
			expectedAddresses: map[string]bool{
				"abc": true,
			},
		}),
		Entry("IPPool without ownerref", testCaseReleaseAddressFromPool{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: capm3.GroupVersion.String(),
				},
			},
			poolName: "abc",
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc-abc",
					Namespace: namespaceName,
				},
			},
			expectedAddresses: map[string]bool{
				"abc": true,
			},
		}),
	)

	type testCaseRenderNetworkData struct {
		m3d            *capm3.Metal3Data
		m3dt           *capm3.Metal3DataTemplate
		bmh            *bmo.BareMetalHost
		poolAddresses  map[string]addressFromPool
		expectError    bool
		expectedOutput map[string][]interface{}
	}

	DescribeTable("Test renderNetworkData",
		func(tc testCaseRenderNetworkData) {
			result, err := renderNetworkData(tc.m3d, tc.m3dt, tc.bmh, tc.poolAddresses)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			output := map[string][]interface{}{}
			err = yaml.Unmarshal(result, output)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(tc.expectedOutput))
		},
		Entry("Full example", testCaseRenderNetworkData{
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{
					Index: 2,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				Spec: capm3.Metal3DataTemplateSpec{
					NetworkData: &capm3.NetworkData{
						Links: capm3.NetworkDataLink{
							Ethernets: []capm3.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &capm3.NetworkLinkEthernetMac{
										String: pointer.StringPtr("XX:XX:XX:XX:XX:XX"),
									},
								},
							},
						},
						Networks: capm3.NetworkDataNetwork{
							IPv4: []capm3.NetworkDataIPv4{
								{
									ID:                  "abc",
									Link:                "def",
									IPAddressFromIPPool: "abc",
									Routes: []capm3.NetworkDataRoutev4{
										{
											Network: "10.0.0.0",
											Prefix:  16,
											Gateway: capm3.NetworkGatewayv4{
												String: (*ipamv1.IPAddressv4Str)(pointer.StringPtr("192.168.1.1")),
											},
											Services: capm3.NetworkDataServicev4{
												DNS: []ipamv1.IPAddressv4Str{
													ipamv1.IPAddressv4Str("8.8.8.8"),
												},
											},
										},
									},
								},
							},
						},
						Services: capm3.NetworkDataService{
							DNS: []ipamv1.IPAddressStr{
								ipamv1.IPAddressStr("8.8.8.8"),
								ipamv1.IPAddressStr("2001::8888"),
							},
						},
					},
				},
			},
			poolAddresses: map[string]addressFromPool{
				"abc": {
					address: "192.168.0.14",
					prefix:  24,
				},
			},
			expectedOutput: map[string][]interface{}{
				"services": {
					map[interface{}]interface{}{
						"type":    "dns",
						"address": "8.8.8.8",
					},
					map[interface{}]interface{}{
						"type":    "dns",
						"address": "2001::8888",
					},
				},
				"links": {
					map[interface{}]interface{}{
						"type":                 "phy",
						"id":                   "eth0",
						"mtu":                  1500,
						"ethernet_mac_address": "XX:XX:XX:XX:XX:XX",
					},
				},
				"networks": {
					map[interface{}]interface{}{
						"ip_address": "192.168.0.14",
						"routes": []interface{}{
							map[interface{}]interface{}{
								"network": "10.0.0.0",
								"netmask": "255.255.0.0",
								"gateway": "192.168.1.1",
								"services": []interface{}{
									map[interface{}]interface{}{
										"type":    "dns",
										"address": "8.8.8.8",
									},
								},
							},
						},
						"type":    "ipv4",
						"id":      "abc",
						"link":    "def",
						"netmask": "255.255.255.0",
					},
				},
			},
		}),
		Entry("Error in link", testCaseRenderNetworkData{
			m3dt: &capm3.Metal3DataTemplate{
				Spec: capm3.Metal3DataTemplateSpec{
					NetworkData: &capm3.NetworkData{
						Links: capm3.NetworkDataLink{
							Ethernets: []capm3.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &capm3.NetworkLinkEthernetMac{
										FromHostInterface: pointer.StringPtr("eth0"),
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("Address error", testCaseRenderNetworkData{
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{
					Index: 2,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				Spec: capm3.Metal3DataTemplateSpec{
					NetworkData: &capm3.NetworkData{
						Networks: capm3.NetworkDataNetwork{
							IPv4: []capm3.NetworkDataIPv4{
								{
									ID:                  "abc",
									Link:                "def",
									IPAddressFromIPPool: "abc",
								},
							},
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("Empty", testCaseRenderNetworkData{
			m3dt: &capm3.Metal3DataTemplate{
				Spec: capm3.Metal3DataTemplateSpec{
					NetworkData: nil,
				},
			},
			expectedOutput: map[string][]interface{}{},
		}),
	)

	It("Test renderNetworkServices", func() {
		services := capm3.NetworkDataService{
			DNS: []ipamv1.IPAddressStr{
				(ipamv1.IPAddressStr)("8.8.8.8"),
				(ipamv1.IPAddressStr)("2001::8888"),
			},
			DNSFromIPPool: pointer.StringPtr("pool1"),
		}
		poolAddresses := map[string]addressFromPool{
			"pool1": {
				dnsServers: []ipamv1.IPAddressStr{
					ipamv1.IPAddressStr("8.8.4.4"),
				},
			},
		}
		expectedOutput := []interface{}{
			map[string]interface{}{
				"type":    "dns",
				"address": ipamv1.IPAddressStr("8.8.8.8"),
			},
			map[string]interface{}{
				"type":    "dns",
				"address": ipamv1.IPAddressStr("2001::8888"),
			},
			map[string]interface{}{
				"type":    "dns",
				"address": ipamv1.IPAddressStr("8.8.4.4"),
			},
		}
		result, err := renderNetworkServices(services, poolAddresses)
		Expect(result).To(Equal(expectedOutput))
		Expect(err).NotTo(HaveOccurred())
	})

	type testCaseRenderNetworkLinks struct {
		links          capm3.NetworkDataLink
		bmh            *bmo.BareMetalHost
		expectError    bool
		expectedOutput []interface{}
	}

	DescribeTable("Test renderNetworkLinks",
		func(tc testCaseRenderNetworkLinks) {
			result, err := renderNetworkLinks(tc.links, tc.bmh)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(tc.expectedOutput))
		},
		Entry("Ethernet, MAC from string", testCaseRenderNetworkLinks{
			links: capm3.NetworkDataLink{
				Ethernets: []capm3.NetworkDataLinkEthernet{
					{
						Type: "phy",
						Id:   "eth0",
						MTU:  1500,
						MACAddress: &capm3.NetworkLinkEthernetMac{
							String: pointer.StringPtr("XX:XX:XX:XX:XX:XX"),
						},
					},
				},
			},
			expectedOutput: []interface{}{
				map[string]interface{}{
					"type":                 "phy",
					"id":                   "eth0",
					"mtu":                  1500,
					"ethernet_mac_address": "XX:XX:XX:XX:XX:XX",
				},
			},
		}),
		Entry("Ethernet, MAC error", testCaseRenderNetworkLinks{
			links: capm3.NetworkDataLink{
				Ethernets: []capm3.NetworkDataLinkEthernet{
					{
						Type: "phy",
						Id:   "eth0",
						MTU:  1500,
						MACAddress: &capm3.NetworkLinkEthernetMac{
							FromHostInterface: pointer.StringPtr("eth2"),
						},
					},
				},
			},
			bmh: &bmo.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bmh-abc",
				},
				Status: bmo.BareMetalHostStatus{},
			},
			expectError: true,
		}),
		Entry("Bond, MAC from string", testCaseRenderNetworkLinks{
			links: capm3.NetworkDataLink{
				Bonds: []capm3.NetworkDataLinkBond{
					{
						BondMode: "802.3ad",
						Id:       "bond0",
						MTU:      1500,
						MACAddress: &capm3.NetworkLinkEthernetMac{
							String: pointer.StringPtr("XX:XX:XX:XX:XX:XX"),
						},
						BondLinks: []string{"eth0"},
					},
				},
			},
			expectedOutput: []interface{}{
				map[string]interface{}{
					"type":                 "bond",
					"id":                   "bond0",
					"mtu":                  1500,
					"ethernet_mac_address": "XX:XX:XX:XX:XX:XX",
					"bond_mode":            "802.3ad",
					"bond_links":           []string{"eth0"},
				},
			},
		}),
		Entry("Bond, MAC error", testCaseRenderNetworkLinks{
			links: capm3.NetworkDataLink{
				Bonds: []capm3.NetworkDataLinkBond{
					{
						BondMode: "802.3ad",
						Id:       "bond0",
						MTU:      1500,
						MACAddress: &capm3.NetworkLinkEthernetMac{
							FromHostInterface: pointer.StringPtr("eth2"),
						},
						BondLinks: []string{"eth0"},
					},
				},
			},
			bmh: &bmo.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bmh-abc",
				},
				Status: bmo.BareMetalHostStatus{},
			},
			expectError: true,
		}),
		Entry("Vlan, MAC from string", testCaseRenderNetworkLinks{
			links: capm3.NetworkDataLink{
				Vlans: []capm3.NetworkDataLinkVlan{
					{
						VlanID: 2222,
						Id:     "bond0",
						MTU:    1500,
						MACAddress: &capm3.NetworkLinkEthernetMac{
							String: pointer.StringPtr("XX:XX:XX:XX:XX:XX"),
						},
						VlanLink: "eth0",
					},
				},
			},
			expectedOutput: []interface{}{
				map[string]interface{}{
					"vlan_mac_address": "XX:XX:XX:XX:XX:XX",
					"vlan_id":          2222,
					"vlan_link":        "eth0",
					"type":             "vlan",
					"id":               "bond0",
					"mtu":              1500,
				},
			},
		}),
		Entry("Vlan, MAC error", testCaseRenderNetworkLinks{
			links: capm3.NetworkDataLink{
				Vlans: []capm3.NetworkDataLinkVlan{
					{
						VlanID: 2222,
						Id:     "bond0",
						MTU:    1500,
						MACAddress: &capm3.NetworkLinkEthernetMac{
							FromHostInterface: pointer.StringPtr("eth2"),
						},
						VlanLink: "eth0",
					},
				},
			},
			bmh: &bmo.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bmh-abc",
				},
				Status: bmo.BareMetalHostStatus{},
			},
			expectError: true,
		}),
	)

	type testCaseRenderNetworkNetworks struct {
		networks       capm3.NetworkDataNetwork
		m3d            *capm3.Metal3Data
		poolAddresses  map[string]addressFromPool
		expectError    bool
		expectedOutput []interface{}
	}

	DescribeTable("Test renderNetworkNetworks",
		func(tc testCaseRenderNetworkNetworks) {
			result, err := renderNetworkNetworks(tc.networks, tc.m3d, tc.poolAddresses)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(tc.expectedOutput))
		},
		Entry("IPv4 network", testCaseRenderNetworkNetworks{
			poolAddresses: map[string]addressFromPool{
				"abc": {
					address: ipamv1.IPAddressStr("192.168.0.14"),
					prefix:  24,
					gateway: ipamv1.IPAddressStr("192.168.1.1"),
				},
			},
			networks: capm3.NetworkDataNetwork{
				IPv4: []capm3.NetworkDataIPv4{
					{
						ID:                  "abc",
						Link:                "def",
						IPAddressFromIPPool: "abc",
						Routes: []capm3.NetworkDataRoutev4{
							{
								Network: "10.0.0.0",
								Prefix:  16,
								Gateway: capm3.NetworkGatewayv4{
									FromIPPool: pointer.StringPtr("abc"),
								},
								Services: capm3.NetworkDataServicev4{
									DNS: []ipamv1.IPAddressv4Str{
										ipamv1.IPAddressv4Str("8.8.8.8"),
									},
								},
							},
						},
					},
				},
			},
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{
					Index: 2,
				},
			},
			expectedOutput: []interface{}{
				map[string]interface{}{
					"ip_address": ipamv1.IPAddressv4Str("192.168.0.14"),
					"routes": []interface{}{
						map[string]interface{}{
							"network": ipamv1.IPAddressv4Str("10.0.0.0"),
							"netmask": ipamv1.IPAddressv4Str("255.255.0.0"),
							"gateway": ipamv1.IPAddressv4Str("192.168.1.1"),
							"services": []interface{}{
								map[string]interface{}{
									"type":    "dns",
									"address": ipamv1.IPAddressv4Str("8.8.8.8"),
								},
							},
						},
					},
					"type":    "ipv4",
					"id":      "abc",
					"link":    "def",
					"netmask": ipamv1.IPAddressv4Str("255.255.255.0"),
				},
			},
		}),
		Entry("IPv4 network, error", testCaseRenderNetworkNetworks{
			networks: capm3.NetworkDataNetwork{
				IPv4: []capm3.NetworkDataIPv4{
					{
						IPAddressFromIPPool: "abc",
					},
				},
			},
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{
					Index: 1000,
				},
			},
			expectError: true,
		}),
		Entry("IPv6 network", testCaseRenderNetworkNetworks{
			poolAddresses: map[string]addressFromPool{
				"abc": {
					address: ipamv1.IPAddressStr("fe80::2001:38"),
					prefix:  96,
					gateway: ipamv1.IPAddressStr("fe80::2001:1"),
				},
			},
			networks: capm3.NetworkDataNetwork{
				IPv6: []capm3.NetworkDataIPv6{
					{
						ID:                  "abc",
						Link:                "def",
						IPAddressFromIPPool: "abc",
						Routes: []capm3.NetworkDataRoutev6{
							{
								Network: "2001::",
								Prefix:  64,
								Gateway: capm3.NetworkGatewayv6{
									FromIPPool: pointer.StringPtr("abc"),
								},
								Services: capm3.NetworkDataServicev6{
									DNS: []ipamv1.IPAddressv6Str{
										ipamv1.IPAddressv6Str("2001::8888"),
									},
								},
							},
						},
					},
				},
			},
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{
					Index: 2,
				},
			},
			expectedOutput: []interface{}{
				map[string]interface{}{
					"ip_address": ipamv1.IPAddressv6Str("fe80::2001:38"),
					"routes": []interface{}{
						map[string]interface{}{
							"network": ipamv1.IPAddressv6Str("2001::"),
							"netmask": ipamv1.IPAddressv6Str("ffff:ffff:ffff:ffff::"),
							"gateway": ipamv1.IPAddressv6Str("fe80::2001:1"),
							"services": []interface{}{
								map[string]interface{}{
									"type":    "dns",
									"address": ipamv1.IPAddressv6Str("2001::8888"),
								},
							},
						},
					},
					"type":    "ipv6",
					"id":      "abc",
					"link":    "def",
					"netmask": ipamv1.IPAddressv6Str("ffff:ffff:ffff:ffff:ffff:ffff::"),
				},
			},
		}),
		Entry("IPv6 network error", testCaseRenderNetworkNetworks{
			networks: capm3.NetworkDataNetwork{
				IPv6: []capm3.NetworkDataIPv6{
					{
						IPAddressFromIPPool: "abc",
					},
				},
			},
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{
					Index: 10000,
				},
			},
			expectError: true,
		}),
		Entry("IPv4 DHCP", testCaseRenderNetworkNetworks{
			networks: capm3.NetworkDataNetwork{
				IPv4DHCP: []capm3.NetworkDataIPv4DHCP{
					{
						ID:   "abc",
						Link: "def",
						Routes: []capm3.NetworkDataRoutev4{
							{
								Network: "10.0.0.0",
								Prefix:  16,
								Gateway: capm3.NetworkGatewayv4{
									String: (*ipamv1.IPAddressv4Str)(pointer.StringPtr("192.168.1.1")),
								},
								Services: capm3.NetworkDataServicev4{
									DNS: []ipamv1.IPAddressv4Str{
										ipamv1.IPAddressv4Str("8.8.8.8"),
									},
								},
							},
						},
					},
				},
			},
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{
					Index: 2,
				},
			},
			expectedOutput: []interface{}{
				map[string]interface{}{
					"routes": []interface{}{
						map[string]interface{}{
							"network": ipamv1.IPAddressv4Str("10.0.0.0"),
							"netmask": ipamv1.IPAddressv4Str("255.255.0.0"),
							"gateway": ipamv1.IPAddressv4Str("192.168.1.1"),
							"services": []interface{}{
								map[string]interface{}{
									"type":    "dns",
									"address": ipamv1.IPAddressv4Str("8.8.8.8"),
								},
							},
						},
					},
					"type": "ipv4_dhcp",
					"id":   "abc",
					"link": "def",
				},
			},
		}),
		Entry("IPv6 DHCP", testCaseRenderNetworkNetworks{
			networks: capm3.NetworkDataNetwork{
				IPv6DHCP: []capm3.NetworkDataIPv6DHCP{
					{
						ID:   "abc",
						Link: "def",
						Routes: []capm3.NetworkDataRoutev6{
							{
								Network: "2001::",
								Prefix:  64,
								Gateway: capm3.NetworkGatewayv6{
									String: (*ipamv1.IPAddressv6Str)(pointer.StringPtr("fe80::2001:1")),
								},
								Services: capm3.NetworkDataServicev6{
									DNS: []ipamv1.IPAddressv6Str{
										ipamv1.IPAddressv6Str("2001::8888"),
									},
								},
							},
						},
					},
				},
			},
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{
					Index: 2,
				},
			},
			expectedOutput: []interface{}{
				map[string]interface{}{
					"routes": []interface{}{
						map[string]interface{}{
							"network": ipamv1.IPAddressv6Str("2001::"),
							"netmask": ipamv1.IPAddressv6Str("ffff:ffff:ffff:ffff::"),
							"gateway": ipamv1.IPAddressv6Str("fe80::2001:1"),
							"services": []interface{}{
								map[string]interface{}{
									"type":    "dns",
									"address": ipamv1.IPAddressv6Str("2001::8888"),
								},
							},
						},
					},
					"type": "ipv6_dhcp",
					"id":   "abc",
					"link": "def",
				},
			},
		}),
		Entry("IPv6 SLAAC", testCaseRenderNetworkNetworks{
			networks: capm3.NetworkDataNetwork{
				IPv6SLAAC: []capm3.NetworkDataIPv6DHCP{
					{
						ID:   "abc",
						Link: "def",
						Routes: []capm3.NetworkDataRoutev6{
							{
								Network: "2001::",
								Prefix:  64,
								Gateway: capm3.NetworkGatewayv6{
									String: (*ipamv1.IPAddressv6Str)(pointer.StringPtr("fe80::2001:1")),
								},
								Services: capm3.NetworkDataServicev6{
									DNS: []ipamv1.IPAddressv6Str{
										ipamv1.IPAddressv6Str("2001::8888"),
									},
								},
							},
						},
					},
				},
			},
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{
					Index: 2,
				},
			},
			expectedOutput: []interface{}{
				map[string]interface{}{
					"routes": []interface{}{
						map[string]interface{}{
							"network": ipamv1.IPAddressv6Str("2001::"),
							"netmask": ipamv1.IPAddressv6Str("ffff:ffff:ffff:ffff::"),
							"gateway": ipamv1.IPAddressv6Str("fe80::2001:1"),
							"services": []interface{}{
								map[string]interface{}{
									"type":    "dns",
									"address": ipamv1.IPAddressv6Str("2001::8888"),
								},
							},
						},
					},
					"type": "ipv6_slaac",
					"id":   "abc",
					"link": "def",
				},
			},
		}),
	)

	It("Test getRoutesv4", func() {
		netRoutes := []capm3.NetworkDataRoutev4{
			{
				Network: "192.168.0.0",
				Prefix:  24,
				Gateway: capm3.NetworkGatewayv4{
					String: (*ipamv1.IPAddressv4Str)(pointer.StringPtr("192.168.1.1")),
				},
			},
			{
				Network: "10.0.0.0",
				Prefix:  16,
				Gateway: capm3.NetworkGatewayv4{
					FromIPPool: pointer.StringPtr("abc"),
				},
				Services: capm3.NetworkDataServicev4{
					DNS: []ipamv1.IPAddressv4Str{
						ipamv1.IPAddressv4Str("8.8.8.8"),
						ipamv1.IPAddressv4Str("8.8.4.4"),
					},
					DNSFromIPPool: pointer.StringPtr("abc"),
				},
			},
		}
		poolAddresses := map[string]addressFromPool{
			"abc": {
				gateway: "192.168.2.1",
				dnsServers: []ipamv1.IPAddressStr{
					"1.1.1.1",
				},
			},
		}
		ExpectedOutput := []interface{}{
			map[string]interface{}{
				"network":  ipamv1.IPAddressv4Str("192.168.0.0"),
				"netmask":  ipamv1.IPAddressv4Str("255.255.255.0"),
				"gateway":  ipamv1.IPAddressv4Str("192.168.1.1"),
				"services": []interface{}{},
			},
			map[string]interface{}{
				"network": ipamv1.IPAddressv4Str("10.0.0.0"),
				"netmask": ipamv1.IPAddressv4Str("255.255.0.0"),
				"gateway": ipamv1.IPAddressv4Str("192.168.2.1"),
				"services": []interface{}{
					map[string]interface{}{
						"type":    "dns",
						"address": ipamv1.IPAddressv4Str("8.8.8.8"),
					},
					map[string]interface{}{
						"type":    "dns",
						"address": ipamv1.IPAddressv4Str("8.8.4.4"),
					},
					map[string]interface{}{
						"type":    "dns",
						"address": ipamv1.IPAddressStr("1.1.1.1"),
					},
				},
			},
		}
		output, err := getRoutesv4(netRoutes, poolAddresses)
		Expect(output).To(Equal(ExpectedOutput))
		Expect(err).NotTo(HaveOccurred())
		_, err = getRoutesv4(netRoutes, map[string]addressFromPool{})
		Expect(err).To(HaveOccurred())
	})

	It("Test getRoutesv6", func() {
		netRoutes := []capm3.NetworkDataRoutev6{
			{
				Network: "2001::0",
				Prefix:  96,
				Gateway: capm3.NetworkGatewayv6{
					String: (*ipamv1.IPAddressv6Str)(pointer.StringPtr("2001::1")),
				},
			},
			{
				Network: "fe80::0",
				Prefix:  64,
				Gateway: capm3.NetworkGatewayv6{
					FromIPPool: pointer.StringPtr("abc"),
				},
				Services: capm3.NetworkDataServicev6{
					DNS: []ipamv1.IPAddressv6Str{
						ipamv1.IPAddressv6Str("fe80:2001::8888"),
						ipamv1.IPAddressv6Str("fe80:2001::8844"),
					},
					DNSFromIPPool: pointer.StringPtr("abc"),
				},
			},
		}
		poolAddresses := map[string]addressFromPool{
			"abc": {
				gateway: "fe80::1",
				dnsServers: []ipamv1.IPAddressStr{
					"fe80:2001::1111",
				},
			},
		}
		ExpectedOutput := []interface{}{
			map[string]interface{}{
				"network":  ipamv1.IPAddressv6Str("2001::0"),
				"netmask":  ipamv1.IPAddressv6Str("ffff:ffff:ffff:ffff:ffff:ffff::"),
				"gateway":  ipamv1.IPAddressv6Str("2001::1"),
				"services": []interface{}{},
			},
			map[string]interface{}{
				"network": ipamv1.IPAddressv6Str("fe80::0"),
				"netmask": ipamv1.IPAddressv6Str("ffff:ffff:ffff:ffff::"),
				"gateway": ipamv1.IPAddressv6Str("fe80::1"),
				"services": []interface{}{
					map[string]interface{}{
						"type":    "dns",
						"address": ipamv1.IPAddressv6Str("fe80:2001::8888"),
					},
					map[string]interface{}{
						"type":    "dns",
						"address": ipamv1.IPAddressv6Str("fe80:2001::8844"),
					},
					map[string]interface{}{
						"type":    "dns",
						"address": ipamv1.IPAddressStr("fe80:2001::1111"),
					},
				},
			},
		}
		output, err := getRoutesv6(netRoutes, poolAddresses)
		Expect(output).To(Equal(ExpectedOutput))
		Expect(err).NotTo(HaveOccurred())
		_, err = getRoutesv6(netRoutes, map[string]addressFromPool{})
		Expect(err).To(HaveOccurred())
	})

	type testCaseTranslateMask struct {
		mask         int
		ipv4         bool
		expectedMask interface{}
	}

	DescribeTable("Test translateMask",
		func(tc testCaseTranslateMask) {
			Expect(translateMask(tc.mask, tc.ipv4)).To(Equal(tc.expectedMask))
		},
		Entry("IPv4 mask 24", testCaseTranslateMask{
			mask:         24,
			ipv4:         true,
			expectedMask: ipamv1.IPAddressv4Str("255.255.255.0"),
		}),
		Entry("IPv4 mask 16", testCaseTranslateMask{
			mask:         16,
			ipv4:         true,
			expectedMask: ipamv1.IPAddressv4Str("255.255.0.0"),
		}),
		Entry("IPv6 mask 64", testCaseTranslateMask{
			mask:         64,
			expectedMask: ipamv1.IPAddressv6Str("ffff:ffff:ffff:ffff::"),
		}),
		Entry("IPv6 mask 96", testCaseTranslateMask{
			mask:         96,
			expectedMask: ipamv1.IPAddressv6Str("ffff:ffff:ffff:ffff:ffff:ffff::"),
		}),
	)

	type testCaseGetLinkMacAddress struct {
		mac         *capm3.NetworkLinkEthernetMac
		bmh         *bmo.BareMetalHost
		expectError bool
		expectedMAC string
	}

	DescribeTable("Test getLinkMacAddress",
		func(tc testCaseGetLinkMacAddress) {
			result, err := getLinkMacAddress(tc.mac, tc.bmh)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(tc.expectedMAC))
		},
		Entry("String", testCaseGetLinkMacAddress{
			mac: &capm3.NetworkLinkEthernetMac{
				String: pointer.StringPtr("XX:XX:XX:XX:XX:XX"),
			},
			expectedMAC: "XX:XX:XX:XX:XX:XX",
		}),
		Entry("from host interface", testCaseGetLinkMacAddress{
			mac: &capm3.NetworkLinkEthernetMac{
				FromHostInterface: pointer.StringPtr("eth1"),
			},
			bmh: &bmo.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bmh-abc",
				},
				Status: bmo.BareMetalHostStatus{
					HardwareDetails: &bmo.HardwareDetails{
						NIC: []bmo.NIC{
							{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// Check if empty value cause failure
							{},
							{
								Name: "eth1",
								MAC:  "XX:XX:XX:XX:XX:YY",
							},
						},
					},
				},
			},
			expectedMAC: "XX:XX:XX:XX:XX:YY",
		}),
		Entry("from host interface not found", testCaseGetLinkMacAddress{
			mac: &capm3.NetworkLinkEthernetMac{
				FromHostInterface: pointer.StringPtr("eth2"),
			},
			bmh: &bmo.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bmh-abc",
				},
				Status: bmo.BareMetalHostStatus{
					HardwareDetails: &bmo.HardwareDetails{
						NIC: []bmo.NIC{
							{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// Check if empty value cause failure
							{},
							{
								Name: "eth1",
								MAC:  "XX:XX:XX:XX:XX:YY",
							},
						},
					},
				},
			},
			expectError: true,
		}),
	)

	type testCaseRenderMetaData struct {
		m3d              *capm3.Metal3Data
		m3dt             *capm3.Metal3DataTemplate
		m3m              *capm3.Metal3Machine
		machine          *clusterv1.Machine
		bmh              *bmo.BareMetalHost
		poolAddresses    map[string]addressFromPool
		expectedMetaData map[string]string
		expectError      bool
	}

	DescribeTable("Test renderMetaData",
		func(tc testCaseRenderMetaData) {
			resultBytes, err := renderMetaData(tc.m3d, tc.m3dt, tc.m3m, tc.machine,
				tc.bmh, tc.poolAddresses,
			)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			var outputMap map[string]string
			err = yaml.Unmarshal(resultBytes, &outputMap)
			Expect(err).NotTo(HaveOccurred())
			Expect(outputMap).To(Equal(tc.expectedMetaData))
		},
		Entry("Empty", testCaseRenderMetaData{
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "datatemplate-abc",
				},
			},
			expectedMetaData: nil,
		}),
		Entry("Full example", testCaseRenderMetaData{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "data-abc",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataSpec{
					Index: 2,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "datatemplate-abc",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						Strings: []capm3.MetaDataString{
							{
								Key:   "String-1",
								Value: "String-1",
							},
						},
						ObjectNames: []capm3.MetaDataObjectName{
							{
								Key:    "ObjectName-1",
								Object: "machine",
							},
							{
								Key:    "ObjectName-2",
								Object: "metal3machine",
							},
							{
								Key:    "ObjectName-3",
								Object: "baremetalhost",
							},
						},
						Namespaces: []capm3.MetaDataNamespace{
							{
								Key: "Namespace-1",
							},
						},
						Indexes: []capm3.MetaDataIndex{
							{
								Key:    "Index-1",
								Offset: 10,
								Step:   2,
								Prefix: "abc",
								Suffix: "def",
							},
							{
								Key: "Index-2",
							},
						},
						IPAddressesFromPool: []capm3.FromPool{
							{
								Key:  "Address-1",
								Name: "abcd",
							},
							{
								Key:  "Address-2",
								Name: "abcd",
							},
							{
								Key:  "Address-3",
								Name: "bcde",
							},
						},
						PrefixesFromPool: []capm3.FromPool{
							{
								Key:  "Prefix-1",
								Name: "abcd",
							},
							{
								Key:  "Prefix-2",
								Name: "abcd",
							},
							{
								Key:  "Prefix-3",
								Name: "bcde",
							},
						},
						GatewaysFromPool: []capm3.FromPool{
							{
								Key:  "Gateway-1",
								Name: "abcd",
							},
							{
								Key:  "Gateway-2",
								Name: "abcd",
							},
							{
								Key:  "Gateway-3",
								Name: "bcde",
							},
						},
						FromHostInterfaces: []capm3.MetaDataHostInterface{
							{
								Key:       "Mac-1",
								Interface: "eth1",
							},
						},
						FromLabels: []capm3.MetaDataFromLabel{
							{
								Key:    "Label-1",
								Object: "metal3machine",
								Label:  "Doesnotexist",
							},
							{
								Key:    "Label-2",
								Object: "metal3machine",
								Label:  "Empty",
							},
							{
								Key:    "Label-3",
								Object: "metal3machine",
								Label:  "M3M",
							},
							{
								Key:    "Label-4",
								Object: "machine",
								Label:  "Machine",
							},
							{
								Key:    "Label-5",
								Object: "baremetalhost",
								Label:  "BMH",
							},
						},
						FromAnnotations: []capm3.MetaDataFromAnnotation{
							{
								Key:        "Annotation-1",
								Object:     "metal3machine",
								Annotation: "Doesnotexist",
							},
							{
								Key:        "Annotation-2",
								Object:     "metal3machine",
								Annotation: "Empty",
							},
							{
								Key:        "Annotation-3",
								Object:     "metal3machine",
								Annotation: "M3M",
							},
							{
								Key:        "Annotation-4",
								Object:     "machine",
								Annotation: "Machine",
							},
							{
								Key:        "Annotation-5",
								Object:     "baremetalhost",
								Annotation: "BMH",
							},
						},
					},
				},
			},
			m3m: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metal3machine-abc",
					Namespace: namespaceName,
					Labels: map[string]string{
						"M3M":   "Metal3MachineLabel",
						"Empty": "",
					},
					UID: m3muid,
					Annotations: map[string]string{
						"M3M":   "Metal3MachineAnnotation",
						"Empty": "",
					},
				},
			},
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machine-abc",
					Labels: map[string]string{
						"Machine": "MachineLabel",
					},
					Annotations: map[string]string{
						"Machine": "MachineAnnotation",
					},
				},
			},
			bmh: &bmo.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bmh-abc",
					Namespace: namespaceName,
					Labels: map[string]string{
						"BMH": "BMHLabel",
					},
					Annotations: map[string]string{
						"BMH": "BMHAnnotation",
					},
					UID: bmhuid,
				},
				Status: bmo.BareMetalHostStatus{
					HardwareDetails: &bmo.HardwareDetails{
						NIC: []bmo.NIC{
							{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// To check if empty value cause failure
							{},
							{
								Name: "eth1",
								MAC:  "XX:XX:XX:XX:XX:YY",
							},
						},
					},
				},
			},
			poolAddresses: map[string]addressFromPool{
				"abcd": {
					address: "192.168.0.14",
					prefix:  25,
					gateway: "192.168.0.1",
				},
				"bcde": {
					address: "192.168.1.14",
					prefix:  26,
					gateway: "192.168.1.1",
				},
			},
			expectedMetaData: map[string]string{
				"String-1":     "String-1",
				"providerid":   fmt.Sprintf("%s/%s/%s", namespaceName, bmhuid, m3muid),
				"ObjectName-1": "machine-abc",
				"ObjectName-2": "metal3machine-abc",
				"ObjectName-3": "bmh-abc",
				"Namespace-1":  namespaceName,
				"Index-1":      "abc14def",
				"Index-2":      "2",
				"Address-1":    "192.168.0.14",
				"Address-2":    "192.168.0.14",
				"Address-3":    "192.168.1.14",
				"Gateway-1":    "192.168.0.1",
				"Gateway-2":    "192.168.0.1",
				"Gateway-3":    "192.168.1.1",
				"Prefix-1":     "25",
				"Prefix-2":     "25",
				"Prefix-3":     "26",
				"Mac-1":        "XX:XX:XX:XX:XX:YY",
				"Label-1":      "",
				"Label-2":      "",
				"Label-3":      "Metal3MachineLabel",
				"Label-4":      "MachineLabel",
				"Label-5":      "BMHLabel",
				"Annotation-1": "",
				"Annotation-2": "",
				"Annotation-3": "Metal3MachineAnnotation",
				"Annotation-4": "MachineAnnotation",
				"Annotation-5": "BMHAnnotation",
			},
		}),
		Entry("Interface absent", testCaseRenderMetaData{
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "datatemplate-abc",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						FromHostInterfaces: []capm3.MetaDataHostInterface{
							{
								Key:       "Mac-1",
								Interface: "eth2",
							},
						},
					},
				},
			},
			bmh: &bmo.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bmh-abc",
				},
				Status: bmo.BareMetalHostStatus{
					HardwareDetails: &bmo.HardwareDetails{
						NIC: []bmo.NIC{
							{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// Check if empty value cause failure
							{},
							{
								Name: "eth1",
								MAC:  "XX:XX:XX:XX:XX:YY",
							},
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("IP missing", testCaseRenderMetaData{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "data-abc",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataSpec{
					Index: 2,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "datatemplate-abc",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						IPAddressesFromPool: []capm3.FromPool{
							{
								Key:  "Address-1",
								Name: "abc",
							},
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("Prefix missing", testCaseRenderMetaData{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "data-abc",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataSpec{
					Index: 2,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "datatemplate-abc",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						PrefixesFromPool: []capm3.FromPool{
							{
								Key:  "Address-1",
								Name: "abc",
							},
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("Gateway missing", testCaseRenderMetaData{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "data-abc",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataSpec{
					Index: 2,
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "datatemplate-abc",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						GatewaysFromPool: []capm3.FromPool{
							{
								Key:  "Address-1",
								Name: "abc",
							},
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("Wrong object in name", testCaseRenderMetaData{
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "datatemplate-abc",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						ObjectNames: []capm3.MetaDataObjectName{
							{
								Key:    "ObjectName-3",
								Object: "baremetalhost2",
							},
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("Wrong object in Label", testCaseRenderMetaData{
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "datatemplate-abc",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						FromLabels: []capm3.MetaDataFromLabel{
							{
								Key:    "ObjectName-3",
								Object: "baremetalhost2",
								Label:  "abc",
							},
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("Wrong object in Annotation", testCaseRenderMetaData{
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "datatemplate-abc",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						FromAnnotations: []capm3.MetaDataFromAnnotation{
							{
								Key:        "ObjectName-3",
								Object:     "baremetalhost2",
								Annotation: "abc",
							},
						},
					},
				},
			},
			expectError: true,
		}),
	)

	type testCaseGetBMHMacByName struct {
		bmh         *bmo.BareMetalHost
		name        string
		expectError bool
		expectedMAC string
	}

	DescribeTable("Test getBMHMacByName",
		func(tc testCaseGetBMHMacByName) {
			result, err := getBMHMacByName(tc.name, tc.bmh)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(tc.expectedMAC))
			}
		},
		Entry("No hardware details", testCaseGetBMHMacByName{
			bmh: &bmo.BareMetalHost{
				Status: bmo.BareMetalHostStatus{},
			},
			name:        "eth1",
			expectError: true,
		}),
		Entry("No Nics detail", testCaseGetBMHMacByName{
			bmh: &bmo.BareMetalHost{
				Status: bmo.BareMetalHostStatus{
					HardwareDetails: &bmo.HardwareDetails{},
				},
			},
			name:        "eth1",
			expectError: true,
		}),
		Entry("Empty nic list", testCaseGetBMHMacByName{
			bmh: &bmo.BareMetalHost{
				Status: bmo.BareMetalHostStatus{
					HardwareDetails: &bmo.HardwareDetails{
						NIC: []bmo.NIC{},
					},
				},
			},
			name:        "eth1",
			expectError: true,
		}),
		Entry("Nic not found", testCaseGetBMHMacByName{
			bmh: &bmo.BareMetalHost{
				Status: bmo.BareMetalHostStatus{
					HardwareDetails: &bmo.HardwareDetails{
						NIC: []bmo.NIC{
							{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
						},
					},
				},
			},
			name:        "eth1",
			expectError: true,
		}),
		Entry("Nic found", testCaseGetBMHMacByName{
			bmh: &bmo.BareMetalHost{
				Status: bmo.BareMetalHostStatus{
					HardwareDetails: &bmo.HardwareDetails{
						NIC: []bmo.NIC{
							{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// Check if empty value cause failure
							{},
							{
								Name: "eth1",
								MAC:  "XX:XX:XX:XX:XX:YY",
							},
						},
					},
				},
			},
			name:        "eth1",
			expectedMAC: "XX:XX:XX:XX:XX:YY",
		}),
		Entry("Nic found, Empty Mac", testCaseGetBMHMacByName{
			bmh: &bmo.BareMetalHost{
				Status: bmo.BareMetalHostStatus{
					HardwareDetails: &bmo.HardwareDetails{
						NIC: []bmo.NIC{
							{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// Check if empty value cause failure
							{},
							{
								Name: "eth1",
							},
						},
					},
				},
			},
			name:        "eth1",
			expectedMAC: "",
		}),
	)

	type testCaseGetM3Machine struct {
		Machine       *capm3.Metal3Machine
		Data          *capm3.Metal3Data
		DataTemplate  *capm3.Metal3DataTemplate
		DataClaim     *capm3.Metal3DataClaim
		ExpectError   bool
		ExpectRequeue bool
		ExpectEmpty   bool
	}

	DescribeTable("Test getM3Machine",
		func(tc testCaseGetM3Machine) {
			fakeClient := k8sClient
			if tc.Machine != nil {
				err := fakeClient.Create(context.TODO(), tc.Machine)
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.DataClaim != nil {
				err := fakeClient.Create(context.TODO(), tc.DataClaim)
				Expect(err).NotTo(HaveOccurred())
			}

			machineMgr, err := NewDataManager(fakeClient, tc.Data,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			result, err := machineMgr.getM3Machine(context.TODO(), tc.DataTemplate)
			if tc.ExpectError || tc.ExpectRequeue {
				Expect(err).To(HaveOccurred())
				if tc.ExpectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
				if tc.ExpectEmpty {
					Expect(result).To(BeNil())
				} else {
					Expect(result).NotTo(BeNil())
				}
			}
			if tc.Machine != nil {
				err = fakeClient.Delete(context.TODO(), tc.Machine)
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.DataClaim != nil {
				err = fakeClient.Delete(context.TODO(), tc.DataClaim)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Object does not exist", testCaseGetM3Machine{
			Data: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Claim: *testObjectReference,
				},
			},
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			ExpectRequeue: true,
		}),
		Entry("Data spec unset", testCaseGetM3Machine{
			Data:        &capm3.Metal3Data{},
			ExpectError: true,
		}),
		Entry("Data Spec name unset", testCaseGetM3Machine{
			Data: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{
					Claim: corev1.ObjectReference{},
				},
			},
			ExpectError: true,
		}),
		Entry("Dataclaim Spec ownerref unset", testCaseGetM3Machine{
			Data: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Claim: *testObjectReference,
				},
			},
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMeta,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			ExpectError: true,
		}),
		Entry("M3Machine not found", testCaseGetM3Machine{
			Data: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Claim: *testObjectReference,
				},
			},
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			ExpectRequeue: true,
		}),
		Entry("Object exists", testCaseGetM3Machine{
			Machine: &capm3.Metal3Machine{
				ObjectMeta: testObjectMeta,
			},
			Data: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Claim: *testObjectReference,
				},
			},
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
		}),
		Entry("Object exists, dataTemplate nil", testCaseGetM3Machine{
			Machine: &capm3.Metal3Machine{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: nil,
				},
			},
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
			},
			Data: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Claim: *testObjectReference,
				},
			},
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			ExpectEmpty: true,
		}),
		Entry("Object exists, dataTemplate name mismatch", testCaseGetM3Machine{
			Machine: &capm3.Metal3Machine{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name:      "abcd",
						Namespace: namespaceName,
					},
				},
			},
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
			},
			Data: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Claim: *testObjectReference,
				},
			},
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			ExpectEmpty: true,
		}),
		Entry("Object exists, dataTemplate namespace mismatch", testCaseGetM3Machine{
			Machine: &capm3.Metal3Machine{
				ObjectMeta: testObjectMeta,
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name:      "abc",
						Namespace: "defg",
					},
				},
			},
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
			},
			Data: &capm3.Metal3Data{
				ObjectMeta: testObjectMetaWithOR,
				Spec: capm3.Metal3DataSpec{
					Claim: *testObjectReference,
				},
			},
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
				Spec:       capm3.Metal3DataClaimSpec{},
			},
			ExpectEmpty: true,
		}),
	)
})
