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
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capipamv1beta1 "sigs.k8s.io/cluster-api/api/ipam/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type testCaseEnsureM3Claim struct {
	poolRef          corev1.TypedLocalObjectReference
	ipClaim          *ipamv1.IPClaim
	expectError      bool
	expectFetchAgain bool
	expectClaim      bool
}

var _ = Describe("Metal3Data manager", func() {
	DescribeTable("Test Finalizers",
		func(data *infrav1.Metal3Data) {
			machineMgr, err := NewDataManager(nil, data,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			machineMgr.SetFinalizer()

			Expect(data.ObjectMeta.Finalizers).To(ContainElement(
				infrav1.DataFinalizer,
			))

			machineMgr.UnsetFinalizer()

			Expect(data.ObjectMeta.Finalizers).NotTo(ContainElement(
				infrav1.DataFinalizer,
			))
		},
		Entry("No finalizers", &infrav1.Metal3Data{}),
		Entry("Additional Finalizers", &infrav1.Metal3Data{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"foo"},
			},
		}),
	)

	It("should be able to set and clear errors", func() {
		data := &infrav1.Metal3Data{}
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
		m3d              *infrav1.Metal3Data
		m3dt             *infrav1.Metal3DataTemplate
		m3m              *infrav1.Metal3Machine
		expectError      bool
		expectRequeue    bool
		expectedErrorSet bool
	}

	DescribeTable("Test Reconcile",
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
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
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
			m3d: &infrav1.Metal3Data{
				Spec: infrav1.Metal3DataSpec{},
				Status: infrav1.Metal3DataStatus{
					ErrorMessage: ptr.To("Error Happened"),
				},
			},
		}),
		Entry("requeue error", testCaseReconcile{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, m3duid),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
				},
			},
			expectRequeue: true,
		}),
	)

	type testCaseCreateSecrets struct {
		m3d                 *infrav1.Metal3Data
		m3dt                *infrav1.Metal3DataTemplate
		m3m                 *infrav1.Metal3Machine
		dataClaim           *infrav1.Metal3DataClaim
		machine             *clusterv1beta1.Machine
		bmh                 *bmov1alpha1.BareMetalHost
		metadataSecret      *corev1.Secret
		networkdataSecret   *corev1.Secret
		expectError         bool
		expectRequeue       bool
		expectReady         bool
		expectedMetadata    *string
		expectedNetworkData *string
	}

	DescribeTable("Test createSecrets",
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
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
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
						Name:      metal3machineName + metaDataSuffix,
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
						Name:      metal3machineName + networkDataSuffix,
						Namespace: namespaceName,
					},
					&tmpSecret,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(tmpSecret.Data["networkData"])).To(Equal(*tc.expectedNetworkData))
			}
		},
		Entry("Empty", testCaseCreateSecrets{
			m3d: &infrav1.Metal3Data{
				Spec: infrav1.Metal3DataSpec{},
			},
		}),
		Entry("No Metal3DataTemplate", testCaseCreateSecrets{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, m3duid),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
				},
			},
			expectRequeue: true,
		}),
		Entry("No Metal3Machine in owner refs", testCaseCreateSecrets{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, m3duid),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
					Claim:    *testObjectReference(metal3DataClaimName),
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMeta(metal3DataClaimName, namespaceName, m3dcuid),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			expectError: true,
		}),
		Entry("No Metal3Machine", testCaseCreateSecrets{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
					Claim:    *testObjectReference(metal3DataClaimName),
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			expectRequeue: true,
		}),
		Entry("No Secret needed", testCaseCreateSecrets{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
					Claim:    *testObjectReference(metal3DataClaimName),
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
			},
			m3m: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, m3muid),
				Spec: infrav1.Metal3MachineSpec{
					DataTemplate: testObjectReference(metal3DataTemplateName),
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			expectReady: true,
		}),
		Entry("Machine without datatemplate", testCaseCreateSecrets{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
					Claim:    *testObjectReference(metal3DataClaimName),
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
			},
			m3m: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, m3muid),
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			expectError: true,
		}),
		Entry("secrets exist", testCaseCreateSecrets{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
					Claim:    *testObjectReference(metal3DataClaimName),
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						Strings: []infrav1.MetaDataString{
							{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &infrav1.NetworkData{
						Links: infrav1.NetworkDataLink{
							Ethernets: []infrav1.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &infrav1.NetworkLinkEthernetMac{
										String: ptr.To("12:34:56:78:9A:BC"),
									},
								},
							},
						},
					},
				},
			},
			m3m: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, m3muid),
				Spec: infrav1.Metal3MachineSpec{
					DataTemplate: testObjectReference(metal3DataTemplateName),
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			metadataSecret: &corev1.Secret{
				ObjectMeta: testObjectMeta(metal3machineName+metaDataSuffix, namespaceName, ""),
				Data: map[string][]byte{
					"metaData": []byte("Hello"),
				},
			},
			networkdataSecret: &corev1.Secret{
				ObjectMeta: testObjectMeta(metal3machineName+networkDataSuffix, namespaceName, ""),
				Data: map[string][]byte{
					"networkData": []byte("Bye"),
				},
			},
			expectReady:         true,
			expectedMetadata:    ptr.To("Hello"),
			expectedNetworkData: ptr.To("Bye"),
		}),
		Entry("secrets do not exist", testCaseCreateSecrets{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
					Claim:    *testObjectReference(metal3DataClaimName),
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						Strings: []infrav1.MetaDataString{
							{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &infrav1.NetworkData{
						Links: infrav1.NetworkDataLink{
							Ethernets: []infrav1.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &infrav1.NetworkLinkEthernetMac{
										String: ptr.To("12:34:56:78:9A:BC"),
									},
								},
							},
						},
					},
				},
			},
			m3m: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      metal3machineName,
					Namespace: namespaceName,
					UID:       m3muid,
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       machineName,
							Kind:       "Machine",
							APIVersion: clusterv1beta1.GroupVersion.String(),
						},
					},
					Annotations: map[string]string{
						"metal3.io/BareMetalHost": namespaceName + "/" + baremetalhostName,
					},
				},
				Spec: infrav1.Metal3MachineSpec{
					DataTemplate: testObjectReference(metal3DataTemplateName),
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			machine: &clusterv1beta1.Machine{
				ObjectMeta: testObjectMeta(machineName, namespaceName, muid),
			},
			bmh: &bmov1alpha1.BareMetalHost{
				ObjectMeta: testObjectMeta(baremetalhostName, namespaceName, bmhuid),
			},
			expectReady:         true,
			expectedMetadata:    ptr.To(fmt.Sprintf("String-1: String-1\nproviderid: %s\n", providerid)),
			expectedNetworkData: ptr.To("links:\n- ethernet_mac_address: 12:34:56:78:9A:BC\n  id: eth0\n  mtu: 1500\n  type: phy\nnetworks: []\nservices: []\n"),
		}),
		Entry("No Machine OwnerRef on M3M", testCaseCreateSecrets{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
					Claim:    *testObjectReference(metal3DataClaimName),
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						Strings: []infrav1.MetaDataString{
							{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &infrav1.NetworkData{
						Links: infrav1.NetworkDataLink{
							Ethernets: []infrav1.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &infrav1.NetworkLinkEthernetMac{
										String: ptr.To("12:34:56:78:9A:BC"),
									},
								},
							},
						},
					},
				},
			},
			m3m: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, m3muid),
				Spec: infrav1.Metal3MachineSpec{
					DataTemplate: testObjectReference(metal3DataTemplateName),
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			expectRequeue: true,
		}),
		Entry("secrets do not exist", testCaseCreateSecrets{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
					Claim:    *testObjectReference(metal3DataClaimName),
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						Strings: []infrav1.MetaDataString{
							{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &infrav1.NetworkData{
						Links: infrav1.NetworkDataLink{
							Ethernets: []infrav1.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &infrav1.NetworkLinkEthernetMac{
										String: ptr.To("12:34:56:78:9A:BC"),
									},
								},
							},
						},
					},
				},
			},
			m3m: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      metal3machineName,
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       machineName,
							Kind:       "Machine",
							APIVersion: clusterv1beta1.GroupVersion.String(),
						},
					},
				},
				Spec: infrav1.Metal3MachineSpec{
					DataTemplate: testObjectReference(metal3DataTemplateName),
				},
			},
			machine: &clusterv1beta1.Machine{
				ObjectMeta: testObjectMeta(machineName, namespaceName, muid),
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			expectRequeue: true,
		}),
	)

	type testCaseReleaseLeases struct {
		m3d           *infrav1.Metal3Data
		m3dt          *infrav1.Metal3DataTemplate
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
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Empty spec", testCaseReleaseLeases{
			m3d: &infrav1.Metal3Data{},
		}),
		Entry("M3dt not found", testCaseReleaseLeases{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				Spec: infrav1.Metal3DataSpec{
					Template: corev1.ObjectReference{
						Name: metal3DataTemplateName,
					},
				},
			},
			expectRequeue: true,
		}),
		Entry("M3dt found", testCaseReleaseLeases{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				Spec: infrav1.Metal3DataSpec{
					Template: corev1.ObjectReference{
						Name: metal3DataTemplateName,
					},
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
			},
		}),
	)

	type testCaseGetAddressesFromPool struct {
		m3dtSpec      infrav1.Metal3DataTemplateSpec
		m3IPClaims    []string
		ipClaims      []string
		expectError   bool
		expectRequeue bool
	}

	DescribeTable("Test getAddressesFromPool",
		func(tc testCaseGetAddressesFromPool) {
			objects := []client.Object{}
			for _, claimName := range tc.m3IPClaims {
				claim := &ipamv1.IPClaim{
					ObjectMeta: testObjectMeta(metal3DataName+"-"+claimName, namespaceName, ""),
					Spec: ipamv1.IPClaimSpec{
						Pool: *testObjectReference("abc"),
					},
				}
				objects = append(objects, claim)
			}
			for _, claimName := range tc.ipClaims {
				claim := &capipamv1beta1.IPAddressClaim{
					ObjectMeta: testObjectMeta(metal3DataName+"-"+claimName, namespaceName, ""),
					Spec: capipamv1beta1.IPAddressClaimSpec{
						PoolRef: corev1.TypedLocalObjectReference{
							Name:     "abc",
							APIGroup: ptr.To("ipam.cluster.x-k8s.io"),
							Kind:     "TestPool",
						},
					},
				}
				objects = append(objects, claim)
			}
			m3d := &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: infrav1.GroupVersion.String(),
				},
			}
			m3dt := infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
				Spec:       tc.m3dtSpec,
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
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			expectedPoolAddress := make(map[string]addressFromPool)
			for _, poolName := range tc.m3IPClaims {
				expectedPoolAddress[poolName] = addressFromPool{}
			}
			for _, poolName := range tc.ipClaims {
				expectedPoolAddress[poolName] = addressFromPool{}
			}
			Expect(expectedPoolAddress).To(Equal(poolAddresses))
		},
		Entry("Metadata ok", testCaseGetAddressesFromPool{
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{
					IPAddressesFromPool: []infrav1.FromPool{
						{
							Key:  "Address-1",
							Name: "abcd-1",
						},
					},
					PrefixesFromPool: []infrav1.FromPool{
						{
							Key:  "Prefix-1",
							Name: "abcd-2",
						},
					},
					GatewaysFromPool: []infrav1.FromPool{
						{
							Key:  "Gateway-1",
							Name: "abcd-3",
						},
					},
				},
				NetworkData: &infrav1.NetworkData{
					Networks: infrav1.NetworkDataNetwork{
						IPv4: []infrav1.NetworkDataIPv4{
							{
								IPAddressFromIPPool: "abcd-4",
								Routes: []infrav1.NetworkDataRoutev4{
									{
										Gateway: infrav1.NetworkGatewayv4{
											FromIPPool: ptr.To("abcd-5"),
										},
									},
								},
							},
						},
						IPv6: []infrav1.NetworkDataIPv6{
							{
								IPAddressFromIPPool: "abcd-6",
								Routes: []infrav1.NetworkDataRoutev6{
									{
										Gateway: infrav1.NetworkGatewayv6{
											FromIPPool: ptr.To("abcd-7"),
										},
									},
								},
							},
						},
						IPv4DHCP: []infrav1.NetworkDataIPv4DHCP{
							{
								Routes: []infrav1.NetworkDataRoutev4{
									{
										Gateway: infrav1.NetworkGatewayv4{
											FromIPPool: ptr.To("abcd-8"),
										},
									},
								},
							},
						},
						IPv6DHCP: []infrav1.NetworkDataIPv6DHCP{
							{
								Routes: []infrav1.NetworkDataRoutev6{
									{
										Gateway: infrav1.NetworkGatewayv6{
											FromIPPool: ptr.To("abcd-9"),
										},
									},
								},
							},
						},
						IPv6SLAAC: []infrav1.NetworkDataIPv6DHCP{
							{
								Routes: []infrav1.NetworkDataRoutev6{
									{
										Gateway: infrav1.NetworkGatewayv6{
											FromIPPool: ptr.To("abcd-10"),
										},
									},
								},
							},
						},
					},
				},
			},
			m3IPClaims: []string{
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
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{
					IPAddressesFromPool: []infrav1.FromPool{
						{
							Key:  "Address-1",
							Name: "abcd",
						},
					},
				},
				NetworkData: &infrav1.NetworkData{},
			},
			m3IPClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
		Entry("PrefixesFromPool", testCaseGetAddressesFromPool{
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{
					PrefixesFromPool: []infrav1.FromPool{
						{
							Key:  "Prefix-1",
							Name: "abcd",
						},
					},
				},
				NetworkData: &infrav1.NetworkData{},
			},
			m3IPClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
		Entry("GatewaysFromPool", testCaseGetAddressesFromPool{
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{
					GatewaysFromPool: []infrav1.FromPool{
						{
							Key:  "Gateway-1",
							Name: "abcd",
						},
					},
				},
				NetworkData: &infrav1.NetworkData{},
			},
			m3IPClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
		Entry("IPv4", testCaseGetAddressesFromPool{
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{},
				NetworkData: &infrav1.NetworkData{
					Networks: infrav1.NetworkDataNetwork{
						IPv4: []infrav1.NetworkDataIPv4{
							{
								IPAddressFromIPPool: "abcd-1",
								Routes: []infrav1.NetworkDataRoutev4{
									{
										Gateway: infrav1.NetworkGatewayv4{
											FromIPPool: ptr.To("abcd-2"),
										},
									},
								},
							},
						},
					},
				},
			},
			m3IPClaims: []string{
				"abcd-1",
				"abcd-2",
			},
			expectRequeue: true,
		}),
		Entry("IPv6", testCaseGetAddressesFromPool{
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{},
				NetworkData: &infrav1.NetworkData{
					Networks: infrav1.NetworkDataNetwork{
						IPv6: []infrav1.NetworkDataIPv6{
							{
								IPAddressFromIPPool: "abcd-1",
								Routes: []infrav1.NetworkDataRoutev6{
									{
										Gateway: infrav1.NetworkGatewayv6{
											FromIPPool: ptr.To("abcd-2"),
										},
									},
								},
							},
						},
					},
				},
			},
			m3IPClaims: []string{
				"abcd-1",
				"abcd-2",
			},
			expectRequeue: true,
		}),
		Entry("IPv4DHCP", testCaseGetAddressesFromPool{
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{},
				NetworkData: &infrav1.NetworkData{
					Networks: infrav1.NetworkDataNetwork{
						IPv4DHCP: []infrav1.NetworkDataIPv4DHCP{
							{
								Routes: []infrav1.NetworkDataRoutev4{
									{
										Gateway: infrav1.NetworkGatewayv4{
											FromIPPool: ptr.To("abcd"),
										},
									},
								},
							},
						},
					},
				},
			},
			m3IPClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
		Entry("IPv6DHCP", testCaseGetAddressesFromPool{
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{},
				NetworkData: &infrav1.NetworkData{
					Networks: infrav1.NetworkDataNetwork{
						IPv6DHCP: []infrav1.NetworkDataIPv6DHCP{
							{
								Routes: []infrav1.NetworkDataRoutev6{
									{
										Gateway: infrav1.NetworkGatewayv6{
											FromIPPool: ptr.To("abcd"),
										},
									},
								},
							},
						},
					},
				},
			},
			m3IPClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
		Entry("IPv6SLAAC", testCaseGetAddressesFromPool{
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{},
				NetworkData: &infrav1.NetworkData{
					Networks: infrav1.NetworkDataNetwork{
						IPv6SLAAC: []infrav1.NetworkDataIPv6DHCP{
							{
								Routes: []infrav1.NetworkDataRoutev6{
									{
										Gateway: infrav1.NetworkGatewayv6{
											FromIPPool: ptr.To("abcd"),
										},
									},
								},
							},
						},
					},
				},
			},
			m3IPClaims: []string{
				"abcd",
			},
			expectRequeue: true,
		}),
		Entry("Addresses from CAPI Pool", testCaseGetAddressesFromPool{
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{},
				NetworkData: &infrav1.NetworkData{
					Networks: infrav1.NetworkDataNetwork{
						IPv4: []infrav1.NetworkDataIPv4{
							{
								FromPoolRef: &corev1.TypedLocalObjectReference{
									Name:     "test",
									APIGroup: ptr.To("ipam.cluster.x-k8s.io"),
									Kind:     "TestPool",
								},
							},
						},
						IPv6: []infrav1.NetworkDataIPv6{
							{
								FromPoolRef: &corev1.TypedLocalObjectReference{
									Name:     "test-2",
									APIGroup: ptr.To("ipam.cluster.x-k8s.io"),
									Kind:     "TestPool",
								},
							},
						},
					},
				},
			},
			ipClaims: []string{
				"test",
				"test-2",
			},
			expectRequeue: true,
		}),
	)

	type testCaseReleaseAddressesFromPool struct {
		m3dtSpec      infrav1.Metal3DataTemplateSpec
		m3IPClaims    []string
		ipClaims      []string
		expectError   bool
		expectRequeue bool
	}

	DescribeTable("Test releaseAddressesFromPool",
		func(tc testCaseReleaseAddressesFromPool) {
			objects := []client.Object{}
			for _, poolName := range tc.m3IPClaims {
				objects = append(objects, &ipamv1.IPClaim{
					ObjectMeta: testObjectMeta(metal3DataName+"-"+poolName, namespaceName, ""),
					Spec: ipamv1.IPClaimSpec{
						Pool: *testObjectReference("abc"),
					},
				})
			}
			for _, poolName := range tc.ipClaims {
				objects = append(objects, &capipamv1beta1.IPAddressClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:       metal3DataName + "-" + poolName,
						Namespace:  namespaceName,
						Finalizers: []string{infrav1.DataFinalizer},
					},
					Spec: capipamv1beta1.IPAddressClaimSpec{
						PoolRef: corev1.TypedLocalObjectReference{
							APIGroup: ptr.To("ipam.cluster.x-k8s.io"),
							Kind:     "TestPool",
							Name:     "test",
						},
					},
				})
			}
			m3d := &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: infrav1.GroupVersion.String(),
				},
			}
			m3dt := infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName+"-abc", "", ""),
				Spec:       tc.m3dtSpec,
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
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			for _, poolName := range tc.m3IPClaims {
				capm3IPClaim := &ipamv1.IPClaim{}
				claimNamespacedName := types.NamespacedName{
					Name:      metal3DataName + "-" + poolName,
					Namespace: m3d.Namespace,
				}

				err = dataMgr.client.Get(context.TODO(), claimNamespacedName, capm3IPClaim)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
			for _, poolName := range tc.ipClaims {
				claim := &capipamv1beta1.IPAddressClaim{}
				claimNamespacedName := types.NamespacedName{
					Name:      metal3DataName + "-" + poolName,
					Namespace: m3d.Namespace,
				}

				err = dataMgr.client.Get(context.TODO(), claimNamespacedName, claim)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		},
		Entry("Metadata ok", testCaseReleaseAddressesFromPool{
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{
					IPAddressesFromPool: []infrav1.FromPool{
						{
							Key:  "Address-1",
							Name: "abcd-1",
						},
					},
					PrefixesFromPool: []infrav1.FromPool{
						{
							Key:  "Prefix-1",
							Name: "abcd-2",
						},
					},
					GatewaysFromPool: []infrav1.FromPool{
						{
							Key:  "Gateway-1",
							Name: "abcd-3",
						},
					},
				},
				NetworkData: &infrav1.NetworkData{
					Networks: infrav1.NetworkDataNetwork{
						IPv4: []infrav1.NetworkDataIPv4{
							{
								IPAddressFromIPPool: "abcd-4",
								Routes: []infrav1.NetworkDataRoutev4{
									{
										Gateway: infrav1.NetworkGatewayv4{
											FromIPPool: ptr.To("abcd-5"),
										},
									},
								},
							},
						},
						IPv6: []infrav1.NetworkDataIPv6{
							{
								IPAddressFromIPPool: "abcd-6",
								Routes: []infrav1.NetworkDataRoutev6{
									{
										Gateway: infrav1.NetworkGatewayv6{
											FromIPPool: ptr.To("abcd-7"),
										},
									},
								},
							},
						},
						IPv4DHCP: []infrav1.NetworkDataIPv4DHCP{
							{
								Routes: []infrav1.NetworkDataRoutev4{
									{
										Gateway: infrav1.NetworkGatewayv4{
											FromIPPool: ptr.To("abcd-8"),
										},
									},
								},
							},
						},
						IPv6DHCP: []infrav1.NetworkDataIPv6DHCP{
							{
								Routes: []infrav1.NetworkDataRoutev6{
									{
										Gateway: infrav1.NetworkGatewayv6{
											FromIPPool: ptr.To("abcd-9"),
										},
									},
								},
							},
						},
						IPv6SLAAC: []infrav1.NetworkDataIPv6DHCP{
							{
								Routes: []infrav1.NetworkDataRoutev6{
									{
										Gateway: infrav1.NetworkGatewayv6{
											FromIPPool: ptr.To("abcd-10"),
										},
									},
								},
							},
						},
					},
				},
			},
			m3IPClaims: []string{
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
		Entry("CAPI IPAM", testCaseReleaseAddressesFromPool{
			m3dtSpec: infrav1.Metal3DataTemplateSpec{
				MetaData: &infrav1.MetaData{},
				NetworkData: &infrav1.NetworkData{
					Networks: infrav1.NetworkDataNetwork{
						IPv4: []infrav1.NetworkDataIPv4{
							{
								FromPoolRef: &corev1.TypedLocalObjectReference{APIGroup: ptr.To("ipam.cluster.x-k8s.io"), Kind: "TestPool", Name: "v4"},
							},
						},
						IPv6: []infrav1.NetworkDataIPv6{
							{
								FromPoolRef: &corev1.TypedLocalObjectReference{APIGroup: ptr.To("ipam.cluster.x-k8s.io"), Kind: "TestPool", Name: "v6"},
							},
						},
					},
				},
			},
			ipClaims: []string{
				"v4",
				"v6",
			},
		}),
	)

	type testCaseAddressFromM3Claim struct {
		m3d             *infrav1.Metal3Data
		m3dt            *infrav1.Metal3DataTemplate
		poolName        string
		poolRef         corev1.TypedLocalObjectReference
		ipClaim         *ipamv1.IPClaim
		ipAddress       *ipamv1.IPAddress
		expectError     bool
		expectRequeue   bool
		expectedAddress addressFromPool
		expectDataError bool
		expectClaim     bool
	}

	DescribeTable("Test addressFromM3Claim",
		func(tc testCaseAddressFromM3Claim) {
			objects := []client.Object{}
			if tc.ipAddress != nil {
				objects = append(objects, tc.ipAddress)
			}
			if tc.ipClaim != nil {
				objects = append(objects, tc.ipClaim)
			}
			if tc.m3dt != nil {
				objects = append(objects, tc.m3dt)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			dataMgr, err := NewDataManager(fakeClient, tc.m3d,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			poolAddress, requeue, err := dataMgr.addressFromM3Claim(
				context.TODO(), tc.poolRef, tc.ipClaim,
			)
			if tc.expectError {
				if tc.m3dt != nil {
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).To(HaveOccurred())
				}
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
			Expect(poolAddress).To(Equal(tc.expectedAddress))
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
				Expect(capm3IPClaim.Finalizers).To(ContainElement(infrav1.DataFinalizer))
			}
		},
		Entry("IPClaim without allocation", testCaseAddressFromM3Claim{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
				},
			},
			poolName:        testPoolName,
			poolRef:         corev1.TypedLocalObjectReference{Name: testPoolName},
			expectedAddress: addressFromPool{},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: testObjectMeta(metal3DataName+"-"+testPoolName, namespaceName, ""),
			},
			expectRequeue: true,
		}),
		Entry("Old IPClaim with deletion timestamp", testCaseAddressFromM3Claim{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
			},
			poolName:        testPoolName,
			poolRef:         corev1.TypedLocalObjectReference{Name: testPoolName},
			expectedAddress: addressFromPool{},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:              metal3DataName + "-" + testPoolName,
					Namespace:         namespaceName,
					DeletionTimestamp: &metav1.Time{Time: time.Now().Add(time.Minute)},
					Finalizers:        []string{"ipclaim.ipam.metal3.io"},
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{
						Name:      "abc-192.168.0.10",
						Namespace: namespaceName,
					},
				},
			},
			ipAddress: &ipamv1.IPAddress{
				ObjectMeta: testObjectMeta("abc-192.168.0.10", namespaceName, ""),
				Spec: ipamv1.IPAddressSpec{
					Address: ipamv1.IPAddressStr("192.168.0.10"),
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
					DNSServers: []ipamv1.IPAddressStr{
						"8.8.8.8",
					},
				},
			},
			expectRequeue: true,
			expectError:   false,
		}),
		Entry("In-use IPClaim with deletion timestamp", testCaseAddressFromM3Claim{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, "abc-def-ghi-jkl"),
			},
			poolName: testPoolName,
			poolRef:  corev1.TypedLocalObjectReference{Name: testPoolName},
			expectedAddress: addressFromPool{
				Address: ipamv1.IPAddressStr("192.168.0.10"),
				Prefix:  26,
				Gateway: ipamv1.IPAddressStr("192.168.0.1"),
				dnsServers: []ipamv1.IPAddressStr{
					"8.8.8.8",
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:              metal3DataName + "-" + testPoolName,
					Namespace:         namespaceName,
					DeletionTimestamp: &metav1.Time{Time: time.Now().Add(time.Minute)},
					Finalizers:        []string{"ipclaim.ipam.metal3.io"},
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "Metal3Data",
							Name: metal3DataName,
							UID:  "abc-def-ghi-jkl",
						},
					},
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{
						Name:      "abc-192.168.0.10",
						Namespace: namespaceName,
					},
				},
			},
			ipAddress: &ipamv1.IPAddress{
				ObjectMeta: testObjectMeta("abc-192.168.0.10", namespaceName, ""),
				Spec: ipamv1.IPAddressSpec{
					Address: ipamv1.IPAddressStr("192.168.0.10"),
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
					DNSServers: []ipamv1.IPAddressStr{
						"8.8.8.8",
					},
				},
			},
			expectRequeue: false,
		}),
		Entry("Old IPClaim (wrong UID) without deletion timestamp", testCaseAddressFromM3Claim{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, "abc-def-ghi-jkl"),
			},
			poolName: testPoolName,
			poolRef:  corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:       metal3DataName + "-" + testPoolName,
					Namespace:  namespaceName,
					Finalizers: []string{"ipclaim.ipam.metal3.io"},
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "Metal3Data",
							Name: metal3DataName,
							UID:  "not-the-same",
						},
					},
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{
						Name:      "abc-192.168.0.10",
						Namespace: namespaceName,
					},
				},
			},
			expectRequeue: true,
			expectError:   false,
		}),
		Entry("IPPool with allocation error", testCaseAddressFromM3Claim{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, "123-456-789"),
			},
			poolName:        testPoolName,
			poolRef:         corev1.TypedLocalObjectReference{Name: testPoolName},
			expectedAddress: addressFromPool{},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:       metal3DataName + "-" + testPoolName,
					Namespace:  namespaceName,
					Finalizers: []string{"ipclaim.ipam.metal3.io"},
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "Metal3Data",
							Name: metal3DataName,
							UID:  "123-456-789",
						},
					},
				},
				Status: ipamv1.IPClaimStatus{
					ErrorMessage: ptr.To("Error happened"),
				},
			},
			expectError:     true,
			expectDataError: true,
		}),
		Entry("IPAddress not found", testCaseAddressFromM3Claim{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
			},
			poolName:        testPoolName,
			poolRef:         corev1.TypedLocalObjectReference{Name: testPoolName},
			expectedAddress: addressFromPool{},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: testObjectMeta(metal3DataName+"-"+testPoolName, namespaceName, ""),
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{
						Name:      "abc-192.168.0.11",
						Namespace: namespaceName,
					},
				},
			},
			expectRequeue: true,
			expectError:   false,
		}),
		Entry("IPAddress found", testCaseAddressFromM3Claim{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, "123-456-789"),
				Spec: infrav1.Metal3DataSpec{
					Template: *testObjectReference(metal3DataTemplateName),
				},
			},
			poolName: testPoolName,
			poolRef:  corev1.TypedLocalObjectReference{Name: testPoolName},
			expectedAddress: addressFromPool{
				Address: ipamv1.IPAddressStr("192.168.0.10"),
				Prefix:  26,
				Gateway: ipamv1.IPAddressStr("192.168.0.1"),
				dnsServers: []ipamv1.IPAddressStr{
					"8.8.8.8",
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:       metal3DataName + "-" + testPoolName,
					Namespace:  namespaceName,
					Finalizers: []string{"ipclaim.ipam.metal3.io"},
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "Metal3Data",
							Name: metal3DataName,
							UID:  "123-456-789",
						},
					},
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{
						Name:      "abc-192.168.0.10",
						Namespace: namespaceName,
					},
				},
			},
			ipAddress: &ipamv1.IPAddress{
				ObjectMeta: testObjectMeta("abc-192.168.0.10", namespaceName, ""),
				Spec: ipamv1.IPAddressSpec{
					Address: ipamv1.IPAddressStr("192.168.0.10"),
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
					DNSServers: []ipamv1.IPAddressStr{
						"8.8.8.8",
					},
				},
			},
		}),
	)

	type testCaseReleaseAddressFromM3Pool struct {
		m3d             *infrav1.Metal3Data
		poolRef         corev1.TypedLocalObjectReference
		ipClaim         *ipamv1.IPClaim
		expectError     bool
		injectDeleteErr bool
	}

	DescribeTable("Test releaseAddressFromM3Pool",
		func(tc testCaseReleaseAddressFromM3Pool) {
			objects := []client.Object{}
			if tc.ipClaim != nil {
				objects = append(objects, tc.ipClaim)
			}
			fake := &releaseAddressFromPoolFakeClient{
				Client:          fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build(),
				injectDeleteErr: tc.injectDeleteErr,
			}
			dataMgr, err := NewDataManager(fake, tc.m3d,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			err = dataMgr.releaseAddressFromM3Pool(
				context.TODO(), tc.poolRef,
			)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.ipClaim != nil {
				capm3IPClaim := &ipamv1.IPClaim{}
				claimNamespacedName := types.NamespacedName{
					Name:      tc.ipClaim.Name,
					Namespace: tc.ipClaim.Namespace,
				}

				err = dataMgr.client.Get(context.TODO(), claimNamespacedName, capm3IPClaim)
				if tc.injectDeleteErr {
					// There was an error deleting the claim, so we expect it to still be there
					Expect(err).ToNot(HaveOccurred())
					// We expect the finalizer to be gone
					Expect(capm3IPClaim.Finalizers).To(BeEmpty())
				} else {
					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsNotFound(err)).To(BeTrue())
				}
			}
		},
		Entry("IPClaim exists", testCaseReleaseAddressFromM3Pool{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: infrav1.GroupVersion.String(),
				},
			},
			poolRef: corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: testObjectMeta(metal3DataName+"-"+testPoolName, namespaceName, ""),
			},
		}),
		Entry("IPClaim does not exist", testCaseReleaseAddressFromM3Pool{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
			},
			poolRef: corev1.TypedLocalObjectReference{Name: "abc"},
		}),
		Entry("Deletion error and finalizer removal", testCaseReleaseAddressFromM3Pool{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: infrav1.GroupVersion.String(),
				},
			},
			poolRef: corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:       metal3DataName + "-" + testPoolName,
					Namespace:  namespaceName,
					Finalizers: []string{infrav1.DataFinalizer},
				},
			},
			injectDeleteErr: true,
			expectError:     true,
		}),
	)

	type testCaseMultiReleaseAddressFromM3Pool struct {
		m3d      *infrav1.Metal3Data
		poolRef  corev1.TypedLocalObjectReference
		ipClaims []ipamv1.IPClaim
	}

	DescribeTable("Test releaseAddressFromM3Pool with multiple namespaces",
		func(tc testCaseMultiReleaseAddressFromM3Pool) {
			objects := []client.Object{}
			for i := range tc.ipClaims {
				// To make the test entries a bit smaller, we add the
				// .spec.pool here based on the labels.
				tc.ipClaims[i].Spec.Pool.Name = tc.ipClaims[i].Labels[PoolLabelName]
				objects = append(objects, &tc.ipClaims[i])
			}
			fake := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			dataMgr, err := NewDataManager(fake, tc.m3d,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			err = dataMgr.releaseAddressFromM3Pool(
				context.TODO(), tc.poolRef,
			)
			Expect(err).NotTo(HaveOccurred())

			for i := range tc.ipClaims {
				capm3IPClaim := &ipamv1.IPClaim{}
				claimNamespacedName := types.NamespacedName{
					Name:      tc.ipClaims[i].Name,
					Namespace: tc.ipClaims[i].Namespace,
				}

				err = dataMgr.client.Get(context.TODO(), claimNamespacedName, capm3IPClaim)
				if tc.ipClaims[i].Namespace != dataMgr.Data.Namespace {
					// We should not touch other namespaces!
					Expect(err).ToNot(HaveOccurred())
				} else if tc.ipClaims[i].Spec.Pool.Name != tc.poolRef.Name {
					// We should not touch other pools!
					Expect(err).ToNot(HaveOccurred())
				} else {
					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsNotFound(err)).To(BeTrue())
				}
			}
		},
		Entry("Singe IPClaim deleted", testCaseMultiReleaseAddressFromM3Pool{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: infrav1.GroupVersion.String(),
				},
			},
			poolRef: corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaims: []ipamv1.IPClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "host-0-" + testPoolName,
					Namespace: namespaceName,
					Labels: map[string]string{
						DataLabelName: metal3DataName,
						PoolLabelName: testPoolName,
					},
				},
			}},
		}),
		Entry("Multiple IPClaims related to the same M3D", testCaseMultiReleaseAddressFromM3Pool{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: infrav1.GroupVersion.String(),
				},
			},
			poolRef: corev1.TypedLocalObjectReference{Name: "first-pool"},
			ipClaims: []ipamv1.IPClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host-0-" + "first-pool",
						Namespace: namespaceName,
						Labels: map[string]string{
							DataLabelName: metal3DataName,
							PoolLabelName: "first-pool",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host-0-" + "second-pool",
						Namespace: namespaceName,
						Labels: map[string]string{
							DataLabelName: metal3DataName,
							PoolLabelName: "second-pool",
						},
					},
				},
			},
		}),
		Entry("Multiple IPClaims in different namespaces exists with different labels", testCaseMultiReleaseAddressFromM3Pool{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: infrav1.GroupVersion.String(),
				},
			},
			poolRef: corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaims: []ipamv1.IPClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host-0-" + testPoolName,
						Namespace: namespaceName,
						Labels: map[string]string{
							DataLabelName: metal3DataName,
							PoolLabelName: testPoolName,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host-1-" + testPoolName,
						Namespace: "other-namespace",
						Labels: map[string]string{
							DataLabelName: "different-dataname",
							PoolLabelName: testPoolName,
						},
					},
				},
			},
		}),
		Entry("Multiple IPClaims in different namespaces exists with same labels", testCaseMultiReleaseAddressFromM3Pool{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: infrav1.GroupVersion.String(),
				},
			},
			poolRef: corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaims: []ipamv1.IPClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host-0-" + testPoolName,
						Namespace: namespaceName,
						Labels: map[string]string{
							DataLabelName: metal3DataName,
							PoolLabelName: testPoolName,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host-1-" + testPoolName,
						Namespace: "other-namespace",
						Labels: map[string]string{
							DataLabelName: metal3DataName,
							PoolLabelName: testPoolName,
						},
					},
				},
			},
		}),
	)

	DescribeTable("ensureM3IPClaim", func(tc testCaseEnsureM3Claim) {
		bmh := &bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "host-0",
				Namespace: namespaceName,
			},
		}
		m3m := &infrav1.Metal3Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      metal3machineName,
				Namespace: namespaceName,
				Annotations: map[string]string{
					HostAnnotation: namespaceName + "/" + bmh.Name,
				},
			},
			Spec: infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name:      metal3DataTemplateName,
					Namespace: namespaceName,
				},
			},
		}
		m3dt := &infrav1.Metal3DataTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      metal3DataTemplateName,
				Namespace: namespaceName,
			},
		}
		m3dc := &infrav1.Metal3DataClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      metal3DataClaimName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: infrav1.GroupVersion.Group + "/" + infrav1.GroupVersion.Version,
						Kind:       "Metal3Machine",
						Name:       m3m.Name,
					},
				},
			},
		}
		m3d := &infrav1.Metal3Data{
			TypeMeta: metav1.TypeMeta{
				APIVersion: infrav1.GroupVersion.Group + "/" + infrav1.GroupVersion.Version,
				Kind:       "Metal3Data",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      metal3DataName,
				Namespace: namespaceName,
			},
			Spec: infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name:      m3dt.Name,
					Namespace: m3dt.Namespace,
				},
				Claim: corev1.ObjectReference{
					Namespace: namespaceName,
					Name:      metal3DataClaimName,
				},
			},
		}

		// Setup fake client with objects
		objects := []client.Object{bmh, m3m, m3d, m3dt, m3dc}
		if tc.ipClaim != nil {
			objects = append(objects, tc.ipClaim)
		}
		fc := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
		dataMgr, err := NewDataManager(fc, m3d, logr.Discard())
		Expect(err).NotTo(HaveOccurred())

		rc, err := dataMgr.ensureM3IPClaim(context.Background(), tc.poolRef)

		if tc.expectError {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
		Expect(rc.fetchAgain).To(Equal(tc.expectFetchAgain))
		if tc.expectClaim {
			Expect(rc.m3Claim).NotTo(BeNil())
			claim := &ipamv1.IPClaim{}
			nn := types.NamespacedName{
				Name:      m3d.Name + "-" + tc.poolRef.Name,
				Namespace: m3d.Namespace,
			}
			err = fc.Get(context.Background(), nn, claim)
			Expect(err).NotTo(HaveOccurred())

			_, err := findOwnerRefFromList(claim.OwnerReferences,
				m3d.TypeMeta, m3d.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(tc.ipClaim).To(BeNil())
		}
	},
		Entry("should create claim if missing", testCaseEnsureM3Claim{
			poolRef:          corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaim:          nil,
			expectError:      false,
			expectFetchAgain: true,
			expectClaim:      true,
		}),
		Entry("should do nothing when claim exists", testCaseEnsureM3Claim{
			poolRef: corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      metal3DataName + "-" + testPoolName,
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: infrav1.GroupVersion.Group + "/" + infrav1.GroupVersion.Version,
							Kind:       "Metal3Data",
							Name:       metal3DataName,
							Controller: ptr.To(true),
						},
					}},
			},
			expectError:      false,
			expectFetchAgain: false,
			expectClaim:      true,
		}),
	)

	type testCaseEnsureClaim struct {
		poolRef          corev1.TypedLocalObjectReference
		ipClaim          *capipamv1beta1.IPAddressClaim
		expectError      bool
		expectFetchAgain bool
		expectClaim      bool
	}

	DescribeTable("ensureIPClaim", func(tc testCaseEnsureClaim) {
		fc := fakeClient(tc.ipClaim)
		m3d := &infrav1.Metal3Data{
			ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
		}
		dataMgr, err := NewDataManager(fc, m3d, logr.Discard())
		Expect(err).NotTo(HaveOccurred())

		rc, err := dataMgr.ensureIPClaim(context.Background(), tc.poolRef)

		if tc.expectError {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
		Expect(rc.fetchAgain).To(Equal(tc.expectFetchAgain))
		if tc.expectClaim {
			Expect(rc.claim).NotTo(BeNil())
			claim := &capipamv1beta1.IPAddressClaim{}
			nn := types.NamespacedName{
				Name:      m3d.Name + "-" + tc.poolRef.Name,
				Namespace: m3d.Namespace,
			}
			err = fc.Get(context.Background(), nn, claim)
			Expect(err).NotTo(HaveOccurred())

			_, err := findOwnerRefFromList(claim.OwnerReferences,
				m3d.TypeMeta, m3d.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(tc.ipClaim).To(BeNil())
		}
	},
		Entry("no claim exists", testCaseEnsureClaim{
			poolRef:          corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaim:          nil,
			expectError:      false,
			expectFetchAgain: true,
			expectClaim:      true,
		}),
		Entry("claim exists", testCaseEnsureClaim{
			poolRef: corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaim: &capipamv1beta1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      metal3DataName + "-" + testPoolName,
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "/",
							Name:       metal3DataName,
							Controller: ptr.To(true),
						},
					}},
			},
			expectError:      false,
			expectFetchAgain: false,
			expectClaim:      true,
		}),
	)

	type testCaseAddressFromClaim struct {
		m3d             *infrav1.Metal3Data
		poolName        string
		poolRef         corev1.TypedLocalObjectReference
		ipClaim         *capipamv1beta1.IPAddressClaim
		ipAddress       *capipamv1beta1.IPAddress
		expectError     bool
		expectRequeue   bool
		expectedAddress addressFromPool
		expectDataError bool
		expectClaim     bool
	}

	DescribeTable("Test addressFromClaim",
		func(tc testCaseAddressFromClaim) {
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
			poolAddress, requeue, err := dataMgr.addressFromClaim(
				context.TODO(), tc.poolRef, tc.ipClaim,
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
			Expect(poolAddress).To(Equal(tc.expectedAddress))
			if tc.expectClaim {
				claim := &capipamv1beta1.IPAddressClaim{}
				nn := types.NamespacedName{
					Name:      tc.m3d.Name + "-" + tc.poolName,
					Namespace: tc.m3d.Namespace,
				}

				err = dataMgr.client.Get(context.TODO(), nn, claim)
				Expect(err).NotTo(HaveOccurred())
				_, err := findOwnerRefFromList(claim.OwnerReferences,
					tc.m3d.TypeMeta, tc.m3d.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("IPClaim without allocation", testCaseAddressFromClaim{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
			},
			poolName:        testPoolName,
			poolRef:         corev1.TypedLocalObjectReference{Name: testPoolName},
			expectedAddress: addressFromPool{},
			ipClaim: &capipamv1beta1.IPAddressClaim{
				ObjectMeta: testObjectMeta(metal3DataName+"-"+testPoolName, namespaceName, ""),
			},
			expectRequeue: true,
		}),
		Entry("IPClaim with deletion timestamp", testCaseAddressFromClaim{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
			},
			poolName:        testPoolName,
			expectedAddress: addressFromPool{},
			ipClaim: &capipamv1beta1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:              metal3DataName + "-" + testPoolName,
					Namespace:         namespaceName,
					DeletionTimestamp: &metav1.Time{Time: time.Now().Add(time.Minute)},
					Finalizers:        []string{"ipclaim.ipam.metal3.io"},
				},
				Status: capipamv1beta1.IPAddressClaimStatus{
					AddressRef: corev1.LocalObjectReference{
						Name: "abc-192.168.0.10",
					},
				},
			},
			ipAddress: &capipamv1beta1.IPAddress{
				ObjectMeta: testObjectMeta("abc-192.168.0.10", namespaceName, ""),
				Spec: capipamv1beta1.IPAddressSpec{
					Address: "192.168.0.10",
					Prefix:  26,
					Gateway: "192.168.0.1",
				},
			},
			expectRequeue: true,
		}),
		Entry("IPAddress not found", testCaseAddressFromClaim{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
			},
			poolName:        testPoolName,
			poolRef:         corev1.TypedLocalObjectReference{Name: testPoolName},
			expectedAddress: addressFromPool{},
			ipClaim: &capipamv1beta1.IPAddressClaim{
				ObjectMeta: testObjectMeta("abc-abc", namespaceName, ""),
				Status: capipamv1beta1.IPAddressClaimStatus{
					AddressRef: corev1.LocalObjectReference{
						Name: "abc-192.168.0.11",
					},
				},
			},
			expectRequeue: true,
		}),
		Entry("IPAddress found", testCaseAddressFromClaim{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
			},
			poolName: testPoolName,
			poolRef:  corev1.TypedLocalObjectReference{Name: testPoolName},
			expectedAddress: addressFromPool{
				Address:    ipamv1.IPAddressStr("192.168.0.10"),
				Prefix:     26,
				Gateway:    ipamv1.IPAddressStr("192.168.0.1"),
				dnsServers: []ipamv1.IPAddressStr{},
			},
			ipClaim: &capipamv1beta1.IPAddressClaim{
				ObjectMeta: testObjectMeta(metal3DataName+"-"+testPoolName, namespaceName, ""),
				Status: capipamv1beta1.IPAddressClaimStatus{
					AddressRef: corev1.LocalObjectReference{
						Name: "abc-192.168.0.10",
					},
				},
			},
			ipAddress: &capipamv1beta1.IPAddress{
				ObjectMeta: testObjectMeta("abc-192.168.0.10", namespaceName, ""),
				Spec: capipamv1beta1.IPAddressSpec{
					Address: "192.168.0.10",
					Prefix:  26,
					Gateway: "192.168.0.1",
				},
			},
		}),
	)

	type testCaseReleaseAddressFromPool struct {
		m3d         *infrav1.Metal3Data
		poolRef     corev1.TypedLocalObjectReference
		ipClaim     *capipamv1beta1.IPAddressClaim
		expectError bool
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
			err = dataMgr.releaseAddressFromPool(
				context.TODO(), tc.poolRef,
			)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.ipClaim != nil {
				capm3IPClaim := &capipamv1beta1.IPAddressClaim{}
				nn := types.NamespacedName{
					Name:      tc.m3d.Name,
					Namespace: tc.m3d.Namespace,
				}
				err = dataMgr.client.Get(context.TODO(), nn, capm3IPClaim)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		},
		Entry("Deletion already attempted", testCaseReleaseAddressFromPool{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
				TypeMeta: metav1.TypeMeta{
					Kind:       "Metal3Data",
					APIVersion: infrav1.GroupVersion.String(),
				},
			},
			poolRef: corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaim: &capipamv1beta1.IPAddressClaim{
				ObjectMeta: testObjectMeta(metal3DataName+"-"+testPoolName, namespaceName, ""),
			},
		}),
		Entry("IPClaim does not exist", testCaseReleaseAddressFromPool{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
			},
			poolRef: corev1.TypedLocalObjectReference{Name: "abc"},
		}),
	)

	type testCaseRenderNetworkData struct {
		m3dt           *infrav1.Metal3DataTemplate
		m3m            *infrav1.Metal3Machine
		machine        *clusterv1beta1.Machine
		bmh            *bmov1alpha1.BareMetalHost
		poolAddresses  map[string]addressFromPool
		expectError    bool
		expectedOutput map[string][]interface{}
	}

	DescribeTable("Test renderNetworkData",
		func(tc testCaseRenderNetworkData) {
			result, err := renderNetworkData(tc.m3dt, tc.m3m, tc.machine, tc.bmh, tc.poolAddresses)
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
			m3dt: &infrav1.Metal3DataTemplate{
				Spec: infrav1.Metal3DataTemplateSpec{
					NetworkData: &infrav1.NetworkData{
						Links: infrav1.NetworkDataLink{
							Ethernets: []infrav1.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &infrav1.NetworkLinkEthernetMac{
										String: ptr.To("12:34:56:78:9A:BC"),
									},
								},
							},
						},
						Networks: infrav1.NetworkDataNetwork{
							IPv4: []infrav1.NetworkDataIPv4{
								{
									ID:                  "abc",
									Link:                "def",
									IPAddressFromIPPool: "abc",
									Routes: []infrav1.NetworkDataRoutev4{
										{
											Network: "10.0.0.0",
											Prefix:  16,
											Gateway: infrav1.NetworkGatewayv4{
												String: (*ipamv1.IPAddressv4Str)(ptr.To("192.168.1.1")),
											},
											Services: infrav1.NetworkDataServicev4{
												DNS: []ipamv1.IPAddressv4Str{
													ipamv1.IPAddressv4Str("8.8.8.8"),
												},
											},
										},
									},
								},
							},
						},
						Services: infrav1.NetworkDataService{
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
					Address: "192.168.0.14",
					Prefix:  24,
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
						"ethernet_mac_address": "12:34:56:78:9A:BC",
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
			m3dt: &infrav1.Metal3DataTemplate{
				Spec: infrav1.Metal3DataTemplateSpec{
					NetworkData: &infrav1.NetworkData{
						Links: infrav1.NetworkDataLink{
							Ethernets: []infrav1.NetworkDataLinkEthernet{
								{
									Type: "phy",
									Id:   "eth0",
									MTU:  1500,
									MACAddress: &infrav1.NetworkLinkEthernetMac{
										FromHostInterface: ptr.To("eth0"),
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
			m3dt: &infrav1.Metal3DataTemplate{
				Spec: infrav1.Metal3DataTemplateSpec{
					NetworkData: &infrav1.NetworkData{
						Networks: infrav1.NetworkDataNetwork{
							IPv4: []infrav1.NetworkDataIPv4{
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
			m3dt: &infrav1.Metal3DataTemplate{
				Spec: infrav1.Metal3DataTemplateSpec{
					NetworkData: nil,
				},
			},
			expectedOutput: map[string][]interface{}{},
		}),
	)

	type testRenderNetworkServices struct {
		services       infrav1.NetworkDataService
		poolAddresses  map[string]addressFromPool
		expectedOutput []interface{}
		expectError    bool
	}

	DescribeTable("Test renderNetworkServices",
		func(tc testRenderNetworkServices) {
			result, err := renderNetworkServices(tc.services, tc.poolAddresses)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(tc.expectedOutput))
		},
		Entry("Services and poolAddresses have the same pool", testRenderNetworkServices{
			services: infrav1.NetworkDataService{
				DNS: []ipamv1.IPAddressStr{
					(ipamv1.IPAddressStr)("8.8.8.8"),
					(ipamv1.IPAddressStr)("2001::8888"),
				},
				DNSFromIPPool: ptr.To("pool1"),
			},
			poolAddresses: map[string]addressFromPool{
				"pool1": {
					dnsServers: []ipamv1.IPAddressStr{
						ipamv1.IPAddressStr("8.8.4.4"),
					},
				},
			},
			expectedOutput: []interface{}{
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
			},
			expectError: false,
		}),
		Entry("Services and poolAddresses have different pools", testRenderNetworkServices{
			services: infrav1.NetworkDataService{
				DNS: []ipamv1.IPAddressStr{
					(ipamv1.IPAddressStr)("8.8.8.8"),
					(ipamv1.IPAddressStr)("2001::8888"),
				},
				DNSFromIPPool: ptr.To("pool1"),
			},
			poolAddresses: map[string]addressFromPool{
				"pool2": {
					dnsServers: []ipamv1.IPAddressStr{
						ipamv1.IPAddressStr("8.8.4.4"),
					},
				},
			},
			expectError: true,
		}),
	)
	type testCaseRenderNetworkLinks struct {
		links          infrav1.NetworkDataLink
		m3m            *infrav1.Metal3Machine
		machine        *clusterv1beta1.Machine
		bmh            *bmov1alpha1.BareMetalHost
		expectError    bool
		expectedOutput []interface{}
	}

	DescribeTable("Test renderNetworkLinks",
		func(tc testCaseRenderNetworkLinks) {
			result, err := renderNetworkLinks(tc.links, tc.m3m, tc.machine, tc.bmh)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(tc.expectedOutput))
		},
		Entry("Ethernet, MAC from string", testCaseRenderNetworkLinks{
			links: infrav1.NetworkDataLink{
				Ethernets: []infrav1.NetworkDataLinkEthernet{
					{
						Type: "phy",
						Id:   "eth0",
						MTU:  1500,
						MACAddress: &infrav1.NetworkLinkEthernetMac{
							String: ptr.To("12:34:56:78:9A:BC"),
						},
					},
				},
			},
			expectedOutput: []interface{}{
				map[string]interface{}{
					"type":                 "phy",
					"id":                   "eth0",
					"mtu":                  1500,
					"ethernet_mac_address": "12:34:56:78:9A:BC",
				},
			},
		}),
		Entry("Ethernet, MAC error", testCaseRenderNetworkLinks{
			links: infrav1.NetworkDataLink{
				Ethernets: []infrav1.NetworkDataLinkEthernet{
					{
						Type: "phy",
						Id:   "eth0",
						MTU:  1500,
						MACAddress: &infrav1.NetworkLinkEthernetMac{
							FromHostInterface: ptr.To("eth2"),
						},
					},
				},
			},
			bmh: &bmov1alpha1.BareMetalHost{
				ObjectMeta: testObjectMeta(baremetalhostName, namespaceName, ""),
				Status:     bmov1alpha1.BareMetalHostStatus{},
			},
			expectError: true,
		}),
		Entry("Bond, MAC from string", testCaseRenderNetworkLinks{
			links: infrav1.NetworkDataLink{
				Bonds: []infrav1.NetworkDataLinkBond{
					{
						BondMode:           "802.3ad",
						BondXmitHashPolicy: "layer3+4",
						Id:                 "bond0",
						MTU:                1500,
						MACAddress: &infrav1.NetworkLinkEthernetMac{
							String: ptr.To("12:34:56:78:9A:BC"),
						},
						BondLinks: []string{"eth0"},
					},
				},
			},
			expectedOutput: []interface{}{
				map[string]interface{}{
					"type":                  "bond",
					"id":                    "bond0",
					"mtu":                   1500,
					"ethernet_mac_address":  "12:34:56:78:9A:BC",
					"bond_mode":             "802.3ad",
					"bond_xmit_hash_policy": "layer3+4",
					"bond_links":            []string{"eth0"},
				},
			},
		}),
		Entry("Bond, MAC error", testCaseRenderNetworkLinks{
			links: infrav1.NetworkDataLink{
				Bonds: []infrav1.NetworkDataLinkBond{
					{
						BondMode: "802.3ad",
						Id:       "bond0",
						MTU:      1500,
						MACAddress: &infrav1.NetworkLinkEthernetMac{
							FromHostInterface: ptr.To("eth2"),
						},
						BondLinks: []string{"eth0"},
					},
				},
			},
			bmh: &bmov1alpha1.BareMetalHost{
				ObjectMeta: testObjectMeta(baremetalhostName, namespaceName, ""),
				Status:     bmov1alpha1.BareMetalHostStatus{},
			},
			expectError: true,
		}),
		Entry("Vlan, MAC from string", testCaseRenderNetworkLinks{
			links: infrav1.NetworkDataLink{
				Vlans: []infrav1.NetworkDataLinkVlan{
					{
						VlanID: 2222,
						Id:     "bond0",
						MTU:    1500,
						MACAddress: &infrav1.NetworkLinkEthernetMac{
							String: ptr.To("12:34:56:78:9A:BC"),
						},
						VlanLink: "eth0",
					},
				},
			},
			expectedOutput: []interface{}{
				map[string]interface{}{
					"vlan_mac_address": "12:34:56:78:9A:BC",
					"vlan_id":          2222,
					"vlan_link":        "eth0",
					"type":             "vlan",
					"id":               "bond0",
					"mtu":              1500,
				},
			},
		}),
		Entry("Vlan, MAC error", testCaseRenderNetworkLinks{
			links: infrav1.NetworkDataLink{
				Vlans: []infrav1.NetworkDataLinkVlan{
					{
						VlanID: 2222,
						Id:     "bond0",
						MTU:    1500,
						MACAddress: &infrav1.NetworkLinkEthernetMac{
							FromHostInterface: ptr.To("eth2"),
						},
						VlanLink: "eth0",
					},
				},
			},
			bmh: &bmov1alpha1.BareMetalHost{
				ObjectMeta: testObjectMeta(baremetalhostName, namespaceName, ""),
				Status:     bmov1alpha1.BareMetalHostStatus{},
			},
			expectError: true,
		}),
	)

	type testCaseRenderNetworkNetworks struct {
		networks       infrav1.NetworkDataNetwork
		m3d            *infrav1.Metal3Data
		poolAddresses  map[string]addressFromPool
		expectError    bool
		expectedOutput []interface{}
	}

	DescribeTable("Test renderNetworkNetworks",
		func(tc testCaseRenderNetworkNetworks) {
			result, err := renderNetworkNetworks(tc.networks, tc.poolAddresses)
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
					Address: ipamv1.IPAddressStr("192.168.0.14"),
					Prefix:  24,
					Gateway: ipamv1.IPAddressStr("192.168.1.1"),
				},
			},
			networks: infrav1.NetworkDataNetwork{
				IPv4: []infrav1.NetworkDataIPv4{
					{
						ID:                  "abc",
						Link:                "def",
						IPAddressFromIPPool: "abc",
						Routes: []infrav1.NetworkDataRoutev4{
							{
								Network: "10.0.0.0",
								Prefix:  16,
								Gateway: infrav1.NetworkGatewayv4{
									FromIPPool: ptr.To("abc"),
								},
								Services: infrav1.NetworkDataServicev4{
									DNS: []ipamv1.IPAddressv4Str{
										ipamv1.IPAddressv4Str("8.8.8.8"),
									},
								},
							},
						},
					},
				},
			},
			m3d: &infrav1.Metal3Data{
				Spec: infrav1.Metal3DataSpec{
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
			networks: infrav1.NetworkDataNetwork{
				IPv4: []infrav1.NetworkDataIPv4{
					{
						IPAddressFromIPPool: "abc",
					},
				},
			},
			m3d: &infrav1.Metal3Data{
				Spec: infrav1.Metal3DataSpec{
					Index: 1000,
				},
			},
			expectError: true,
		}),
		Entry("IPv6 network", testCaseRenderNetworkNetworks{
			poolAddresses: map[string]addressFromPool{
				"abc": {
					Address: ipamv1.IPAddressStr("fe80::2001:38"),
					Prefix:  96,
					Gateway: ipamv1.IPAddressStr("fe80::2001:1"),
				},
			},
			networks: infrav1.NetworkDataNetwork{
				IPv6: []infrav1.NetworkDataIPv6{
					{
						ID:                  "abc",
						Link:                "def",
						IPAddressFromIPPool: "abc",
						Routes: []infrav1.NetworkDataRoutev6{
							{
								Network: "2001::",
								Prefix:  64,
								Gateway: infrav1.NetworkGatewayv6{
									FromIPPool: ptr.To("abc"),
								},
								Services: infrav1.NetworkDataServicev6{
									DNS: []ipamv1.IPAddressv6Str{
										ipamv1.IPAddressv6Str("2001::8888"),
									},
								},
							},
						},
					},
				},
			},
			m3d: &infrav1.Metal3Data{
				Spec: infrav1.Metal3DataSpec{
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
			networks: infrav1.NetworkDataNetwork{
				IPv6: []infrav1.NetworkDataIPv6{
					{
						IPAddressFromIPPool: "abc",
					},
				},
			},
			m3d: &infrav1.Metal3Data{
				Spec: infrav1.Metal3DataSpec{
					Index: 10000,
				},
			},
			expectError: true,
		}),
		Entry("IPv4 DHCP", testCaseRenderNetworkNetworks{
			networks: infrav1.NetworkDataNetwork{
				IPv4DHCP: []infrav1.NetworkDataIPv4DHCP{
					{
						ID:   "abc",
						Link: "def",
						Routes: []infrav1.NetworkDataRoutev4{
							{
								Network: "10.0.0.0",
								Prefix:  16,
								Gateway: infrav1.NetworkGatewayv4{
									String: (*ipamv1.IPAddressv4Str)(ptr.To("192.168.1.1")),
								},
								Services: infrav1.NetworkDataServicev4{
									DNS: []ipamv1.IPAddressv4Str{
										ipamv1.IPAddressv4Str("8.8.8.8"),
									},
								},
							},
						},
					},
				},
			},
			m3d: &infrav1.Metal3Data{
				Spec: infrav1.Metal3DataSpec{
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
			networks: infrav1.NetworkDataNetwork{
				IPv6DHCP: []infrav1.NetworkDataIPv6DHCP{
					{
						ID:   "abc",
						Link: "def",
						Routes: []infrav1.NetworkDataRoutev6{
							{
								Network: "2001::",
								Prefix:  64,
								Gateway: infrav1.NetworkGatewayv6{
									String: (*ipamv1.IPAddressv6Str)(ptr.To("fe80::2001:1")),
								},
								Services: infrav1.NetworkDataServicev6{
									DNS: []ipamv1.IPAddressv6Str{
										ipamv1.IPAddressv6Str("2001::8888"),
									},
								},
							},
						},
					},
				},
			},
			m3d: &infrav1.Metal3Data{
				Spec: infrav1.Metal3DataSpec{
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
			networks: infrav1.NetworkDataNetwork{
				IPv6SLAAC: []infrav1.NetworkDataIPv6DHCP{
					{
						ID:   "abc",
						Link: "def",
						Routes: []infrav1.NetworkDataRoutev6{
							{
								Network: "2001::",
								Prefix:  64,
								Gateway: infrav1.NetworkGatewayv6{
									String: (*ipamv1.IPAddressv6Str)(ptr.To("fe80::2001:1")),
								},
								Services: infrav1.NetworkDataServicev6{
									DNS: []ipamv1.IPAddressv6Str{
										ipamv1.IPAddressv6Str("2001::8888"),
									},
								},
							},
						},
					},
				},
			},
			m3d: &infrav1.Metal3Data{
				Spec: infrav1.Metal3DataSpec{
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
		netRoutes := []infrav1.NetworkDataRoutev4{
			{
				Network: "192.168.0.0",
				Prefix:  24,
				Gateway: infrav1.NetworkGatewayv4{
					String: (*ipamv1.IPAddressv4Str)(ptr.To("192.168.1.1")),
				},
			},
			{
				Network: "10.0.0.0",
				Prefix:  16,
				Gateway: infrav1.NetworkGatewayv4{
					FromIPPool: ptr.To("abc"),
				},
				Services: infrav1.NetworkDataServicev4{
					DNS: []ipamv1.IPAddressv4Str{
						ipamv1.IPAddressv4Str("8.8.8.8"),
						ipamv1.IPAddressv4Str("8.8.4.4"),
					},
					DNSFromIPPool: ptr.To("abc"),
				},
			},
		}
		poolAddresses := map[string]addressFromPool{
			"abc": {
				Gateway: "192.168.2.1",
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
		netRoutes := []infrav1.NetworkDataRoutev6{
			{
				Network: "2001::0",
				Prefix:  96,
				Gateway: infrav1.NetworkGatewayv6{
					String: (*ipamv1.IPAddressv6Str)(ptr.To("2001::1")),
				},
			},
			{
				Network: "fe80::0",
				Prefix:  64,
				Gateway: infrav1.NetworkGatewayv6{
					FromIPPool: ptr.To("abc"),
				},
				Services: infrav1.NetworkDataServicev6{
					DNS: []ipamv1.IPAddressv6Str{
						ipamv1.IPAddressv6Str("fe80:2001::8888"),
						ipamv1.IPAddressv6Str("fe80:2001::8844"),
					},
					DNSFromIPPool: ptr.To("abc"),
				},
			},
		}
		poolAddresses := map[string]addressFromPool{
			"abc": {
				Gateway: "fe80::1",
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
		mac         *infrav1.NetworkLinkEthernetMac
		m3m         *infrav1.Metal3Machine
		machine     *clusterv1beta1.Machine
		bmh         *bmov1alpha1.BareMetalHost
		expectError bool
		expectedMAC string
	}

	DescribeTable("Test getLinkMacAddress",
		func(tc testCaseGetLinkMacAddress) {
			result, err := getLinkMacAddress(tc.mac, tc.m3m, tc.machine, tc.bmh)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(tc.expectedMAC))
		},
		Entry("string", testCaseGetLinkMacAddress{
			mac: &infrav1.NetworkLinkEthernetMac{
				String: ptr.To("12:34:56:78:9A:BC"),
			},
			expectedMAC: "12:34:56:78:9A:BC",
		}),
		Entry("from host interface", testCaseGetLinkMacAddress{
			mac: &infrav1.NetworkLinkEthernetMac{
				FromHostInterface: ptr.To("eth1"),
			},
			bmh: &bmov1alpha1.BareMetalHost{
				ObjectMeta: testObjectMeta(baremetalhostName, namespaceName, ""),
				Status: bmov1alpha1.BareMetalHostStatus{
					HardwareDetails: &bmov1alpha1.HardwareDetails{
						NIC: []bmov1alpha1.NIC{
							{
								Name: "eth0",
								MAC:  "12:34:56:78:9A:BC",
							},
							// Check if empty value cause failure
							{},
							{
								Name: "eth1",
								MAC:  "DE:F0:12:34:56:78",
							},
						},
					},
				},
			},
			expectedMAC: "DE:F0:12:34:56:78",
		}),
		Entry("from host interface not found", testCaseGetLinkMacAddress{
			mac: &infrav1.NetworkLinkEthernetMac{
				FromHostInterface: ptr.To("eth2"),
			},
			bmh: &bmov1alpha1.BareMetalHost{
				ObjectMeta: testObjectMeta(baremetalhostName, namespaceName, ""),
				Status: bmov1alpha1.BareMetalHostStatus{
					HardwareDetails: &bmov1alpha1.HardwareDetails{
						NIC: []bmov1alpha1.NIC{
							{
								Name: "eth0",
								MAC:  "12:34:56:78:9A:BC",
							},
							// Check if empty value cause failure
							{},
							{
								Name: "eth1",
								MAC:  "DE:F0:12:34:56:78",
							},
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("from machine annotation", testCaseGetLinkMacAddress{
			mac: &infrav1.NetworkLinkEthernetMac{
				FromAnnotation: &infrav1.NetworkLinkEthernetMacFromAnnotation{
					Object:     "machine",
					Annotation: "mac-address",
				},
			},
			machine: &clusterv1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: machineName,
					Annotations: map[string]string{
						"mac-address": "12:34:56:78:9A:BD",
					},
				},
			},
			expectedMAC: "12:34:56:78:9A:BD",
		}),
		Entry("from metal3machine annotation", testCaseGetLinkMacAddress{
			mac: &infrav1.NetworkLinkEthernetMac{
				FromAnnotation: &infrav1.NetworkLinkEthernetMacFromAnnotation{
					Object:     "metal3machine",
					Annotation: "mac-address",
				},
			},
			m3m: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      metal3machineName,
					Namespace: namespaceName,
					UID:       m3muid,
					Annotations: map[string]string{
						"mac-address": "12:34:56:78:9A:BD",
					},
				},
			},
			expectedMAC: "12:34:56:78:9A:BD",
		}),
		Entry("from baremetalhost annotation", testCaseGetLinkMacAddress{
			mac: &infrav1.NetworkLinkEthernetMac{
				FromAnnotation: &infrav1.NetworkLinkEthernetMacFromAnnotation{
					Object:     "baremetalhost",
					Annotation: "mac-address",
				},
			},
			bmh: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      baremetalhostName,
					Namespace: namespaceName,
					UID:       "",
					Annotations: map[string]string{
						"mac-address": "12:34:56:78:9A:BD",
					},
				},
			},
			expectedMAC: "12:34:56:78:9A:BD",
		}),
		Entry("from annotation on unknown object", testCaseGetLinkMacAddress{
			mac: &infrav1.NetworkLinkEthernetMac{
				FromAnnotation: &infrav1.NetworkLinkEthernetMacFromAnnotation{
					Object:     "wrflbrmpfd",
					Annotation: "mac-address",
				},
			},
			expectError: true,
		}),
		Entry("from unknown annotation", testCaseGetLinkMacAddress{
			mac: &infrav1.NetworkLinkEthernetMac{
				FromAnnotation: &infrav1.NetworkLinkEthernetMacFromAnnotation{
					Object:     "machine",
					Annotation: "wrflbrmpfd",
				},
			},
			machine: &clusterv1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: machineName,
					Annotations: map[string]string{
						"mac-address": "12:34:56:78:9A:BD",
					},
				},
			},
			expectError: true,
		}),
		Entry("ill-formed MAC address", testCaseGetLinkMacAddress{
			mac: &infrav1.NetworkLinkEthernetMac{
				FromAnnotation: &infrav1.NetworkLinkEthernetMacFromAnnotation{
					Object:     "machine",
					Annotation: "mac-address",
				},
			},
			machine: &clusterv1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: machineName,
					Annotations: map[string]string{
						"mac-address": "XX:XX:XX:XX:XX:XX",
					},
				},
			},
			expectError: true,
		}),
	)

	type testCaseRenderMetaData struct {
		m3d              *infrav1.Metal3Data
		m3dt             *infrav1.Metal3DataTemplate
		m3m              *infrav1.Metal3Machine
		machine          *clusterv1beta1.Machine
		bmh              *bmov1alpha1.BareMetalHost
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
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName+"-abc", "", ""),
			},
			expectedMetaData: nil,
		}),
		Entry("Full example", testCaseRenderMetaData{
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta("data-abc", namespaceName, ""),
				Spec: infrav1.Metal3DataSpec{
					Index: 2,
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      metal3DataTemplateName + "-abc",
					Namespace: namespaceName,
				},
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						Strings: []infrav1.MetaDataString{
							{
								Key:   "String-1",
								Value: "String-1",
							},
						},
						ObjectNames: []infrav1.MetaDataObjectName{
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
						Namespaces: []infrav1.MetaDataNamespace{
							{
								Key: "Namespace-1",
							},
						},
						Indexes: []infrav1.MetaDataIndex{
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
						IPAddressesFromPool: []infrav1.FromPool{
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
						PrefixesFromPool: []infrav1.FromPool{
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
						GatewaysFromPool: []infrav1.FromPool{
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
						FromHostInterfaces: []infrav1.MetaDataHostInterface{
							{
								Key:       "Mac-1",
								Interface: "eth1",
							},
						},
						FromLabels: []infrav1.MetaDataFromLabel{
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
						FromAnnotations: []infrav1.MetaDataFromAnnotation{
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
			m3m: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      metal3machineName,
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
			machine: &clusterv1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: machineName,
					Labels: map[string]string{
						"Machine": "MachineLabel",
					},
					Annotations: map[string]string{
						"Machine": "MachineAnnotation",
					},
				},
			},
			bmh: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      baremetalhostName,
					Namespace: namespaceName,
					Labels: map[string]string{
						"BMH": "BMHLabel",
					},
					Annotations: map[string]string{
						"BMH": "BMHAnnotation",
					},
					UID: bmhuid,
				},
				Status: bmov1alpha1.BareMetalHostStatus{
					HardwareDetails: &bmov1alpha1.HardwareDetails{
						NIC: []bmov1alpha1.NIC{
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
					Address: "192.168.0.14",
					Prefix:  25,
					Gateway: "192.168.0.1",
				},
				"bcde": {
					Address: "192.168.1.14",
					Prefix:  26,
					Gateway: "192.168.1.1",
				},
			},
			expectedMetaData: map[string]string{
				"String-1":     "String-1",
				"providerid":   fmt.Sprintf("%s/%s/%s", namespaceName, baremetalhostName, metal3machineName),
				"ObjectName-1": machineName,
				"ObjectName-2": metal3machineName,
				"ObjectName-3": baremetalhostName,
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
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName+"-abc", "", ""),
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						FromHostInterfaces: []infrav1.MetaDataHostInterface{
							{
								Key:       "Mac-1",
								Interface: "eth2",
							},
						},
					},
				},
			},
			bmh: &bmov1alpha1.BareMetalHost{
				ObjectMeta: testObjectMeta(baremetalhostName, namespaceName, ""),
				Status: bmov1alpha1.BareMetalHostStatus{
					HardwareDetails: &bmov1alpha1.HardwareDetails{
						NIC: []bmov1alpha1.NIC{
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
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta("data-abc", namespaceName, ""),
				Spec: infrav1.Metal3DataSpec{
					Index: 2,
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName+"-abc", "", ""),
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						IPAddressesFromPool: []infrav1.FromPool{
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
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta("data-abc", namespaceName, ""),
				Spec: infrav1.Metal3DataSpec{
					Index: 2,
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName+"-abc", "", ""),
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						PrefixesFromPool: []infrav1.FromPool{
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
			m3d: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta("data-abc", namespaceName, ""),
				Spec: infrav1.Metal3DataSpec{
					Index: 2,
				},
			},
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName+"-abc", "", ""),
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						GatewaysFromPool: []infrav1.FromPool{
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
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName+"-abc", "", ""),
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						ObjectNames: []infrav1.MetaDataObjectName{
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
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName+"-abc", "", ""),
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						FromLabels: []infrav1.MetaDataFromLabel{
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
			m3dt: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName+"-abc", "", ""),
				Spec: infrav1.Metal3DataTemplateSpec{
					MetaData: &infrav1.MetaData{
						FromAnnotations: []infrav1.MetaDataFromAnnotation{
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
		bmh         *bmov1alpha1.BareMetalHost
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
			bmh: &bmov1alpha1.BareMetalHost{
				Status: bmov1alpha1.BareMetalHostStatus{},
			},
			name:        "eth1",
			expectError: true,
		}),
		Entry("No Nics detail", testCaseGetBMHMacByName{
			bmh: &bmov1alpha1.BareMetalHost{
				Status: bmov1alpha1.BareMetalHostStatus{
					HardwareDetails: &bmov1alpha1.HardwareDetails{},
				},
			},
			name:        "eth1",
			expectError: true,
		}),
		Entry("Empty nic list", testCaseGetBMHMacByName{
			bmh: &bmov1alpha1.BareMetalHost{
				Status: bmov1alpha1.BareMetalHostStatus{
					HardwareDetails: &bmov1alpha1.HardwareDetails{
						NIC: []bmov1alpha1.NIC{},
					},
				},
			},
			name:        "eth1",
			expectError: true,
		}),
		Entry("Nic not found", testCaseGetBMHMacByName{
			bmh: &bmov1alpha1.BareMetalHost{
				Status: bmov1alpha1.BareMetalHostStatus{
					HardwareDetails: &bmov1alpha1.HardwareDetails{
						NIC: []bmov1alpha1.NIC{
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
			bmh: &bmov1alpha1.BareMetalHost{
				Status: bmov1alpha1.BareMetalHostStatus{
					HardwareDetails: &bmov1alpha1.HardwareDetails{
						NIC: []bmov1alpha1.NIC{
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
			bmh: &bmov1alpha1.BareMetalHost{
				Status: bmov1alpha1.BareMetalHostStatus{
					HardwareDetails: &bmov1alpha1.HardwareDetails{
						NIC: []bmov1alpha1.NIC{
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
		Machine       *infrav1.Metal3Machine
		Data          *infrav1.Metal3Data
		DataTemplate  *infrav1.Metal3DataTemplate
		DataClaim     *infrav1.Metal3DataClaim
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
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
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
			Data: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Claim: *testObjectReference(metal3DataClaimName),
				},
			},
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			ExpectRequeue: true,
		}),
		Entry("Data spec unset", testCaseGetM3Machine{
			Data:        &infrav1.Metal3Data{},
			ExpectError: true,
		}),
		Entry("Data Spec name unset", testCaseGetM3Machine{
			Data: &infrav1.Metal3Data{
				Spec: infrav1.Metal3DataSpec{
					Claim: corev1.ObjectReference{},
				},
			},
			ExpectError: true,
		}),
		Entry("Dataclaim Spec ownerref unset", testCaseGetM3Machine{
			Data: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Claim: *testObjectReference(metal3DataClaimName),
				},
			},
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMeta(metal3DataClaimName, namespaceName, m3dcuid),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			ExpectError: true,
		}),
		Entry("M3Machine not found", testCaseGetM3Machine{
			Data: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Claim: *testObjectReference(metal3DataClaimName),
				},
			},
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			ExpectRequeue: true,
		}),
		Entry("Object exists", testCaseGetM3Machine{
			Machine: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, m3muid),
			},
			Data: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Claim: *testObjectReference(metal3DataClaimName),
				},
			},
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
		}),
		Entry("Object exists, dataTemplate nil", testCaseGetM3Machine{
			Machine: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, m3muid),
				Spec: infrav1.Metal3MachineSpec{
					DataTemplate: nil,
				},
			},
			DataTemplate: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
			},
			Data: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Claim: *testObjectReference(metal3DataClaimName),
				},
			},
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			ExpectEmpty: true,
		}),
		Entry("Object exists, dataTemplate name mismatch", testCaseGetM3Machine{
			Machine: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, m3muid),
				Spec: infrav1.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name:      "abcd",
						Namespace: namespaceName,
					},
				},
			},
			DataTemplate: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
			},
			Data: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Claim: *testObjectReference(metal3DataClaimName),
				},
			},
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			ExpectEmpty: true,
		}),
		Entry("Object exists, dataTemplate namespace mismatch", testCaseGetM3Machine{
			Machine: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, m3muid),
				Spec: infrav1.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name:      "abc",
						Namespace: "defg",
					},
				},
			},
			DataTemplate: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
			},
			Data: &infrav1.Metal3Data{
				ObjectMeta: testObjectMetaWithOR(metal3DataName, metal3machineName),
				Spec: infrav1.Metal3DataSpec{
					Claim: *testObjectReference(metal3DataClaimName),
				},
			},
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
				Spec:       infrav1.Metal3DataClaimSpec{},
			},
			ExpectEmpty: true,
		}),
	)
})

type releaseAddressFromPoolFakeClient struct {
	client.Client
	injectDeleteErr bool
}

func (f *releaseAddressFromPoolFakeClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if f.injectDeleteErr {
		return errors.New("failed to delete for some weird reason")
	}
	return f.Client.Delete(ctx, obj, opts...)
}

var _ = Describe("poolRefs map", func() {
	When("the map is empty", func() {
		It("defaults refs to metal3 ipam if not specified", func() {
			refs := poolRefs{}
			Expect(refs.addRef(corev1.TypedLocalObjectReference{Name: "foo"})).To(Succeed())
			Expect(refs["foo"]).To(Equal(corev1.TypedLocalObjectReference{
				Name:     "foo",
				Kind:     "IPPool",
				APIGroup: ptr.To("ipam.metal3.io"),
			}))
		})

		It("defaults refs to metal3 that are added using addName()", func() {
			refs := poolRefs{}
			Expect(refs.addName("foo")).To(Succeed())
			Expect(refs["foo"]).To(Equal(corev1.TypedLocalObjectReference{
				Name:     "foo",
				Kind:     "IPPool",
				APIGroup: ptr.To("ipam.metal3.io"),
			}))
		})

		It("defaults refs to metal3 that are added using addFromPool()", func() {
			refs := poolRefs{}
			Expect(refs.addFromPool(infrav1.FromPool{
				Name: "foo",
			})).To(Succeed())
			Expect(refs["foo"]).To(Equal(corev1.TypedLocalObjectReference{
				Name:     "foo",
				Kind:     "IPPool",
				APIGroup: ptr.To("ipam.metal3.io"),
			}))
		})
	})

	When("the map already contains a ref with a non-default kind", func() {
		var refs poolRefs
		var existing corev1.TypedLocalObjectReference

		BeforeEach(func() {
			existing = corev1.TypedLocalObjectReference{
				Name:     "foo",
				Kind:     "InClusterIPPool",
				APIGroup: ptr.To("ipam.metal3.io"),
			}
			refs = poolRefs{
				"foo": existing,
			}
		})

		It("accepts adding an identical ref again", func() {
			Expect(refs.addRef(existing)).To(Succeed())
			Expect(refs["foo"]).To(Equal(existing))
		})

		It("rejects adding a ref with the same name but different kind", func() {
			Expect(refs.addRef(corev1.TypedLocalObjectReference{
				Name:     "foo",
				Kind:     "IPPool",
				APIGroup: ptr.To("ipam.metal3.io"),
			})).NotTo(Succeed())
			Expect(refs["foo"]).To(Equal(existing))
		})

		It("rejects adding a ref with the same name but different API group", func() {
			Expect(refs.addRef(corev1.TypedLocalObjectReference{
				Name:     "foo",
				Kind:     "InClusterIPPool",
				APIGroup: ptr.To("ipam.cluster.x-k8s.io"),
			})).NotTo(Succeed())
			Expect(refs["foo"]).To(Equal(existing))
		})

		It("rejects adding a ref with the same name but different API group added using addFromPool()", func() {
			Expect(refs.addFromPool(infrav1.FromPool{
				Name:     "foo",
				Kind:     "InClusterIPPool",
				APIGroup: "ipam.cluster.x-k8s.io",
			})).NotTo(Succeed())
			Expect(refs["foo"]).To(Equal(existing))
		})

		It("rejects adding a ref with the same name but default kind added using addFromPool()", func() {
			Expect(refs.addFromPool(infrav1.FromPool{
				Name:     "foo",
				APIGroup: "ipam.metal3.io",
			})).NotTo(Succeed())
			Expect(refs["foo"]).To(Equal(existing))
		})

		It("rejects adding a ref with the same name but default kind/apigroup added using addName()", func() {
			Expect(refs.addName("foo")).NotTo(Succeed())
			Expect(refs["foo"]).To(Equal(existing))
		})
	})
})

var _ = Describe("When using BMH name based pre-allocation", func() {
	var bmhName = "host-0"

	BeforeEach(func() {
		EnableBMHNameBasedPreallocation = true
	})

	AfterEach(func() {
		EnableBMHNameBasedPreallocation = false
	})

	DescribeTable("ensureM3IPClaim", func(tc testCaseEnsureM3Claim) {
		bmh := &bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bmhName,
				Namespace: namespaceName,
			},
		}
		m3m := &infrav1.Metal3Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      metal3machineName,
				Namespace: namespaceName,
				Annotations: map[string]string{
					HostAnnotation: namespaceName + "/" + bmh.Name,
				},
			},
			Spec: infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name:      metal3DataTemplateName,
					Namespace: namespaceName,
				},
			},
		}
		m3dt := &infrav1.Metal3DataTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      metal3DataTemplateName,
				Namespace: namespaceName,
			},
		}
		m3dc := &infrav1.Metal3DataClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      metal3DataClaimName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: infrav1.GroupVersion.Group + "/" + infrav1.GroupVersion.Version,
						Kind:       "Metal3Machine",
						Name:       m3m.Name,
					},
				},
			},
		}
		m3d := &infrav1.Metal3Data{
			TypeMeta: metav1.TypeMeta{
				APIVersion: infrav1.GroupVersion.Group + "/" + infrav1.GroupVersion.Version,
				Kind:       "Metal3Data",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      metal3DataName,
				Namespace: namespaceName,
			},
			Spec: infrav1.Metal3DataSpec{
				Template: corev1.ObjectReference{
					Name:      m3dt.Name,
					Namespace: m3dt.Namespace,
				},
				Claim: corev1.ObjectReference{
					Namespace: namespaceName,
					Name:      metal3DataClaimName,
				},
			},
		}

		// Setup fake client with objects
		objects := []client.Object{bmh, m3m, m3d, m3dt, m3dc}
		if tc.ipClaim != nil {
			objects = append(objects, tc.ipClaim)
		}
		fc := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
		dataMgr, err := NewDataManager(fc, m3d, logr.Discard())
		Expect(err).NotTo(HaveOccurred())

		rc, err := dataMgr.ensureM3IPClaim(context.Background(), tc.poolRef)

		if tc.expectError {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
		Expect(rc.fetchAgain).To(Equal(tc.expectFetchAgain))
		if tc.expectClaim {
			Expect(rc.m3Claim).NotTo(BeNil())
			claim := &ipamv1.IPClaim{}
			nn := types.NamespacedName{
				Name:      bmh.Name + "-" + tc.poolRef.Name,
				Namespace: bmh.Namespace,
			}
			err = fc.Get(context.Background(), nn, claim)
			Expect(err).NotTo(HaveOccurred())

			_, err := findOwnerRefFromList(claim.OwnerReferences,
				m3d.TypeMeta, m3d.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(tc.ipClaim).To(BeNil())
		}
	},
		Entry("should create claim if missing", testCaseEnsureM3Claim{
			poolRef:          corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaim:          nil,
			expectError:      false,
			expectFetchAgain: true,
			expectClaim:      true,
		}),
		Entry("should do nothing when claim exists", testCaseEnsureM3Claim{
			poolRef: corev1.TypedLocalObjectReference{Name: testPoolName},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bmhName + "-" + testPoolName,
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: infrav1.GroupVersion.Group + "/" + infrav1.GroupVersion.Version,
							Kind:       "Metal3Data",
							Name:       metal3DataName,
							Controller: ptr.To(true),
						},
					}},
			},
			expectError:      false,
			expectFetchAgain: false,
			expectClaim:      true,
		}),
	)

})
