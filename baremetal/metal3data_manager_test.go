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
	"gopkg.in/yaml.v2"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	bmo "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
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

var _ = Describe("Metal3Data manager", func() {
	DescribeTable("Test Finalizers",
		func(data *capm3.Metal3Data) {
			machineMgr, err := NewDataManager(nil, data,
				klogr.New(),
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
			klogr.New(),
		)
		Expect(err).NotTo(HaveOccurred())
		dataMgr.setError(context.TODO(), "This is an error")
		Expect(data.Status.Error).To(BeTrue())
		Expect(*data.Status.ErrorMessage).To(Equal("This is an error"))

		dataMgr.clearError(context.TODO())
		Expect(data.Status.Error).To(BeFalse())
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
			objects := []runtime.Object{}
			if tc.m3dt != nil {
				objects = append(objects, tc.m3dt)
			}
			if tc.m3m != nil {
				objects = append(objects, tc.m3m)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			dataMgr, err := NewDataManager(c, tc.m3d,
				klogr.New(),
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
				Expect(tc.m3d.Status.Error).To(BeTrue())
			} else {
				Expect(tc.m3d.Status.Error).To(BeFalse())
			}
		},
		Entry("Clear Error", testCaseReconcile{
			m3d: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{},
				Status: capm3.Metal3DataStatus{
					Error: true,
				},
			},
		}),
		Entry("requeue error", testCaseReconcile{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			expectRequeue: true,
		}),
		Entry("Set error", testCaseReconcile{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
					Metal3Machine: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			m3m: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			expectError:      true,
			expectedErrorSet: true,
		}),
	)

	type testCaseCreateSecrets struct {
		m3d                 *capm3.Metal3Data
		m3dt                *capm3.Metal3DataTemplate
		m3m                 *capm3.Metal3Machine
		machine             *capi.Machine
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
			objects := []runtime.Object{}
			if tc.m3dt != nil {
				objects = append(objects, tc.m3dt)
			}
			if tc.m3m != nil {
				objects = append(objects, tc.m3m)
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
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			dataMgr, err := NewDataManager(c, tc.m3d,
				klogr.New(),
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
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.expectReady {
				Expect(tc.m3d.Status.Ready).To(BeTrue())
			} else {
				Expect(tc.m3d.Status.Ready).To(BeFalse())
			}
			if tc.expectedMetadata != nil {
				tmpSecret := corev1.Secret{}
				err = c.Get(context.TODO(),
					client.ObjectKey{
						Name:      "abc-metadata",
						Namespace: "def",
					},
					&tmpSecret,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(tmpSecret.Data["metaData"])).To(Equal(*tc.expectedMetadata))
			}
			if tc.expectedNetworkData != nil {
				tmpSecret := corev1.Secret{}
				err = c.Get(context.TODO(),
					client.ObjectKey{
						Name:      "abc-networkdata",
						Namespace: "def",
					},
					&tmpSecret,
				)
				Expect(err).NotTo(HaveOccurred())
				fmt.Println(string(tmpSecret.Data["networkData"]))
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
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			expectRequeue: true,
		}),
		Entry("No Metal3Machine", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
					Metal3Machine: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			expectRequeue: true,
		}),
		Entry("No Secret needed", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
					Metal3Machine: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			m3m: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			expectReady: true,
		}),
		Entry("Machine without datatemplate", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
					Metal3Machine: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			m3m: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			expectError: true,
		}),
		Entry("secrets exist", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
					Metal3Machine: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						Strings: []capm3.MetaDataString{
							capm3.MetaDataString{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &capm3.NetworkData{
						Links: capm3.NetworkDataLink{
							Ethernets: []capm3.NetworkDataLinkEthernet{
								capm3.NetworkDataLinkEthernet{
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
					Namespace: "def",
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			metadataSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc-metadata",
					Namespace: "def",
				},
				Data: map[string][]byte{
					"metaData": []byte("Hello"),
				},
			},
			networkdataSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc-networkdata",
					Namespace: "def",
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
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
					Metal3Machine: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						Strings: []capm3.MetaDataString{
							capm3.MetaDataString{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &capm3.NetworkData{
						Links: capm3.NetworkDataLink{
							Ethernets: []capm3.NetworkDataLinkEthernet{
								capm3.NetworkDataLinkEthernet{
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
					Namespace: "def",
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: capi.GroupVersion.String(),
						},
					},
					Annotations: map[string]string{
						"metal3.io/BareMetalHost": "def/abc",
					},
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			bmh: &bmo.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			expectReady:         true,
			expectedMetadata:    pointer.StringPtr("String-1: String-1\n"),
			expectedNetworkData: pointer.StringPtr("links:\n- ethernet_mac_address: XX:XX:XX:XX:XX:XX\n  id: eth0\n  mtu: 1500\n  type: phy\nnetworks: []\nservices: []\n"),
		}),
		Entry("No Machine OwnerRef on M3M", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
					Metal3Machine: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						Strings: []capm3.MetaDataString{
							capm3.MetaDataString{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &capm3.NetworkData{
						Links: capm3.NetworkDataLink{
							Ethernets: []capm3.NetworkDataLinkEthernet{
								capm3.NetworkDataLinkEthernet{
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
					Namespace: "def",
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			expectRequeue: true,
		}),
		Entry("secrets do not exist", testCaseCreateSecrets{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
					Metal3Machine: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						Strings: []capm3.MetaDataString{
							capm3.MetaDataString{
								Key:   "String-1",
								Value: "String-1",
							},
						},
					},
					NetworkData: &capm3.NetworkData{
						Links: capm3.NetworkDataLink{
							Ethernets: []capm3.NetworkDataLinkEthernet{
								capm3.NetworkDataLinkEthernet{
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
					Namespace: "def",
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: capi.GroupVersion.String(),
						},
					},
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			expectRequeue: true,
		}),
	)

	type testCaseRenderNetworkData struct {
		m3d            *capm3.Metal3Data
		m3dt           *capm3.Metal3DataTemplate
		bmh            *bmo.BareMetalHost
		expectError    bool
		expectedOutput map[string][]interface{}
	}

	DescribeTable("Test renderNetworkData",
		func(tc testCaseRenderNetworkData) {
			result, err := renderNetworkData(tc.m3d, tc.m3dt, tc.bmh)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
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
								capm3.NetworkDataLinkEthernet{
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
								capm3.NetworkDataIPv4{
									ID:      "abc",
									Link:    "def",
									Netmask: 24,
									IPAddress: capm3.NetworkDataIPAddressv4{
										Start:  "192.168.0.10",
										End:    "192.168.0.250",
										Subnet: "192.168.0.0/24",
										Step:   2,
									},
									Routes: []capm3.NetworkDataRoutev4{
										capm3.NetworkDataRoutev4{
											Network: "10.0.0.0",
											Netmask: 16,
											Gateway: "192.168.1.1",
											Services: capm3.NetworkDataServicev4{
												DNS: []capm3.NetworkDataDNSServicev4{
													capm3.NetworkDataDNSServicev4("8.8.8.8"),
												},
											},
										},
									},
								},
							},
						},
						Services: capm3.NetworkDataService{
							DNS: []capm3.NetworkDataDNSService{
								capm3.NetworkDataDNSService("8.8.8.8"),
								capm3.NetworkDataDNSService("2001::8888"),
							},
						},
					},
				},
			},
			expectedOutput: map[string][]interface{}{
				"services": []interface{}{
					map[interface{}]interface{}{
						"type":    "dns",
						"address": "8.8.8.8",
					},
					map[interface{}]interface{}{
						"type":    "dns",
						"address": "2001::8888",
					},
				},
				"links": []interface{}{
					map[interface{}]interface{}{
						"type":                 "phy",
						"id":                   "eth0",
						"mtu":                  1500,
						"ethernet_mac_address": "XX:XX:XX:XX:XX:XX",
					},
				},
				"networks": []interface{}{
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
								capm3.NetworkDataLinkEthernet{
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
		Entry("Full example", testCaseRenderNetworkData{
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
								capm3.NetworkDataIPv4{
									ID:      "abc",
									Link:    "def",
									Netmask: 24,
									IPAddress: capm3.NetworkDataIPAddressv4{
										Start:  "192.168.0.10",
										End:    "192.168.0.11",
										Subnet: "192.168.0.0/24",
										Step:   2,
									},
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
			DNS: []capm3.NetworkDataDNSService{
				capm3.NetworkDataDNSService("8.8.8.8"),
				capm3.NetworkDataDNSService("2001::8888"),
			},
		}
		expectedOutput := []interface{}{
			map[string]string{
				"type":    "dns",
				"address": "8.8.8.8",
			},
			map[string]string{
				"type":    "dns",
				"address": "2001::8888",
			},
		}
		result := renderNetworkServices(services)
		Expect(result).To(Equal(expectedOutput))
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
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(result).To(Equal(tc.expectedOutput))
		},
		Entry("Ethernet, MAC from string", testCaseRenderNetworkLinks{
			links: capm3.NetworkDataLink{
				Ethernets: []capm3.NetworkDataLinkEthernet{
					capm3.NetworkDataLinkEthernet{
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
					capm3.NetworkDataLinkEthernet{
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
					capm3.NetworkDataLinkBond{
						BondMode: "802.1ad",
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
					"bond_mode":            "802.1ad",
					"bond_links":           []string{"eth0"},
				},
			},
		}),
		Entry("Bond, MAC error", testCaseRenderNetworkLinks{
			links: capm3.NetworkDataLink{
				Bonds: []capm3.NetworkDataLinkBond{
					capm3.NetworkDataLinkBond{
						BondMode: "802.1ad",
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
					capm3.NetworkDataLinkVlan{
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
					capm3.NetworkDataLinkVlan{
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
		expectError    bool
		expectedOutput []interface{}
	}

	DescribeTable("Test renderNetworkNetworks",
		func(tc testCaseRenderNetworkNetworks) {
			result, err := renderNetworkNetworks(tc.networks, tc.m3d)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(result).To(Equal(tc.expectedOutput))
		},
		Entry("IPv4 network", testCaseRenderNetworkNetworks{
			networks: capm3.NetworkDataNetwork{
				IPv4: []capm3.NetworkDataIPv4{
					capm3.NetworkDataIPv4{
						ID:      "abc",
						Link:    "def",
						Netmask: 24,
						IPAddress: capm3.NetworkDataIPAddressv4{
							Start:  "192.168.0.10",
							End:    "192.168.0.250",
							Subnet: "192.168.0.0/24",
							Step:   2,
						},
						Routes: []capm3.NetworkDataRoutev4{
							capm3.NetworkDataRoutev4{
								Network: "10.0.0.0",
								Netmask: 16,
								Gateway: "192.168.1.1",
								Services: capm3.NetworkDataServicev4{
									DNS: []capm3.NetworkDataDNSServicev4{
										capm3.NetworkDataDNSServicev4("8.8.8.8"),
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
					"ip_address": "192.168.0.14",
					"routes": []interface{}{
						map[string]interface{}{
							"network": "10.0.0.0",
							"netmask": "255.255.0.0",
							"gateway": "192.168.1.1",
							"services": []interface{}{
								map[string]string{
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
		}),
		Entry("IPv4 network", testCaseRenderNetworkNetworks{
			networks: capm3.NetworkDataNetwork{
				IPv4: []capm3.NetworkDataIPv4{
					capm3.NetworkDataIPv4{
						IPAddress: capm3.NetworkDataIPAddressv4{
							Start:  "192.168.0.10",
							End:    "192.168.0.250",
							Subnet: "192.168.0.0/24",
							Step:   2,
						},
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
			networks: capm3.NetworkDataNetwork{
				IPv6: []capm3.NetworkDataIPv6{
					capm3.NetworkDataIPv6{
						ID:      "abc",
						Link:    "def",
						Netmask: 96,
						IPAddress: capm3.NetworkDataIPAddressv6{
							Start:  "fe80::2001:10",
							End:    "fe80::2001:ff00",
							Subnet: "fe80::2001:0/96",
							Step:   20,
						},
						Routes: []capm3.NetworkDataRoutev6{
							capm3.NetworkDataRoutev6{
								Network: "2001::",
								Netmask: 64,
								Gateway: "fe80::2001:1",
								Services: capm3.NetworkDataServicev6{
									DNS: []capm3.NetworkDataDNSServicev6{
										capm3.NetworkDataDNSServicev6("2001::8888"),
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
					"ip_address": "fe80::2001:38",
					"routes": []interface{}{
						map[string]interface{}{
							"network": "2001::",
							"netmask": "ffff:ffff:ffff:ffff::",
							"gateway": "fe80::2001:1",
							"services": []interface{}{
								map[string]string{
									"type":    "dns",
									"address": "2001::8888",
								},
							},
						},
					},
					"type":    "ipv6",
					"id":      "abc",
					"link":    "def",
					"netmask": "ffff:ffff:ffff:ffff:ffff:ffff::",
				},
			},
		}),
		Entry("IPv6 network", testCaseRenderNetworkNetworks{
			networks: capm3.NetworkDataNetwork{
				IPv6: []capm3.NetworkDataIPv6{
					capm3.NetworkDataIPv6{
						IPAddress: capm3.NetworkDataIPAddressv6{
							Start:  "fe80::2001:10",
							End:    "fe80::2001:ff00",
							Subnet: "fe80::2001:0/96",
							Step:   20,
						},
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
					capm3.NetworkDataIPv4DHCP{
						ID:   "abc",
						Link: "def",
						Routes: []capm3.NetworkDataRoutev4{
							capm3.NetworkDataRoutev4{
								Network: "10.0.0.0",
								Netmask: 16,
								Gateway: "192.168.1.1",
								Services: capm3.NetworkDataServicev4{
									DNS: []capm3.NetworkDataDNSServicev4{
										capm3.NetworkDataDNSServicev4("8.8.8.8"),
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
							"network": "10.0.0.0",
							"netmask": "255.255.0.0",
							"gateway": "192.168.1.1",
							"services": []interface{}{
								map[string]string{
									"type":    "dns",
									"address": "8.8.8.8",
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
					capm3.NetworkDataIPv6DHCP{
						ID:   "abc",
						Link: "def",
						Routes: []capm3.NetworkDataRoutev6{
							capm3.NetworkDataRoutev6{
								Network: "2001::",
								Netmask: 64,
								Gateway: "fe80::2001:1",
								Services: capm3.NetworkDataServicev6{
									DNS: []capm3.NetworkDataDNSServicev6{
										capm3.NetworkDataDNSServicev6("2001::8888"),
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
							"network": "2001::",
							"netmask": "ffff:ffff:ffff:ffff::",
							"gateway": "fe80::2001:1",
							"services": []interface{}{
								map[string]string{
									"type":    "dns",
									"address": "2001::8888",
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
		Entry("IPv6 DHCP", testCaseRenderNetworkNetworks{
			networks: capm3.NetworkDataNetwork{
				IPv6SLAAC: []capm3.NetworkDataIPv6DHCP{
					capm3.NetworkDataIPv6DHCP{
						ID:   "abc",
						Link: "def",
						Routes: []capm3.NetworkDataRoutev6{
							capm3.NetworkDataRoutev6{
								Network: "2001::",
								Netmask: 64,
								Gateway: "fe80::2001:1",
								Services: capm3.NetworkDataServicev6{
									DNS: []capm3.NetworkDataDNSServicev6{
										capm3.NetworkDataDNSServicev6("2001::8888"),
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
							"network": "2001::",
							"netmask": "ffff:ffff:ffff:ffff::",
							"gateway": "fe80::2001:1",
							"services": []interface{}{
								map[string]string{
									"type":    "dns",
									"address": "2001::8888",
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
			capm3.NetworkDataRoutev4{
				Network: "192.168.0.0",
				Netmask: 24,
				Gateway: "192.168.1.1",
			},
			capm3.NetworkDataRoutev4{
				Network: "10.0.0.0",
				Netmask: 16,
				Gateway: "192.168.1.1",
				Services: capm3.NetworkDataServicev4{
					DNS: []capm3.NetworkDataDNSServicev4{
						capm3.NetworkDataDNSServicev4("8.8.8.8"),
						capm3.NetworkDataDNSServicev4("8.8.4.4"),
					},
				},
			},
		}
		ExpectedOutput := []interface{}{
			map[string]interface{}{
				"network":  "192.168.0.0",
				"netmask":  "255.255.255.0",
				"gateway":  "192.168.1.1",
				"services": []interface{}{},
			},
			map[string]interface{}{
				"network": "10.0.0.0",
				"netmask": "255.255.0.0",
				"gateway": "192.168.1.1",
				"services": []interface{}{
					map[string]string{
						"type":    "dns",
						"address": "8.8.8.8",
					},
					map[string]string{
						"type":    "dns",
						"address": "8.8.4.4",
					},
				},
			},
		}
		Expect(getRoutesv4(netRoutes)).To(Equal(ExpectedOutput))
	})

	It("Test getRoutesv6", func() {
		netRoutes := []capm3.NetworkDataRoutev6{
			capm3.NetworkDataRoutev6{
				Network: "2001::0",
				Netmask: 96,
				Gateway: "2001::1",
			},
			capm3.NetworkDataRoutev6{
				Network: "fe80::0",
				Netmask: 64,
				Gateway: "fe80::1",
				Services: capm3.NetworkDataServicev6{
					DNS: []capm3.NetworkDataDNSServicev6{
						capm3.NetworkDataDNSServicev6("fe80:2001::8888"),
						capm3.NetworkDataDNSServicev6("fe80:2001::8844"),
					},
				},
			},
		}
		ExpectedOutput := []interface{}{
			map[string]interface{}{
				"network":  "2001::0",
				"netmask":  "ffff:ffff:ffff:ffff:ffff:ffff::",
				"gateway":  "2001::1",
				"services": []interface{}{},
			},
			map[string]interface{}{
				"network": "fe80::0",
				"netmask": "ffff:ffff:ffff:ffff::",
				"gateway": "fe80::1",
				"services": []interface{}{
					map[string]string{
						"type":    "dns",
						"address": "fe80:2001::8888",
					},
					map[string]string{
						"type":    "dns",
						"address": "fe80:2001::8844",
					},
				},
			},
		}
		Expect(getRoutesv6(netRoutes)).To(Equal(ExpectedOutput))
	})

	type testCaseTranslateMask struct {
		mask         int
		ipv4         bool
		expectedMask string
	}

	DescribeTable("Test translateMask",
		func(tc testCaseTranslateMask) {
			Expect(translateMask(tc.mask, tc.ipv4)).To(Equal(tc.expectedMask))
		},
		Entry("IPv4 mask 24", testCaseTranslateMask{
			mask:         24,
			ipv4:         true,
			expectedMask: "255.255.255.0",
		}),
		Entry("IPv4 mask 16", testCaseTranslateMask{
			mask:         16,
			ipv4:         true,
			expectedMask: "255.255.0.0",
		}),
		Entry("IPv6 mask 64", testCaseTranslateMask{
			mask:         64,
			expectedMask: "ffff:ffff:ffff:ffff::",
		}),
		Entry("IPv6 mask 96", testCaseTranslateMask{
			mask:         96,
			expectedMask: "ffff:ffff:ffff:ffff:ffff:ffff::",
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
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
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
							bmo.NIC{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// Check if empty value cause failure
							bmo.NIC{},
							bmo.NIC{
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
							bmo.NIC{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// Check if empty value cause failure
							bmo.NIC{},
							bmo.NIC{
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
		machine          *capi.Machine
		bmh              *bmo.BareMetalHost
		expectedMetaData map[string]string
		expectError      bool
	}

	DescribeTable("Test renderMetaData",
		func(tc testCaseRenderMetaData) {
			resultBytes, err := renderMetaData(tc.m3d, tc.m3dt, tc.m3m, tc.machine,
				tc.bmh,
			)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
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
					Namespace: "def",
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
						Strings: []capm3.MetaDataString{
							capm3.MetaDataString{
								Key:   "String-1",
								Value: "String-1",
							},
						},
						ObjectNames: []capm3.MetaDataObjectName{
							capm3.MetaDataObjectName{
								Key:    "ObjectName-1",
								Object: "machine",
							},
							capm3.MetaDataObjectName{
								Key:    "ObjectName-2",
								Object: "metal3machine",
							},
							capm3.MetaDataObjectName{
								Key:    "ObjectName-3",
								Object: "baremetalhost",
							},
						},
						Namespaces: []capm3.MetaDataNamespace{
							capm3.MetaDataNamespace{
								Key: "Namespace-1",
							},
						},
						Indexes: []capm3.MetaDataIndex{
							capm3.MetaDataIndex{
								Key:    "Index-1",
								Offset: 10,
								Step:   2,
								Prefix: "abc",
								Suffix: "def",
							},
							capm3.MetaDataIndex{
								Key: "Index-2",
							},
						},
						IPAddresses: []capm3.MetaDataIPAddress{
							capm3.MetaDataIPAddress{
								Key:    "Address-1",
								Start:  pointer.StringPtr("192.168.0.10"),
								End:    pointer.StringPtr("192.168.0.250"),
								Subnet: pointer.StringPtr("192.168.0.0/24"),
								Step:   2,
							},
						},
						FromHostInterfaces: []capm3.MetaDataHostInterface{
							capm3.MetaDataHostInterface{
								Key:       "Mac-1",
								Interface: "eth1",
							},
						},
						FromLabels: []capm3.MetaDataFromLabel{
							capm3.MetaDataFromLabel{
								Key:    "Label-1",
								Object: "metal3machine",
								Label:  "Doesnotexist",
							},
							capm3.MetaDataFromLabel{
								Key:    "Label-2",
								Object: "metal3machine",
								Label:  "Empty",
							},
							capm3.MetaDataFromLabel{
								Key:    "Label-3",
								Object: "metal3machine",
								Label:  "M3M",
							},
							capm3.MetaDataFromLabel{
								Key:    "Label-4",
								Object: "machine",
								Label:  "Machine",
							},
							capm3.MetaDataFromLabel{
								Key:    "Label-5",
								Object: "baremetalhost",
								Label:  "BMH",
							},
						},
					},
				},
			},
			m3m: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metal3machine-abc",
					Labels: map[string]string{
						"M3M":   "Metal3MachineLabel",
						"Empty": "",
					},
				},
			},
			machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machine-abc",
					Labels: map[string]string{
						"Machine": "MachineLabel",
					},
				},
			},
			bmh: &bmo.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bmh-abc",
					Labels: map[string]string{
						"BMH": "BMHLabel",
					},
				},
				Status: bmo.BareMetalHostStatus{
					HardwareDetails: &bmo.HardwareDetails{
						NIC: []bmo.NIC{
							bmo.NIC{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// Check if empty value cause failure
							bmo.NIC{},
							bmo.NIC{
								Name: "eth1",
								MAC:  "XX:XX:XX:XX:XX:YY",
							},
						},
					},
				},
			},
			expectedMetaData: map[string]string{
				"String-1":     "String-1",
				"ObjectName-1": "machine-abc",
				"ObjectName-2": "metal3machine-abc",
				"ObjectName-3": "bmh-abc",
				"Namespace-1":  "def",
				"Index-1":      "abc14def",
				"Index-2":      "2",
				"Address-1":    "192.168.0.14",
				"Mac-1":        "XX:XX:XX:XX:XX:YY",
				"Label-1":      "",
				"Label-2":      "",
				"Label-3":      "Metal3MachineLabel",
				"Label-4":      "MachineLabel",
				"Label-5":      "BMHLabel",
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
							capm3.MetaDataHostInterface{
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
							bmo.NIC{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// Check if empty value cause failure
							bmo.NIC{},
							bmo.NIC{
								Name: "eth1",
								MAC:  "XX:XX:XX:XX:XX:YY",
							},
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("IP out of bounds", testCaseRenderMetaData{
			m3d: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "data-abc",
					Namespace: "def",
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
						IPAddresses: []capm3.MetaDataIPAddress{
							capm3.MetaDataIPAddress{
								Key:   "Address-1",
								Start: pointer.StringPtr("192.168.1.10"),
								End:   pointer.StringPtr("192.168.0.250"),
								Step:  2,
							},
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("Full example", testCaseRenderMetaData{
			m3dt: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "datatemplate-abc",
				},
				Spec: capm3.Metal3DataTemplateSpec{
					MetaData: &capm3.MetaData{
						ObjectNames: []capm3.MetaDataObjectName{
							capm3.MetaDataObjectName{
								Key:    "ObjectName-3",
								Object: "baremetalhost2",
							},
						},
					},
				},
			},
			expectError: true,
		}),
	)

	type testCaseGetIPAddress struct {
		ipAddress   *capm3.MetaDataIPAddress
		index       int
		expectError bool
		expectedIP  string
	}

	DescribeTable("Test getIPAddress",
		func(tc testCaseGetIPAddress) {
			result, err := getIPAddress(tc.ipAddress, tc.index)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(tc.expectedIP))
			}
		},
		Entry("Empty Start and Subnet", testCaseGetIPAddress{
			ipAddress:   &capm3.MetaDataIPAddress{},
			index:       1,
			expectError: true,
		}),
		Entry("Start set, no end or subnet", testCaseGetIPAddress{
			ipAddress: &capm3.MetaDataIPAddress{
				Start: pointer.StringPtr("192.168.0.10"),
			},
			index:      1,
			expectedIP: "192.168.0.11",
		}),
		Entry("Start set, end set, subnet unset", testCaseGetIPAddress{
			ipAddress: &capm3.MetaDataIPAddress{
				Start: pointer.StringPtr("192.168.0.10"),
				End:   pointer.StringPtr("192.168.0.100"),
				Step:  1,
			},
			index:      1,
			expectedIP: "192.168.0.11",
		}),
		Entry("Start set, end set, subnet unset, out of bound", testCaseGetIPAddress{
			ipAddress: &capm3.MetaDataIPAddress{
				Start: pointer.StringPtr("192.168.0.10"),
				End:   pointer.StringPtr("192.168.0.100"),
				Step:  1,
			},
			index:       100,
			expectError: true,
		}),
		Entry("Start set, end unset, subnet set", testCaseGetIPAddress{
			ipAddress: &capm3.MetaDataIPAddress{
				Start:  pointer.StringPtr("192.168.0.10"),
				Subnet: pointer.StringPtr("192.168.0.0/24"),
				Step:   1,
			},
			index:      1,
			expectedIP: "192.168.0.11",
		}),
		Entry("Start set, end unset, subnet set, out of bound", testCaseGetIPAddress{
			ipAddress: &capm3.MetaDataIPAddress{
				Start:  pointer.StringPtr("192.168.0.10"),
				Subnet: pointer.StringPtr("192.168.0.0/24"),
				Step:   1,
			},
			index:       250,
			expectError: true,
		}),
		Entry("Start set, end unset, subnet empty", testCaseGetIPAddress{
			ipAddress: &capm3.MetaDataIPAddress{
				Start:  pointer.StringPtr("192.168.0.10"),
				Subnet: pointer.StringPtr(""),
				Step:   1,
			},
			index:       1,
			expectError: true,
		}),
		Entry("subnet empty", testCaseGetIPAddress{
			ipAddress: &capm3.MetaDataIPAddress{
				Subnet: pointer.StringPtr(""),
				Step:   1,
			},
			index:       1,
			expectError: true,
		}),
		Entry("Start unset, end unset, subnet set", testCaseGetIPAddress{
			ipAddress: &capm3.MetaDataIPAddress{
				Subnet: pointer.StringPtr("192.168.0.10/24"),
				Step:   1,
			},
			index:      1,
			expectedIP: "192.168.0.12",
		}),
		Entry("Start unset, end unset, subnet set, out of bound", testCaseGetIPAddress{
			ipAddress: &capm3.MetaDataIPAddress{
				Subnet: pointer.StringPtr("192.168.0.10/24"),
				Step:   1,
			},
			index:       250,
			expectError: true,
		}),
	)

	type testCaseAddOffsetToIP struct {
		ip          string
		endIP       string
		offset      int
		expectedIP  string
		expectError bool
	}

	DescribeTable("Test AddOffsetToIP",
		func(tc testCaseAddOffsetToIP) {
			testIP := net.ParseIP(tc.ip)
			testEndIP := net.ParseIP(tc.endIP)
			expectedIP := net.ParseIP(tc.expectedIP)

			result, err := addOffsetToIP(testIP, testEndIP, tc.offset)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedIP))
			}
		},
		Entry("valid IPv4", testCaseAddOffsetToIP{
			ip:         "192.168.0.10",
			endIP:      "192.168.0.200",
			offset:     10,
			expectedIP: "192.168.0.20",
		}),
		Entry("valid IPv4, no end ip", testCaseAddOffsetToIP{
			ip:         "192.168.0.10",
			offset:     1000,
			expectedIP: "192.168.3.242",
		}),
		Entry("Over bound ipv4", testCaseAddOffsetToIP{
			ip:          "192.168.0.10",
			endIP:       "192.168.0.200",
			offset:      1000,
			expectError: true,
		}),
		Entry("error ipv4", testCaseAddOffsetToIP{
			ip:          "255.255.255.250",
			offset:      10,
			expectError: true,
		}),
		Entry("valid IPv6", testCaseAddOffsetToIP{
			ip:         "2001::10",
			endIP:      "2001::fff0",
			offset:     10,
			expectedIP: "2001::1A",
		}),
		Entry("valid IPv6, no end ip", testCaseAddOffsetToIP{
			ip:         "2001::10",
			offset:     10000,
			expectedIP: "2001::2720",
		}),
		Entry("Over bound ipv6", testCaseAddOffsetToIP{
			ip:          "2001::10",
			endIP:       "2001::00f0",
			offset:      10000,
			expectError: true,
		}),
		Entry("error ipv6", testCaseAddOffsetToIP{
			ip:          "FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFF0",
			offset:      100,
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
							bmo.NIC{
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
							bmo.NIC{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// Check if empty value cause failure
							bmo.NIC{},
							bmo.NIC{
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
							bmo.NIC{
								Name: "eth0",
								MAC:  "XX:XX:XX:XX:XX:XX",
							},
							// Check if empty value cause failure
							bmo.NIC{},
							bmo.NIC{
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
		ExpectError   bool
		ExpectRequeue bool
		ExpectEmpty   bool
	}

	DescribeTable("Test getM3Machine",
		func(tc testCaseGetM3Machine) {
			c := k8sClient
			if tc.Machine != nil {
				err := c.Create(context.TODO(), tc.Machine)
				Expect(err).NotTo(HaveOccurred())
			}

			machineMgr, err := NewDataManager(c, tc.Data,
				klogr.New(),
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
				err = c.Delete(context.TODO(), tc.Machine)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Object does not exist", testCaseGetM3Machine{
			Data: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					Metal3Machine: &corev1.ObjectReference{
						Name:      "abc",
						Namespace: "def",
					},
				},
			},
			ExpectRequeue: true,
		}),
		Entry("Data spec unset", testCaseGetM3Machine{
			Data:        &capm3.Metal3Data{},
			ExpectEmpty: true,
		}),
		Entry("Data Spec name unset", testCaseGetM3Machine{
			Data: &capm3.Metal3Data{
				Spec: capm3.Metal3DataSpec{
					Metal3Machine: &corev1.ObjectReference{},
				},
			},
			ExpectError: true,
		}),
		Entry("Object exists", testCaseGetM3Machine{
			Machine: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			Data: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					Metal3Machine: &corev1.ObjectReference{
						Name:      "abc",
						Namespace: "def",
					},
				},
			},
		}),
		Entry("Object exists, dataTemplate nil", testCaseGetM3Machine{
			Machine: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: nil,
				},
			},
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			Data: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					Metal3Machine: &corev1.ObjectReference{
						Name:      "abc",
						Namespace: "def",
					},
				},
			},
			ExpectEmpty: true,
		}),
		Entry("Object exists, dataTemplate name mismatch", testCaseGetM3Machine{
			Machine: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name:      "abcd",
						Namespace: "def",
					},
				},
			},
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			Data: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					Metal3Machine: &corev1.ObjectReference{
						Name:      "abc",
						Namespace: "def",
					},
				},
			},
			ExpectEmpty: true,
		}),
		Entry("Object exists, dataTemplate namespace mismatch", testCaseGetM3Machine{
			Machine: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name:      "abc",
						Namespace: "defg",
					},
				},
			},
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "def",
				},
			},
			Data: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "def",
				},
				Spec: capm3.Metal3DataSpec{
					Metal3Machine: &corev1.ObjectReference{
						Name:      "abc",
						Namespace: "def",
					},
				},
			},
			ExpectEmpty: true,
		}),
	)
})
