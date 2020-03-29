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
	"encoding/base64"
	"fmt"
	"log"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	bmoapis "github.com/metal3-io/baremetal-operator/pkg/apis"
	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha3"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientfake "k8s.io/client-go/kubernetes/fake"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testImageURL           = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2"
	testImageChecksumURL   = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum"
	testUserDataSecretName = "worker-user-data"
)

var ProviderID = "metal3://12345ID6789"
var CloudInitData = []byte("metal3:cloudInitData1010101test__hello")

func bmmSpec() *capm3.Metal3MachineSpec {
	return &capm3.Metal3MachineSpec{
		ProviderID: &ProviderID,
	}
}

func bmmSpecAll() *capm3.Metal3MachineSpec {
	return &capm3.Metal3MachineSpec{
		ProviderID: &ProviderID,
		UserData: &corev1.SecretReference{
			Name:      "mybmmachine-user-data",
			Namespace: "myns",
		},
		Image: capm3.Image{
			URL:      testImageURL,
			Checksum: testImageChecksumURL,
		},
		HostSelector: capm3.HostSelector{},
	}
}

func bmmSecret() *capm3.Metal3MachineSpec {
	return &capm3.Metal3MachineSpec{
		UserData: &corev1.SecretReference{
			Name:      "mybmmachine-user-data",
			Namespace: "myns",
		},
	}
}

func consumerRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name:       "mybmmachine",
		Namespace:  "myns",
		Kind:       "BMMachine",
		APIVersion: capm3.GroupVersion.String(),
	}
}

func consumerRefSome() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name:       "someoneelsesmachine",
		Namespace:  "myns",
		Kind:       "BMMachine",
		APIVersion: capi.GroupVersion.String(),
	}
}

func expectedImg() *bmh.Image {
	return &bmh.Image{
		URL:      testImageURL,
		Checksum: testImageChecksumURL,
	}
}

func expectedImgTest() *bmh.Image {
	return &bmh.Image{
		URL:      testImageURL + "test",
		Checksum: testImageChecksumURL + "test",
	}
}

func bmhSpec() *bmh.BareMetalHostSpec {
	return &bmh.BareMetalHostSpec{
		ConsumerRef: consumerRef(),
		Image: &bmh.Image{
			URL: "myimage",
		},
	}
}

func bmhSpecBMC() *bmh.BareMetalHostSpec {
	return &bmh.BareMetalHostSpec{
		ConsumerRef: consumerRef(),
		BMC: bmh.BMCDetails{
			Address:         "myAddress",
			CredentialsName: "mycredentials",
		},
	}
}

func bmhSpecTestImg() *bmh.BareMetalHostSpec {
	return &bmh.BareMetalHostSpec{
		ConsumerRef: consumerRef(),
		Image:       expectedImgTest(),
	}
}

func bmhSpecSomeImg() *bmh.BareMetalHostSpec {
	return &bmh.BareMetalHostSpec{
		ConsumerRef: consumerRefSome(),
		Image: &bmh.Image{
			URL: "someoneelsesimage",
		},
	}
}

func bmhSpecNoImg() *bmh.BareMetalHostSpec {
	return &bmh.BareMetalHostSpec{
		ConsumerRef: consumerRef(),
	}
}

func bmmObjectMetaWithValidAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "mybmmachine",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
		Annotations: map[string]string{
			HostAnnotation: "myns/myhost",
		},
	}
}

func bmmObjectMetaWithInvalidAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "foobarbmmachine",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
		Annotations: map[string]string{
			HostAnnotation: "myns/wrongvalue",
		},
	}
}

func bmmObjectMetaWithSomeAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "bmmachine",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
		Annotations: map[string]string{
			HostAnnotation: "myns/somehost",
		},
	}
}

func bmmObjectMetaEmptyAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "bmmachine",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
		Annotations:     map[string]string{},
	}
}

func bmmObjectMetaNoAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "bmmachine",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
	}
}

func bmhPowerStatus() *bmh.BareMetalHostStatus {
	return &bmh.BareMetalHostStatus{
		Provisioning: bmh.ProvisionStatus{
			State: bmh.StateNone,
		},
		PoweredOn: true,
	}
}

func bmhStatus() *bmh.BareMetalHostStatus {
	return &bmh.BareMetalHostStatus{
		Provisioning: bmh.ProvisionStatus{
			State: bmh.StateNone,
		},
	}
}

var _ = Describe("Metal3Machine manager", func() {
	DescribeTable("Test Finalizers",
		func(bmMachine capm3.Metal3Machine) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &bmMachine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			machineMgr.SetFinalizer()

			Expect(bmMachine.ObjectMeta.Finalizers).To(ContainElement(
				capm3.MachineFinalizer,
			))

			machineMgr.UnsetFinalizer()

			Expect(bmMachine.ObjectMeta.Finalizers).NotTo(ContainElement(
				capm3.MachineFinalizer,
			))
		},
		Entry("No finalizers", capm3.Metal3Machine{}),
		Entry("Additional Finalizers", capm3.Metal3Machine{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"foo"},
			},
		}),
	)

	DescribeTable("Test SetProviderID",
		func(bmMachine capm3.Metal3Machine) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &bmMachine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			machineMgr.SetProviderID("correct")

			Expect(*bmMachine.Spec.ProviderID).To(Equal("correct"))
			Expect(bmMachine.Status.Ready).To(BeTrue())
		},
		Entry("no ProviderID", capm3.Metal3Machine{}),
		Entry("existing ProviderID", capm3.Metal3Machine{
			Spec: capm3.Metal3MachineSpec{
				ProviderID: pointer.StringPtr("wrong"),
			},
			Status: capm3.Metal3MachineStatus{
				Ready: true,
			},
		}),
	)

	type testCaseProvisioned struct {
		BMMachine  capm3.Metal3Machine
		ExpectTrue bool
	}

	DescribeTable("Test IsProvisioned",
		func(tc testCaseProvisioned) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &tc.BMMachine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			provisioningState := machineMgr.IsProvisioned()

			Expect(provisioningState).To(Equal(tc.ExpectTrue))
		},
		Entry("provisioned", testCaseProvisioned{
			BMMachine: capm3.Metal3Machine{
				Spec: capm3.Metal3MachineSpec{
					ProviderID: pointer.StringPtr("abc"),
				},
				Status: capm3.Metal3MachineStatus{
					Ready: true,
				},
			},
			ExpectTrue: true,
		}),
		Entry("missing ready", testCaseProvisioned{
			BMMachine: capm3.Metal3Machine{
				Spec: capm3.Metal3MachineSpec{
					ProviderID: pointer.StringPtr("abc"),
				},
			},
			ExpectTrue: false,
		}),
		Entry("missing providerID", testCaseProvisioned{
			BMMachine: capm3.Metal3Machine{
				Status: capm3.Metal3MachineStatus{
					Ready: true,
				},
			},
			ExpectTrue: false,
		}),
		Entry("missing ProviderID and ready", testCaseProvisioned{
			BMMachine:  capm3.Metal3Machine{},
			ExpectTrue: false,
		}),
	)

	type testCaseBootstrapReady struct {
		Machine    capi.Machine
		ExpectTrue bool
	}

	DescribeTable("Test BootstrapReady",
		func(tc testCaseBootstrapReady) {
			machineMgr, err := NewMachineManager(nil, nil, nil, &tc.Machine, nil,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			bootstrapState := machineMgr.IsBootstrapReady()

			Expect(bootstrapState).To(Equal(tc.ExpectTrue))
		},
		Entry("ready", testCaseBootstrapReady{
			Machine: capi.Machine{
				Status: capi.MachineStatus{
					BootstrapReady: true,
				},
			},
			ExpectTrue: true,
		}),
		Entry("not ready", testCaseBootstrapReady{
			Machine:    capi.Machine{},
			ExpectTrue: false,
		}),
	)

	DescribeTable("Test setting and clearing errors",
		func(bmMachine capm3.Metal3Machine) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &bmMachine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			machineMgr.setError("abc", capierrors.InvalidConfigurationMachineError)

			Expect(*bmMachine.Status.FailureReason).To(Equal(
				capierrors.InvalidConfigurationMachineError,
			))
			Expect(*bmMachine.Status.FailureMessage).To(Equal("abc"))

			machineMgr.clearError()

			Expect(bmMachine.Status.FailureReason).To(BeNil())
			Expect(bmMachine.Status.FailureMessage).To(BeNil())
		},
		Entry("No errors", capm3.Metal3Machine{}),
		Entry("Overwrite existing error message", capm3.Metal3Machine{
			Status: capm3.Metal3MachineStatus{
				FailureMessage: pointer.StringPtr("cba"),
			},
		}),
	)

	Describe("Test ChooseHost", func() {

		//Creating the hosts
		host1 := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "host1",
				Namespace: "myns",
			},
			Spec: bmh.BareMetalHostSpec{
				ConsumerRef: &corev1.ObjectReference{
					Name:       "someothermachine",
					Namespace:  "myns",
					Kind:       "BMMachine",
					APIVersion: capm3.GroupVersion.String(),
				},
			},
		}
		host2 := *newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false)

		host3 := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "host3",
				Namespace: "myns",
			},
			Spec: bmh.BareMetalHostSpec{
				ConsumerRef: &corev1.ObjectReference{
					Name:       "machine1",
					Namespace:  "myns",
					Kind:       "BMMachine",
					APIVersion: capm3.GroupVersion.String(),
				},
			},
		}
		host4 := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "host4",
				Namespace: "someotherns",
			},
		}
		discoveredHost := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "discoveredHost",
				Namespace: "myns",
			},
			Status: bmh.BareMetalHostStatus{
				ErrorMessage: "this host is discovered but not usable",
			},
		}
		hostWithLabel := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostWithLabel",
				Namespace: "myns",
				Labels:    map[string]string{"key1": "value1"},
			},
		}

		bmmconfig, infrastructureRef := newConfig("", map[string]string{},
			[]capm3.HostSelectorRequirement{},
		)
		bmmconfig2, infrastructureRef2 := newConfig("",
			map[string]string{"key1": "value1"}, []capm3.HostSelectorRequirement{},
		)
		bmmconfig3, infrastructureRef3 := newConfig("",
			map[string]string{"boguskey": "value"}, []capm3.HostSelectorRequirement{},
		)
		bmmconfig4, infrastructureRef4 := newConfig("", map[string]string{},
			[]capm3.HostSelectorRequirement{
				capm3.HostSelectorRequirement{
					Key:      "key1",
					Operator: "in",
					Values:   []string{"abc", "value1", "123"},
				},
			},
		)
		bmmconfig5, infrastructureRef5 := newConfig("", map[string]string{},
			[]capm3.HostSelectorRequirement{
				capm3.HostSelectorRequirement{
					Key:      "key1",
					Operator: "pancakes",
					Values:   []string{"abc", "value1", "123"},
				},
			},
		)

		type testCaseChooseHost struct {
			Machine          *capi.Machine
			Hosts            []runtime.Object
			BMMachine        *capm3.Metal3Machine
			ExpectedHostName string
		}

		DescribeTable("Test ChooseHost",
			func(tc testCaseChooseHost) {
				c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.Hosts...)
				machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
					tc.BMMachine, klogr.New(),
				)
				Expect(err).NotTo(HaveOccurred())

				result, err := machineMgr.chooseHost(context.TODO())

				if tc.ExpectedHostName == "" {
					Expect(result).To(BeNil())
					return
				}
				Expect(err).NotTo(HaveOccurred())
				if result != nil {
					Expect(result.Name).To(Equal(tc.ExpectedHostName))
				}
			},
			Entry("Pick host2 which lacks ConsumerRef", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef2),
				Hosts:            []runtime.Object{&host2, &host1},
				BMMachine:        bmmconfig2,
				ExpectedHostName: host2.Name,
			}),
			Entry("Ignore discoveredHost and pick host2, which lacks a ConsumerRef",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef2),
					Hosts:            []runtime.Object{&discoveredHost, &host2, &host1},
					BMMachine:        bmmconfig2,
					ExpectedHostName: host2.Name,
				},
			),
			Entry("Pick host3, which has a matching ConsumerRef", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef3),
				Hosts:            []runtime.Object{&host1, &host3, &host2},
				BMMachine:        bmmconfig3,
				ExpectedHostName: host3.Name,
			}),
			Entry("Two hosts already taken, third is in another namespace",
				testCaseChooseHost{
					Machine:          newMachine("machine2", "", infrastructureRef),
					Hosts:            []runtime.Object{&host1, &host3, &host4},
					BMMachine:        bmmconfig,
					ExpectedHostName: "",
				},
			),
			Entry("Choose hosts with a label, even without a label selector",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef),
					Hosts:            []runtime.Object{&hostWithLabel},
					BMMachine:        bmmconfig,
					ExpectedHostName: hostWithLabel.Name,
				},
			),
			Entry("Choose the host with the right label", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef2),
				Hosts:            []runtime.Object{&hostWithLabel, &host2},
				BMMachine:        bmmconfig2,
				ExpectedHostName: hostWithLabel.Name,
			}),
			Entry("No host that matches required label", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef3),
				Hosts:            []runtime.Object{&host2, &hostWithLabel},
				BMMachine:        bmmconfig3,
				ExpectedHostName: "",
			}),
			Entry("Host that matches a matchExpression", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef4),
				Hosts:            []runtime.Object{&host2, &hostWithLabel},
				BMMachine:        bmmconfig4,
				ExpectedHostName: hostWithLabel.Name,
			}),
			Entry("No Host available that matches a matchExpression",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef4),
					Hosts:            []runtime.Object{&host2},
					BMMachine:        bmmconfig4,
					ExpectedHostName: "",
				},
			),
			Entry("No host chosen, invalid match expression", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef5),
				Hosts:            []runtime.Object{&host2, &hostWithLabel, &host1},
				BMMachine:        bmmconfig5,
				ExpectedHostName: "",
			}),
		)
	})

	type testCaseSetHostSpec struct {
		UserDataNamespace         string
		ExpectedUserDataNamespace string
		Host                      *bmh.BareMetalHost
		ExpectedImage             *bmh.Image
		ExpectUserData            bool
	}

	DescribeTable("Test SetHostSpec",
		func(tc testCaseSetHostSpec) {
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.Host)

			bmmconfig, infrastructureRef := newConfig(tc.UserDataNamespace,
				map[string]string{}, []capm3.HostSelectorRequirement{},
			)
			machine := newMachine("machine1", "", infrastructureRef)

			machineMgr, err := NewMachineManager(c, nil, nil, machine, bmmconfig,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.setHostSpec(context.TODO(), tc.Host)
			Expect(err).NotTo(HaveOccurred())
			Expect(err).NotTo(HaveOccurred())

			// validate the saved host
			Expect(tc.Host.Spec.ConsumerRef).NotTo(BeNil())
			Expect(tc.Host.Spec.ConsumerRef.Name).To(Equal(bmmconfig.Name))
			Expect(tc.Host.Spec.ConsumerRef.Namespace).
				To(Equal(bmmconfig.Namespace))
			Expect(tc.Host.Spec.ConsumerRef.Kind).To(Equal("Metal3Machine"))
			Expect(tc.Host.Spec.Online).To(BeTrue())
			if tc.ExpectedImage == nil {
				Expect(tc.Host.Spec.Image).To(BeNil())
			} else {
				Expect(*tc.Host.Spec.Image).To(Equal(*tc.ExpectedImage))
			}
			if tc.ExpectUserData {
				Expect(tc.Host.Spec.UserData).NotTo(BeNil())
				Expect(tc.Host.Spec.UserData.Namespace).
					To(Equal(tc.ExpectedUserDataNamespace))
				Expect(tc.Host.Spec.UserData.Name).To(Equal(testUserDataSecretName))
			} else {
				Expect(tc.Host.Spec.UserData).To(BeNil())
			}
			_, err = machineMgr.FindOwnerRef(tc.Host.OwnerReferences)
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("User data has explicit alternate namespace", testCaseSetHostSpec{
			UserDataNamespace:         "otherns",
			ExpectedUserDataNamespace: "otherns",
			Host: newBareMetalHost("host2", nil, bmh.StateNone,
				nil, false, false,
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("User data has no namespace", testCaseSetHostSpec{
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: "myns",
			Host: newBareMetalHost("host2", nil, bmh.StateNone,
				nil, false, false,
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("Externally provisioned, same machine", testCaseSetHostSpec{
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: "myns",
			Host: newBareMetalHost("host2", nil, bmh.StateNone,
				nil, false, false,
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("Previously provisioned, different image",
			testCaseSetHostSpec{
				UserDataNamespace:         "",
				ExpectedUserDataNamespace: "myns",
				Host: newBareMetalHost("host2", bmhSpecTestImg(),
					bmh.StateNone, nil, false, false,
				),
				ExpectedImage:  expectedImgTest(),
				ExpectUserData: false,
			},
		),
	)

	Describe("Test Exists function", func() {
		host := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "somehost",
				Namespace: "myns",
			},
		}
		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), &host)

		type testCaseExists struct {
			Machine   *capi.Machine
			BMMachine *capm3.Metal3Machine
			Expected  bool
		}

		DescribeTable("Test Exists function",
			func(tc testCaseExists) {
				machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
					tc.BMMachine, klogr.New(),
				)
				Expect(err).NotTo(HaveOccurred())

				result, err := machineMgr.exists(context.TODO())
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(tc.Expected))
			},
			Entry("Failed to find the existing host", testCaseExists{
				Machine: &capi.Machine{},
				BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil,
					bmmObjectMetaWithSomeAnnotations(),
				),
				Expected: true,
			}),
			Entry("Found host even though annotation value is incorrect",
				testCaseExists{
					Machine: &capi.Machine{},
					BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil,
						bmmObjectMetaWithInvalidAnnotations(),
					),
					Expected: false,
				},
			),
			Entry("Found host even though annotation not present", testCaseExists{
				Machine: &capi.Machine{},
				BMMachine: newMetal3Machine("", nil, nil, nil,
					bmmObjectMetaEmptyAnnotations(),
				),
				Expected: false,
			}),
		)
	})

	Describe("Test GetHost", func() {
		host := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myhost",
				Namespace: "myns",
			},
		}
		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), &host)

		type testCaseGetHost struct {
			Machine       *capi.Machine
			BMMachine     *capm3.Metal3Machine
			ExpectPresent bool
		}

		DescribeTable("Test GetHost",
			func(tc testCaseGetHost) {
				machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
					tc.BMMachine, klogr.New(),
				)
				Expect(err).NotTo(HaveOccurred())

				result, err := machineMgr.getHost(context.TODO())
				Expect(err).NotTo(HaveOccurred())
				if tc.ExpectPresent {
					Expect(result).NotTo(BeNil())
				} else {
					Expect(result).To(BeNil())
				}
			},
			Entry("Should find the expected host", testCaseGetHost{
				Machine: &capi.Machine{},
				BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil,
					bmmObjectMetaWithValidAnnotations(),
				),
				ExpectPresent: true,
			}),
			Entry("Should not find the host, annotation value incorrect",
				testCaseGetHost{
					Machine: &capi.Machine{},
					BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil,
						bmmObjectMetaWithInvalidAnnotations(),
					),
					ExpectPresent: false,
				},
			),
			Entry("Should not find the host, annotation not present", testCaseGetHost{
				Machine: &capi.Machine{},
				BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil,
					bmmObjectMetaEmptyAnnotations(),
				),
				ExpectPresent: false,
			}),
		)
	})

	type testCaseGetSetProviderID struct {
		Machine       *capi.Machine
		BMMachine     *capm3.Metal3Machine
		Host          *bmh.BareMetalHost
		ExpectPresent bool
		ExpectError   bool
	}

	DescribeTable("Test Get and Set Provider ID",
		func(tc testCaseGetSetProviderID) {
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.Host)
			machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
				tc.BMMachine, klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			bmhID, err := machineMgr.GetBaremetalHostID(context.TODO())
			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			if tc.ExpectPresent {
				Expect(bmhID).NotTo(BeNil())
			} else {
				Expect(bmhID).To(BeNil())
				return
			}

			providerID := fmt.Sprintf("metal3://%s", *bmhID)
			Expect(*tc.BMMachine.Spec.ProviderID).To(Equal(providerID))
		},
		Entry("Set ProviderID, empty annotations", testCaseGetSetProviderID{
			Machine: newMachine("", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSpec(), nil,
				bmmObjectMetaEmptyAnnotations(),
			),
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
				},
			},
			ExpectPresent: false,
			ExpectError:   true,
		}),
		Entry("Set ProviderID", testCaseGetSetProviderID{
			Machine: newMachine("", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSpec(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
					UID:       "12345ID6789",
				},
				Status: bmh.BareMetalHostStatus{
					Provisioning: bmh.ProvisionStatus{
						State: bmh.StateProvisioned,
					},
				},
			},
			ExpectPresent: true,
			ExpectError:   false,
		}),
		Entry("Set ProviderID, wrong state", testCaseGetSetProviderID{
			Machine: newMachine("", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSpec(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: "myns",
					UID:       "12345ID6789",
				},
				Status: bmh.BareMetalHostStatus{
					Provisioning: bmh.ProvisionStatus{
						State: bmh.StateProvisioning,
					},
				},
			},
			ExpectPresent: false,
			ExpectError:   true,
		}),
	)

	Describe("Test utility functions", func() {
		type testCaseSmallFunctions struct {
			Machine        *capi.Machine
			BMMachine      *capm3.Metal3Machine
			ExpectCtrlNode bool
		}

		host := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myhost",
				Namespace: "myns",
			},
		}
		c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), &host)

		DescribeTable("Test small functions",
			func(tc testCaseSmallFunctions) {
				machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
					tc.BMMachine, klogr.New(),
				)
				Expect(err).NotTo(HaveOccurred())

				role := machineMgr.role()
				if tc.ExpectCtrlNode {
					Expect(role).To(Equal("control-plane"))
				} else {
					Expect(role).To(Equal("node"))
				}

				isCtrlPlane := machineMgr.isControlPlane()
				if tc.ExpectCtrlNode {
					Expect(isCtrlPlane).To(BeTrue())
				} else {
					Expect(isCtrlPlane).To(BeFalse())
				}
			},
			Entry("Test small functions, worker node", testCaseSmallFunctions{
				Machine: newMachine("", "", nil),
				BMMachine: newMetal3Machine("mybmmachine", nil, bmmSpec(), nil,
					bmmObjectMetaEmptyAnnotations(),
				),
				ExpectCtrlNode: false,
			}),
			Entry("Test small functions, control plane node", testCaseSmallFunctions{
				Machine: &capi.Machine{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Machine",
						APIVersion: capi.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mymachine",
						Namespace: "myns",
						Labels: map[string]string{
							capi.MachineControlPlaneLabelName: "labelHere",
						},
					},
				},
				BMMachine: newMetal3Machine("mybmmachine", nil, bmmSpec(), nil,
					bmmObjectMetaEmptyAnnotations(),
				),
				ExpectCtrlNode: true,
			}),
		)
	})

	type testCaseEnsureAnnotation struct {
		Machine          capi.Machine
		Host             *bmh.BareMetalHost
		BMMachine        *capm3.Metal3Machine
		ExpectAnnotation bool
	}

	DescribeTable("Test EnsureAnnotation",
		func(tc testCaseEnsureAnnotation) {
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.BMMachine)

			machineMgr, err := NewMachineManager(c, nil, nil, &tc.Machine,
				tc.BMMachine, klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.ensureAnnotation(context.TODO(), tc.Host)
			Expect(err).NotTo(HaveOccurred())

			annotations := tc.BMMachine.ObjectMeta.GetAnnotations()
			if !tc.ExpectAnnotation {
				Expect(annotations).To(BeNil())
			} else {
				Expect(annotations).NotTo(BeNil())
				Expect(annotations[HostAnnotation]).
					To(Equal(tc.BMMachine.Annotations[HostAnnotation]))
			}

			ok := machineMgr.HasAnnotation()
			if tc.ExpectAnnotation {
				Expect(ok).To(BeTrue())
			} else {
				Expect(ok).To(BeFalse())
			}
		},
		Entry("Annotation exists and is correct", testCaseEnsureAnnotation{
			Machine: capi.Machine{},
			BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone, nil,
				false, false,
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotation exists but is wrong", testCaseEnsureAnnotation{
			Machine: capi.Machine{},
			BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil,
				bmmObjectMetaWithInvalidAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone,
				nil, false, false,
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotations are empty", testCaseEnsureAnnotation{
			Machine: capi.Machine{},
			BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil,
				bmmObjectMetaEmptyAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone,
				nil, false, false,
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotations are nil", testCaseEnsureAnnotation{
			Machine: capi.Machine{},
			BMMachine: newMetal3Machine("", nil, nil, nil,
				bmmObjectMetaNoAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone,
				nil, false, false,
			),
			ExpectAnnotation: true,
		}),
	)

	type testCaseDelete struct {
		Host                      *bmh.BareMetalHost
		Secret                    *corev1.Secret
		Machine                   *capi.Machine
		BMMachine                 *capm3.Metal3Machine
		BMCSecret                 *corev1.Secret
		ExpectedConsumerRef       *corev1.ObjectReference
		ExpectedResult            error
		ExpectSecretDeleted       bool
		ExpectClusterLabelDeleted bool
	}

	DescribeTable("Test Delete function",
		func(tc testCaseDelete) {
			objects := []runtime.Object{tc.BMMachine}
			if tc.Host != nil {
				objects = append(objects, tc.Host)
			}
			if tc.Secret != nil {
				objects = append(objects, tc.Secret)
			}
			if tc.BMCSecret != nil {
				objects = append(objects, tc.BMCSecret)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)

			machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
				tc.BMMachine, klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.Delete(context.TODO())

			if tc.ExpectedResult == nil {
				Expect(err).NotTo(HaveOccurred())
			} else {
				perr, ok := err.(*RequeueAfterError)
				Expect(ok).To(BeTrue())
				Expect(perr.Error()).To(Equal(tc.ExpectedResult.Error()))
			}

			if tc.Host != nil {
				key := client.ObjectKey{
					Name:      tc.Host.Name,
					Namespace: tc.Host.Namespace,
				}
				host := bmh.BareMetalHost{}

				if tc.Host != nil {
					err := c.Get(context.TODO(), key, &host)
					Expect(err).NotTo(HaveOccurred())
				}

				name := ""
				expectedName := ""
				if host.Spec.ConsumerRef != nil {
					name = host.Spec.ConsumerRef.Name
				}
				if tc.ExpectedConsumerRef != nil {
					expectedName = tc.ExpectedConsumerRef.Name
				}
				Expect(name).To(Equal(expectedName))
			}

			tmpBootstrapSecret := corev1.Secret{}
			key := client.ObjectKey{
				Name:      tc.BMMachine.Spec.UserData.Name,
				Namespace: tc.BMMachine.Spec.UserData.Namespace,
			}
			err = c.Get(context.TODO(), key, &tmpBootstrapSecret)
			if tc.ExpectSecretDeleted {
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			if tc.ExpectClusterLabelDeleted {
				// get the saved host
				savedHost := bmh.BareMetalHost{}
				err = c.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.Host.Name,
						Namespace: tc.Host.Namespace,
					},
					&savedHost,
				)
				Expect(err).NotTo(HaveOccurred())
				// get the BMC credential
				savedCred := corev1.Secret{}
				err = c.Get(context.TODO(),
					client.ObjectKey{
						Name:      savedHost.Spec.BMC.CredentialsName,
						Namespace: savedHost.Namespace,
					},
					&savedCred,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedHost.Labels[capi.ClusterLabelName]).To(Equal(""))
				Expect(savedCred.Labels[capi.ClusterLabelName]).To(Equal(""))
				// Other labels are not removed
				Expect(savedHost.Labels["foo"]).To(Equal("bar"))
				Expect(savedCred.Labels["foo"]).To(Equal("bar"))
			}
		},
		Entry("Deprovisioning needed", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpec(),
				bmh.StateProvisioned, bmhStatus(), false, false,
			),
			Machine: newMachine("mymachine", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef: consumerRef(),
			ExpectedResult:      &RequeueAfterError{},
			Secret:              newSecret(),
		}),
		Entry("No Host status, deprovisioning needed", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpec(), bmh.StateNone,
				nil, false, false,
			),
			Machine: newMachine("mymachine", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef: consumerRef(),
			ExpectedResult:      &RequeueAfterError{},
			Secret:              newSecret(),
		}),
		Entry("No Host status, no deprovisioning needed", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(), bmh.StateNone, nil,
				false, false,
			),
			Machine: newMachine("mymachine", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: true,
		}),
		Entry("Deprovisioning in progress", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(),
				bmh.StateDeprovisioning, bmhStatus(), false, false,
			),
			Machine: newMachine("mymachine", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef: consumerRef(),
			ExpectedResult:      &RequeueAfterError{RequeueAfter: time.Second * 30},
			Secret:              newSecret(),
		}),
		Entry("Externally provisioned host should be powered down", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(),
				bmh.StateExternallyProvisioned, bmhPowerStatus(), true, false,
			),
			Machine: newMachine("mymachine", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef: consumerRef(),
			ExpectedResult:      &RequeueAfterError{RequeueAfter: time.Second * 30},
			Secret:              newSecret(),
		}),
		Entry("Consumer ref should be removed from externally provisioned host",
			testCaseDelete{
				Host: newBareMetalHost("myhost", bmhSpecNoImg(),
					bmh.StateExternallyProvisioned, bmhPowerStatus(), false, false,
				),
				Machine: newMachine("mymachine", "", nil),
				BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
					bmmObjectMetaWithValidAnnotations(),
				),
				Secret:              newSecret(),
				ExpectSecretDeleted: true,
			},
		),
		Entry("Consumer ref should be removed", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(), bmh.StateReady,
				bmhStatus(), false, false,
			),
			Machine: newMachine("mymachine", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: true,
		}),
		Entry("Consumer ref should be removed, secret not deleted", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(), bmh.StateReady,
				bmhStatus(), false, false,
			),
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns2",
				},
				Spec: capi.MachineSpec{
					Bootstrap: capi.Bootstrap{
						DataSecretName: pointer.StringPtr("Foobar"),
					},
				},
			},
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			Secret: newSecret(),
		}),
		Entry("Consumer ref does not match, so it should not be removed",
			testCaseDelete{
				Host: newBareMetalHost("myhost", bmhSpecSomeImg(),
					bmh.StateProvisioned, bmhStatus(), false, false,
				),
				Machine: newMachine("", "", nil),
				BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
					bmmObjectMetaWithValidAnnotations(),
				),
				ExpectedConsumerRef: consumerRefSome(),
				Secret:              newSecret(),
			},
		),
		Entry("No consumer ref, so this is a no-op", testCaseDelete{
			Host:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
			Machine: newMachine("", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: false,
		}),
		Entry("No host at all, so this is a no-op", testCaseDelete{
			Host:    nil,
			Machine: newMachine("", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: false,
		}),
		Entry("dataSecretName set, deleting secret", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(), bmh.StateNone, nil,
				false, false,
			),
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns",
				},
				Spec: capi.MachineSpec{
					Bootstrap: capi.Bootstrap{
						DataSecretName: pointer.StringPtr("mybmmachine-user-data"),
					},
				},
			},
			BMMachine: newMetal3Machine("mybmmachine", nil, bmmSecret(), nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: false,
		}),
		Entry("Clusterlabel should be removed", testCaseDelete{
			Machine:                   newMachine("mymachine", "mybmmachine", nil),
			BMMachine:                 newMetal3Machine("mybmmachine", nil, bmmSpecAll(), nil, bmmObjectMetaWithValidAnnotations()),
			Host:                      newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, true),
			BMCSecret:                 newBMCSecret("mycredentials", true),
			ExpectSecretDeleted:       true,
			ExpectClusterLabelDeleted: true,
		}),
		Entry("No clusterLabel in BMH or BMC Secret so this is a no-op ", testCaseDelete{
			Machine:                   newMachine("mymachine", "mybmmachine", nil),
			BMMachine:                 newMetal3Machine("mybmmachine", nil, bmmSpecAll(), nil, bmmObjectMetaWithValidAnnotations()),
			Host:                      newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, false),
			BMCSecret:                 newBMCSecret("mycredentials", false),
			ExpectSecretDeleted:       true,
			ExpectClusterLabelDeleted: false,
		}),
	)

	Describe("Test UpdateMachineStatus", func() {
		nic1 := bmh.NIC{
			IP: "192.168.1.1",
		}

		nic2 := bmh.NIC{
			IP: "172.0.20.2",
		}

		type testCaseUpdateMachineStatus struct {
			Host            *bmh.BareMetalHost
			Machine         *capi.Machine
			ExpectedMachine capi.Machine
			BMMachine       capm3.Metal3Machine
		}

		DescribeTable("Test UpdateMachineStatus",
			func(tc testCaseUpdateMachineStatus) {
				c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), &tc.BMMachine)

				machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
					&tc.BMMachine, klogr.New(),
				)
				Expect(err).NotTo(HaveOccurred())

				err = machineMgr.updateMachineStatus(context.TODO(), tc.Host)
				Expect(err).NotTo(HaveOccurred())

				key := client.ObjectKey{
					Name:      tc.BMMachine.ObjectMeta.Name,
					Namespace: tc.BMMachine.ObjectMeta.Namespace,
				}
				bmmachine := capm3.Metal3Machine{}
				err = c.Get(context.TODO(), key, &bmmachine)
				Expect(err).NotTo(HaveOccurred())

				if tc.BMMachine.Status.Addresses != nil {
					for i, address := range tc.ExpectedMachine.Status.Addresses {
						Expect(bmmachine.Status.Addresses[i]).To(Equal(address))
					}
				}
			},
			Entry("Machine status updated", testCaseUpdateMachineStatus{
				Host: &bmh.BareMetalHost{
					Status: bmh.BareMetalHostStatus{
						HardwareDetails: &bmh.HardwareDetails{
							NIC: []bmh.NIC{nic1, nic2},
						},
					},
				},
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mymachine",
						Namespace: "myns",
					},
					Status: capi.MachineStatus{
						Addresses: []capi.MachineAddress{
							capi.MachineAddress{
								Address: "192.168.1.255",
								Type:    "InternalIP",
							},
							capi.MachineAddress{
								Address: "172.0.20.255",
								Type:    "InternalIP",
							},
						},
					},
				},
				BMMachine: capm3.Metal3Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mybmmachine",
						Namespace: "myns",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "BMMachine",
						APIVersion: capi.GroupVersion.String(),
					},
					Status: capm3.Metal3MachineStatus{
						Addresses: []capi.MachineAddress{
							capi.MachineAddress{
								Address: "192.168.1.1",
								Type:    "InternalIP",
							},
							capi.MachineAddress{
								Address: "172.0.20.2",
								Type:    "InternalIP",
							},
						},
						Ready: true,
					},
				},
				ExpectedMachine: capi.Machine{
					Status: capi.MachineStatus{
						Addresses: []capi.MachineAddress{
							capi.MachineAddress{
								Address: "192.168.1.1",
								Type:    "InternalIP",
							},
							capi.MachineAddress{
								Address: "172.0.20.2",
								Type:    "InternalIP",
							},
						},
					},
				},
			}),
			Entry("Machine status unchanged", testCaseUpdateMachineStatus{
				Host: &bmh.BareMetalHost{
					Status: bmh.BareMetalHostStatus{
						HardwareDetails: &bmh.HardwareDetails{
							NIC: []bmh.NIC{nic1, nic2},
						},
					},
				},
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mymachine",
						Namespace: "myns",
					},
					Status: capi.MachineStatus{
						Addresses: []capi.MachineAddress{
							capi.MachineAddress{
								Address: "192.168.1.1",
								Type:    "InternalIP",
							},
							capi.MachineAddress{
								Address: "172.0.20.2",
								Type:    "InternalIP",
							},
						},
					},
				},
				BMMachine: capm3.Metal3Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mybmmachine",
						Namespace: "myns",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "BMMachine",
						APIVersion: capi.GroupVersion.String(),
					},
					Status: capm3.Metal3MachineStatus{
						Addresses: []capi.MachineAddress{
							capi.MachineAddress{
								Address: "192.168.1.1",
								Type:    "InternalIP",
							},
							capi.MachineAddress{
								Address: "172.0.20.2",
								Type:    "InternalIP",
							},
						},
						Ready: true,
					},
				},
				ExpectedMachine: capi.Machine{
					Status: capi.MachineStatus{
						Addresses: []capi.MachineAddress{
							capi.MachineAddress{
								Address: "192.168.1.1",
								Type:    "InternalIP",
							},
							capi.MachineAddress{
								Address: "172.0.20.2",
								Type:    "InternalIP",
							},
						},
					},
				},
			}),
			Entry("Machine status unchanged, status set empty",
				testCaseUpdateMachineStatus{
					Host: &bmh.BareMetalHost{},
					Machine: &capi.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "mymachine",
							Namespace: "myns",
						},
						Status: capi.MachineStatus{},
					},
					BMMachine: capm3.Metal3Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "mybmmachine",
							Namespace: "myns",
						},
						TypeMeta: metav1.TypeMeta{
							Kind:       "BMMachine",
							APIVersion: capi.GroupVersion.String(),
						},
						Status: capm3.Metal3MachineStatus{
							Addresses: []capi.MachineAddress{},
							Ready:     true,
						},
					},
					ExpectedMachine: capi.Machine{
						Status: capi.MachineStatus{},
					},
				},
			),
		)
	})

	Describe("Test NodeAddresses", func() {
		nic1 := bmh.NIC{
			IP: "192.168.1.1",
		}

		nic2 := bmh.NIC{
			IP: "172.0.20.2",
		}

		addr1 := capi.MachineAddress{
			Type:    capi.MachineInternalIP,
			Address: "192.168.1.1",
		}

		addr2 := capi.MachineAddress{
			Type:    capi.MachineInternalIP,
			Address: "172.0.20.2",
		}

		addr3 := capi.MachineAddress{
			Type:    capi.MachineHostName,
			Address: "mygreathost",
		}

		type testCaseNodeAddress struct {
			Machine               capi.Machine
			BMMachine             capm3.Metal3Machine
			Host                  *bmh.BareMetalHost
			ExpectedNodeAddresses []capi.MachineAddress
		}

		DescribeTable("Test NodeAddress",
			func(tc testCaseNodeAddress) {
				var nodeAddresses []capi.MachineAddress

				c := fakeclient.NewFakeClientWithScheme(setupSchemeMm())
				machineMgr, err := NewMachineManager(c, nil, nil, &tc.Machine,
					&tc.BMMachine, klogr.New(),
				)
				Expect(err).NotTo(HaveOccurred())

				if tc.Host != nil {
					nodeAddresses = machineMgr.nodeAddresses(tc.Host)
					Expect(err).NotTo(HaveOccurred())
				}
				for i, address := range tc.ExpectedNodeAddresses {
					Expect(nodeAddresses[i]).To(Equal(address))
				}
			},
			Entry("One NIC", testCaseNodeAddress{
				Host: &bmh.BareMetalHost{
					Status: bmh.BareMetalHostStatus{
						HardwareDetails: &bmh.HardwareDetails{
							NIC: []bmh.NIC{nic1},
						},
					},
				},
				ExpectedNodeAddresses: []capi.MachineAddress{addr1},
			}),
			Entry("Two NICs", testCaseNodeAddress{
				Host: &bmh.BareMetalHost{
					Status: bmh.BareMetalHostStatus{
						HardwareDetails: &bmh.HardwareDetails{
							NIC: []bmh.NIC{nic1, nic2},
						},
					},
				},
				ExpectedNodeAddresses: []capi.MachineAddress{addr1, addr2},
			}),
			Entry("Hostname is set", testCaseNodeAddress{
				Host: &bmh.BareMetalHost{
					Status: bmh.BareMetalHostStatus{
						HardwareDetails: &bmh.HardwareDetails{
							Hostname: "mygreathost",
						},
					},
				},
				ExpectedNodeAddresses: []capi.MachineAddress{addr3},
			}),
			Entry("Empty Hostname", testCaseNodeAddress{
				Host: &bmh.BareMetalHost{
					Status: bmh.BareMetalHostStatus{
						HardwareDetails: &bmh.HardwareDetails{
							Hostname: "",
						},
					},
				},
				ExpectedNodeAddresses: []capi.MachineAddress{},
			}),
			Entry("No host at all, so this is a no-op", testCaseNodeAddress{
				Host:                  nil,
				ExpectedNodeAddresses: nil,
			}),
		)
	})

	Describe("Test SetNodeProviderID", func() {
		scheme := runtime.NewScheme()
		err := capi.AddToScheme(scheme)
		if err != nil {
			log.Printf("AddToScheme failed: %v", err)
		}
		err = bmoapis.AddToScheme(scheme)
		if err != nil {
			log.Printf("AddToScheme failed: %v", err)
		}

		type testCaseSetNodePoviderID struct {
			Node               v1.Node
			HostID             string
			ExpectedError      bool
			ExpectedProviderID string
		}

		DescribeTable("Test SetNodeProviderID",
			func(tc testCaseSetNodePoviderID) {
				c := fakeclient.NewFakeClientWithScheme(scheme)
				corev1Client := clientfake.NewSimpleClientset(&tc.Node).CoreV1()
				mockCapiClientGetter := func(ctx context.Context, c client.Client, cluster *capi.Cluster) (
					clientcorev1.CoreV1Interface, error,
				) {
					return corev1Client, nil
				}

				machineMgr, err := NewMachineManager(c, newCluster(clusterName),
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef,
						&capm3.Metal3ClusterSpec{NoCloudProvider: true}, nil,
					),
					&capi.Machine{}, &capm3.Metal3Machine{}, klogr.New(),
				)
				Expect(err).NotTo(HaveOccurred())

				err = machineMgr.SetNodeProviderID(context.TODO(), tc.HostID,
					tc.ExpectedProviderID, mockCapiClientGetter,
				)

				if tc.ExpectedError {
					Expect(err).To(HaveOccurred())
					return
				} else {
					Expect(err).NotTo(HaveOccurred())
				}

				// get the node
				node, err := corev1Client.Nodes().Get(tc.Node.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				Expect(node.Spec.ProviderID).To(Equal(tc.ExpectedProviderID))
			},
			Entry("Set target ProviderID, No matching node", testCaseSetNodePoviderID{
				Node:               v1.Node{},
				HostID:             "abcd",
				ExpectedError:      true,
				ExpectedProviderID: "metal3://abcd",
			}),
			Entry("Set target ProviderID, matching node", testCaseSetNodePoviderID{
				Node: v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"metal3.io/uuid": "abcd",
						},
					},
				},
				HostID:             "abcd",
				ExpectedError:      false,
				ExpectedProviderID: "metal3://abcd",
			}),
			Entry("Set target ProviderID, providerID set", testCaseSetNodePoviderID{
				Node: v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"metal3.io/uuid": "abcd",
						},
					},
					Spec: v1.NodeSpec{
						ProviderID: "metal3://abcd",
					},
				},
				HostID:             "abcd",
				ExpectedError:      false,
				ExpectedProviderID: "metal3://abcd",
			}),
		)
	})

	type testCaseGetUserData struct {
		Machine     *capi.Machine
		BMMachine   *capm3.Metal3Machine
		BMHost      *bmh.BareMetalHost
		Secret      *corev1.Secret
		ExpectError bool
	}

	DescribeTable("Test GetUserData function",
		func(tc testCaseGetUserData) {
			objects := []runtime.Object{
				tc.BMMachine,
				tc.Machine,
			}
			if tc.Secret != nil {
				objects = append(objects, tc.Secret)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)

			machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
				tc.BMMachine, klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.GetUserData(context.TODO(), tc.BMHost)
			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
				return
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			// Expect the reference to the secret to be passed through
			if tc.Machine.Spec.Bootstrap.DataSecretName != nil &&
				tc.Machine.Namespace == tc.BMHost.Namespace {
				Expect(tc.BMMachine.Spec.UserData.Name).To(Equal(
					*tc.Machine.Spec.Bootstrap.DataSecretName,
				))
				Expect(tc.BMMachine.Spec.UserData.Namespace).To(Equal(
					tc.BMHost.Namespace,
				))
			}

			// if we had to create an additional secret (dataSecretName not set and
			// Data set)
			if tc.Machine.Spec.Bootstrap.DataSecretName == nil &&
				tc.Machine.Spec.Bootstrap.Data != nil {

				Expect(tc.BMMachine.Spec.UserData.Name).To(Equal(
					tc.BMMachine.Name + "-user-data",
				))
				Expect(tc.BMMachine.Spec.UserData.Namespace).To(Equal(
					tc.BMMachine.Namespace,
				))
				tmpBootstrapSecret := corev1.Secret{}
				key := client.ObjectKey{
					Name:      tc.BMMachine.Spec.UserData.Name,
					Namespace: tc.BMMachine.Namespace,
				}
				err = c.Get(context.TODO(), key, &tmpBootstrapSecret)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(tmpBootstrapSecret.Data["userData"])).To(Equal("FooBar\n"))
				Expect(len(tmpBootstrapSecret.OwnerReferences)).To(BeEquivalentTo(1))
				Expect(tmpBootstrapSecret.OwnerReferences[0].APIVersion).
					To(Equal(tc.BMMachine.APIVersion))
				Expect(tmpBootstrapSecret.OwnerReferences[0].Kind).
					To(Equal(tc.BMMachine.Kind))
				Expect(tmpBootstrapSecret.OwnerReferences[0].Name).
					To(Equal(tc.BMMachine.Name))
				Expect(tmpBootstrapSecret.OwnerReferences[0].UID).
					To(Equal(tc.BMMachine.UID))
				Expect(*tmpBootstrapSecret.OwnerReferences[0].Controller).
					To(BeTrue())
				Expect(len(tmpBootstrapSecret.Finalizers)).To(Equal(1))
				Expect(tmpBootstrapSecret.Finalizers).To(ContainElement(userDataFinalizer))
			}
		},
		Entry("Secret set in Machine", testCaseGetUserData{
			Secret: &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Foobar",
					Namespace: "myns",
				},
				Data: map[string][]byte{
					"value": []byte("FooBar\n"),
				},
				Type: "Opaque",
			},
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns",
				},
				Spec: capi.MachineSpec{
					Bootstrap: capi.Bootstrap{
						DataSecretName: pointer.StringPtr("Foobar"),
					},
				},
			},
			BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
		}),
		Entry("Secret set in Machine, different namespace", testCaseGetUserData{
			Secret: &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Foobar",
					Namespace: "myns2",
				},
				Data: map[string][]byte{
					"value": []byte("FooBar\n"),
				},
				Type: "Opaque",
			},
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns2",
				},
				Spec: capi.MachineSpec{
					Bootstrap: capi.Bootstrap{
						DataSecretName: pointer.StringPtr("Foobar"),
					},
				},
			},
			BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
		}),
		Entry("Secret in other namespace set in Machine", testCaseGetUserData{
			Secret: &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Foobar",
					Namespace: "def",
				},
				Data: map[string][]byte{
					"value": []byte("FooBar\n"),
				},
				Type: "Opaque",
			},
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns2",
				},
				Spec: capi.MachineSpec{
					Bootstrap: capi.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							Name:      "abc",
							Namespace: "def",
						},
						DataSecretName: pointer.StringPtr("Foobar"),
					},
				},
			},
			BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
		}),
		Entry("Userdata set in Machine, secret exists", testCaseGetUserData{
			Secret: newSecret(),
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns",
				},
				Spec: capi.MachineSpec{
					Bootstrap: capi.Bootstrap{
						Data: pointer.StringPtr("Rm9vQmFyCg=="),
					},
				},
			},
			BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
		}),
		Entry("Userdata set in Machine, no secret", testCaseGetUserData{
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns",
				},
				Spec: capi.MachineSpec{
					Bootstrap: capi.Bootstrap{
						Data: pointer.StringPtr("Rm9vQmFyCg=="),
					},
				},
			},
			BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
		}),
		Entry("Userdata set in Machine, invalid", testCaseGetUserData{
			ExpectError: true,
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns",
				},
				Spec: capi.MachineSpec{
					Bootstrap: capi.Bootstrap{
						Data: pointer.StringPtr("Rm9vQmFyCg="),
					},
				},
			},
			BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
		}),
		Entry("No userData in Machine", testCaseGetUserData{
			Machine:   &capi.Machine{},
			BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
		}),
	)

	type testCaseAssociate struct {
		Machine            *capi.Machine
		Host               *bmh.BareMetalHost
		BMMachine          *capm3.Metal3Machine
		BMCSecret          *corev1.Secret
		ExpectRequeue      bool
		ExpectClusterLabel bool
		ExpectOwnerRef     bool
	}

	DescribeTable("Test Associate function",
		func(tc testCaseAssociate) {
			objects := []runtime.Object{
				tc.BMMachine,
				tc.Machine,
			}
			if tc.Host != nil {
				objects = append(objects, tc.Host)
			}
			if tc.BMCSecret != nil {
				objects = append(objects, tc.BMCSecret)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)

			machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
				tc.BMMachine, klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.Associate(context.TODO())
			if tc.ExpectRequeue {
				_, ok := errors.Cause(err).(HasRequeueAfterError)
				Expect(ok).To(BeTrue())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			if tc.Host == nil {
				return
			}
			// get the saved host
			savedHost := bmh.BareMetalHost{}
			err = c.Get(context.TODO(),
				client.ObjectKey{
					Name:      tc.Host.Name,
					Namespace: tc.Host.Namespace,
				},
				&savedHost,
			)
			Expect(err).NotTo(HaveOccurred())
			_, err = machineMgr.FindOwnerRef(savedHost.OwnerReferences)
			if tc.ExpectOwnerRef {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
			if tc.ExpectClusterLabel {
				// get the BMC credential
				savedCred := corev1.Secret{}
				err = c.Get(context.TODO(),
					client.ObjectKey{
						Name:      savedHost.Spec.BMC.CredentialsName,
						Namespace: savedHost.Namespace,
					},
					&savedCred,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedHost.Labels[capi.ClusterLabelName]).To(Equal(tc.Machine.Spec.ClusterName))
				Expect(savedCred.Labels[capi.ClusterLabelName]).To(Equal(tc.Machine.Spec.ClusterName))
			}
		},
		Entry("Associate empty machine, Metal3 machine spec nil",
			testCaseAssociate{
				Machine: newMachine("", "", nil),
				BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil,
					bmmObjectMetaWithValidAnnotations(),
				),
				Host: newBareMetalHost("myhost", nil, bmh.StateNone, nil,
					false, false,
				),
				ExpectRequeue: false,
			},
		),
		Entry("Associate empty machine, Metal3 machine spec set",
			testCaseAssociate{
				Machine: newMachine("", "", nil),
				BMMachine: newMetal3Machine("mybmmachine", nil, bmmSpecAll(), nil,
					bmmObjectMetaWithValidAnnotations(),
				),
				Host: newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil,
					false, false,
				),
				BMCSecret:      newBMCSecret("mycredentials", false),
				ExpectRequeue:  false,
				ExpectOwnerRef: true,
			},
		),
		Entry("Associate empty machine, host empty, Metal3 machine spec set",
			testCaseAssociate{
				Machine: newMachine("", "", nil),
				BMMachine: newMetal3Machine("mybmmachine", nil, bmmSpecAll(), nil,
					bmmObjectMetaWithValidAnnotations(),
				),
				Host:           newBareMetalHost("", nil, bmh.StateNone, nil, false, false),
				ExpectRequeue:  false,
				ExpectOwnerRef: true,
			},
		),
		Entry("Associate machine, host nil, Metal3 machine spec set, requeue",
			testCaseAssociate{
				Machine: newMachine("myUniqueMachine", "", nil),
				BMMachine: newMetal3Machine("mybmmachine", nil, bmmSpecAll(), nil,
					bmmObjectMetaWithValidAnnotations(),
				),
				Host:          nil,
				ExpectRequeue: true,
			},
		),
		Entry("Associate machine, host set, Metal3 machine spec set, set clusterLabel",
			testCaseAssociate{
				Machine:            newMachine("mymachine", "mybmmachine", nil),
				BMMachine:          newMetal3Machine("mybmmachine", nil, bmmSpecAll(), nil, nil),
				Host:               newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, false),
				BMCSecret:          newBMCSecret("mycredentials", false),
				ExpectClusterLabel: true,
				ExpectRequeue:      false,
				ExpectOwnerRef:     true,
			},
		),
	)

	type testCaseUpdate struct {
		Machine   *capi.Machine
		Host      *bmh.BareMetalHost
		BMMachine *capm3.Metal3Machine
	}

	DescribeTable("Test Update function",
		func(tc testCaseUpdate) {
			objects := []runtime.Object{
				tc.Host,
				tc.BMMachine,
				tc.Machine,
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)

			machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
				tc.BMMachine, klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.Update(context.TODO())
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("Update machine", testCaseUpdate{
			Machine: newMachine("mymachine", "", nil),
			BMMachine: newMetal3Machine("mybmmachine", nil, nil, nil,
				bmmObjectMetaWithValidAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
		}),
	)

	type testCaseFindOwnerRef struct {
		BMMachine     capm3.Metal3Machine
		OwnerRefs     []metav1.OwnerReference
		ExpectError   bool
		ExpectedIndex int
	}

	DescribeTable("Test FindOwnerRef",
		func(tc testCaseFindOwnerRef) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &tc.BMMachine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			index, err := machineMgr.FindOwnerRef(tc.OwnerRefs)
			if tc.ExpectError {
				Expect(err).NotTo(BeNil())
				Expect(err).To(BeAssignableToTypeOf(&NotFoundError{}))
			} else {
				Expect(err).To(BeNil())
				Expect(index).To(BeEquivalentTo(tc.ExpectedIndex))
			}
		},
		Entry("Empty list", testCaseFindOwnerRef{
			BMMachine:   *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs:   []metav1.OwnerReference{},
			ExpectError: true,
		}),
		Entry("Absent", testCaseFindOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
			ExpectError: true,
		}),
		Entry("Present 0", testCaseFindOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					Kind:       "BMMachine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
			ExpectError:   false,
			ExpectedIndex: 0,
		}),
		Entry("Present 1", testCaseFindOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				metav1.OwnerReference{
					Kind:       "BMMachine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
			ExpectError:   false,
			ExpectedIndex: 1,
		}),
		Entry("Present but different versions", testCaseFindOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				metav1.OwnerReference{
					Kind:       "BMMachine",
					APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
			ExpectError:   false,
			ExpectedIndex: 1,
		}),
		Entry("Wrong group", testCaseFindOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				metav1.OwnerReference{
					Kind:       "BMMachine",
					APIVersion: "nfrastructure.cluster.x-k8s.io/v1alpha1",
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
			ExpectError: true,
		}),
	)

	type testCaseOwnerRef struct {
		BMMachine  capm3.Metal3Machine
		OwnerRefs  []metav1.OwnerReference
		Controller bool
	}

	DescribeTable("Test DeleteOwnerRef",
		func(tc testCaseOwnerRef) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &tc.BMMachine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			refList, err := machineMgr.DeleteOwnerRef(tc.OwnerRefs)
			Expect(err).To(BeNil())
			_, err = machineMgr.FindOwnerRef(refList)
			Expect(err).NotTo(BeNil())
		},
		Entry("Empty list", testCaseOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{},
		}),
		Entry("Absent", testCaseOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 0", testCaseOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					Kind:       "BMMachine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 1", testCaseOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				metav1.OwnerReference{
					Kind:       "BMMachine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
		}),
	)

	DescribeTable("Test SetOwnerRef",
		func(tc testCaseOwnerRef) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &tc.BMMachine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			refList, err := machineMgr.SetOwnerRef(tc.OwnerRefs, tc.Controller)
			Expect(err).To(BeNil())
			index, err := machineMgr.FindOwnerRef(refList)
			Expect(err).To(BeNil())
			Expect(*refList[index].Controller).To(BeEquivalentTo(tc.Controller))
		},
		Entry("Empty list", testCaseOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{},
		}),
		Entry("Absent", testCaseOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 0", testCaseOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					Kind:       "BMMachine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 1", testCaseOwnerRef{
			BMMachine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				metav1.OwnerReference{
					Kind:       "BMMachine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
		}),
	)
})

//-----------------
// Helper functions
//-----------------
func setupSchemeMm() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := capm3.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := bmoapis.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := capi.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
}

func newConfig(UserDataNamespace string,
	labels map[string]string, reqs []capm3.HostSelectorRequirement,
) (*capm3.Metal3Machine, *corev1.ObjectReference) {
	config := capm3.Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "myns",
		},
		Spec: capm3.Metal3MachineSpec{
			Image: capm3.Image{
				URL:      testImageURL,
				Checksum: testImageChecksumURL,
			},
			UserData: &corev1.SecretReference{
				Name:      testUserDataSecretName,
				Namespace: UserDataNamespace,
			},
			HostSelector: capm3.HostSelector{
				MatchLabels:      labels,
				MatchExpressions: reqs,
			},
		},
	}

	infrastructureRef := &corev1.ObjectReference{
		Name:       "someothermachine",
		Namespace:  "myns",
		Kind:       "BMMachine",
		APIVersion: capm3.GroupVersion.String(),
	}
	return &config, infrastructureRef
}

func newMachine(machineName string, metal3machineName string,
	infraRef *corev1.ObjectReference,
) *capi.Machine {
	if machineName == "" {
		return &capi.Machine{}
	}

	if infraRef == nil {
		infraRef = &corev1.ObjectReference{}
	}

	data := base64.StdEncoding.EncodeToString(CloudInitData)

	machine := &capi.Machine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Machine",
			APIVersion: capi.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: "myns",
		},
		Spec: capi.MachineSpec{
			ClusterName:       clusterName,
			InfrastructureRef: *infraRef,
			Bootstrap: capi.Bootstrap{
				ConfigRef:      &corev1.ObjectReference{},
				Data:           &data,
				DataSecretName: nil,
			},
		},
	}
	return machine
}

func newMetal3Machine(name string,
	ownerRef *metav1.OwnerReference,
	spec *capm3.Metal3MachineSpec,
	status *capm3.Metal3MachineStatus,
	objMeta *metav1.ObjectMeta) *capm3.Metal3Machine {

	if name == "" {
		return &capm3.Metal3Machine{}
	}

	if objMeta == nil {
		objMeta = &metav1.ObjectMeta{
			Name:      name,
			Namespace: "myns",
		}
	}

	typeMeta := &metav1.TypeMeta{
		Kind:       "BMMachine",
		APIVersion: capm3.GroupVersion.String(),
	}

	if spec == nil {
		spec = &capm3.Metal3MachineSpec{}
	}
	if status == nil {
		status = &capm3.Metal3MachineStatus{}
	}

	return &capm3.Metal3Machine{
		TypeMeta:   *typeMeta,
		ObjectMeta: *objMeta,
		Spec:       *spec,
		Status:     *status,
	}
}

func newBareMetalHost(name string,
	spec *bmh.BareMetalHostSpec,
	state bmh.ProvisioningState,
	status *bmh.BareMetalHostStatus,
	powerOn bool,
	clusterlabel bool) *bmh.BareMetalHost {

	if name == "" {
		return &bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "myns",
			},
		}
	}

	objMeta := &metav1.ObjectMeta{
		Name:      name,
		Namespace: "myns",
	}

	if clusterlabel == true {
		objMeta = &metav1.ObjectMeta{
			Name:      name,
			Namespace: "myns",
			Labels: map[string]string{
				capi.ClusterLabelName: clusterName,
				"foo":                 "bar",
			},
		}
	}

	if spec == nil {
		return &bmh.BareMetalHost{
			ObjectMeta: *objMeta,
		}
	}

	if status != nil {
		status.Provisioning.State = state
		status.PoweredOn = powerOn
	} else {
		status = &bmh.BareMetalHostStatus{}
	}

	return &bmh.BareMetalHost{
		ObjectMeta: *objMeta,
		Spec:       *spec,
		Status:     *status,
	}
}

func newBMCSecret(name string, clusterlabel bool) *corev1.Secret {
	//objMeta := &metav1.ObjectMeta{}
	objMeta := &metav1.ObjectMeta{
		Name:      name,
		Namespace: "myns",
	}
	if clusterlabel == true {
		objMeta = &metav1.ObjectMeta{
			Name:      name,
			Namespace: "myns",
			Labels: map[string]string{
				capi.ClusterLabelName: clusterName,
				"foo":                 "bar",
			},
		}
	}
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: *objMeta,
		Type:       "Opaque",
	}
}

func newSecret() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "mybmmachine-user-data",
			Namespace:  "myns",
			Finalizers: []string{"abcd"},
		},
		Data: map[string][]byte{
			"userData": []byte("QmFyRm9vCg=="),
		},
		Type: "Opaque",
	}
}
