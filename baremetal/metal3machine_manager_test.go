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
	"encoding/json"
	"fmt"
	"log"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
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
	testImageURL              = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2"
	testImageChecksumURL      = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum"
	testUserDataSecretName    = "worker-user-data"
	testMetaDataSecretName    = "worker-metadata"
	testNetworkDataSecretName = "worker-network-data"
)

var ProviderID = "metal3://12345ID6789"
var CloudInitData = []byte("metal3:cloudInitData1010101test__hello")
var testImageDiskFormat = pointer.StringPtr("raw")

func m3mSpec() *capm3.Metal3MachineSpec {
	return &capm3.Metal3MachineSpec{
		ProviderID: &ProviderID,
	}
}

func m3mSpecAll() *capm3.Metal3MachineSpec {
	return &capm3.Metal3MachineSpec{
		ProviderID: &ProviderID,
		UserData: &corev1.SecretReference{
			Name:      "mym3machine-user-data",
			Namespace: "myns",
		},
		Image: capm3.Image{
			URL:          testImageURL,
			Checksum:     testImageChecksumURL,
			ChecksumType: pointer.StringPtr("sha512"),
			DiskFormat:   testImageDiskFormat,
		},
		HostSelector: capm3.HostSelector{},
	}
}

func m3mSecretStatus() *capm3.Metal3MachineStatus {
	return &capm3.Metal3MachineStatus{
		UserData: &corev1.SecretReference{
			Name:      "mym3machine-user-data",
			Namespace: "myns",
		},
		MetaData: &corev1.SecretReference{
			Name:      "mym3machine-meta-data",
			Namespace: "myns",
		},
		NetworkData: &corev1.SecretReference{
			Name:      "mym3machine-network-data",
			Namespace: "myns",
		},
	}
}

func m3mSecretStatusNil() *capm3.Metal3MachineStatus {
	return &capm3.Metal3MachineStatus{}
}

func consumerRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name:       "mym3machine",
		Namespace:  "myns",
		Kind:       "M3Machine",
		APIVersion: capm3.GroupVersion.String(),
	}
}

func consumerRefSome() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name:       "someoneelsesmachine",
		Namespace:  "myns",
		Kind:       "M3Machine",
		APIVersion: capi.GroupVersion.String(),
	}
}

func expectedImg() *bmh.Image {
	return &bmh.Image{
		URL:        testImageURL,
		Checksum:   testImageChecksumURL,
		DiskFormat: testImageDiskFormat,
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
		MetaData:    &corev1.SecretReference{},
		NetworkData: &corev1.SecretReference{},
		UserData:    &corev1.SecretReference{},
	}
}

func bmhSpecNoImg() *bmh.BareMetalHostSpec {
	return &bmh.BareMetalHostSpec{
		ConsumerRef: consumerRef(),
	}
}

func m3mObjectMetaWithValidAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "mym3machine",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
		Labels: map[string]string{
			capi.ClusterLabelName: clusterName,
		},
		Annotations: map[string]string{
			HostAnnotation: "myns/myhost",
		},
	}
}

func bmhObjectMetaWithValidCAPM3PausedAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "myhost",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
		Labels: map[string]string{
			capi.ClusterLabelName: clusterName,
		},
		Annotations: map[string]string{
			bmh.PausedAnnotation: PausedAnnotationKey,
		},
	}
}

func bmhObjectMetaWithValidEmptyPausedAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "myhost",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
		Annotations: map[string]string{
			bmh.PausedAnnotation: "",
		},
	}
}

func bmhObjectMetaEmptyAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "myhost",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
		Annotations:     map[string]string{},
	}
}

func bmhObjectMetaNoAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "myhost",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
	}
}

func m3mObjectMetaWithInvalidAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "foobarm3machine",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
		Annotations: map[string]string{
			HostAnnotation: "myns/wrongvalue",
		},
	}
}

func m3mObjectMetaWithSomeAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "m3machine",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
		Annotations: map[string]string{
			HostAnnotation: "myns/somehost",
		},
	}
}

func m3mObjectMetaEmptyAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "m3machine",
		Namespace:       "myns",
		OwnerReferences: []metav1.OwnerReference{},
		Annotations:     map[string]string{},
	}
}

func m3mObjectMetaNoAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "m3machine",
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
		M3Machine  capm3.Metal3Machine
		ExpectTrue bool
	}

	DescribeTable("Test IsProvisioned",
		func(tc testCaseProvisioned) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &tc.M3Machine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			provisioningState := machineMgr.IsProvisioned()

			Expect(provisioningState).To(Equal(tc.ExpectTrue))
		},
		Entry("provisioned", testCaseProvisioned{
			M3Machine: capm3.Metal3Machine{
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
			M3Machine: capm3.Metal3Machine{
				Spec: capm3.Metal3MachineSpec{
					ProviderID: pointer.StringPtr("abc"),
				},
			},
			ExpectTrue: false,
		}),
		Entry("missing providerID", testCaseProvisioned{
			M3Machine: capm3.Metal3Machine{
				Status: capm3.Metal3MachineStatus{
					Ready: true,
				},
			},
			ExpectTrue: false,
		}),
		Entry("missing ProviderID and ready", testCaseProvisioned{
			M3Machine:  capm3.Metal3Machine{},
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

			machineMgr.SetError("abc", capierrors.InvalidConfigurationMachineError)

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
					Kind:       "M3Machine",
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
					Kind:       "M3Machine",
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
		hostWithUnhealthyAnnotation := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "hostWithUnhealthyAnnotation",
				Namespace:   "myns",
				Annotations: map[string]string{capm3.UnhealthyAnnotation: "unhealthy"},
			},
		}

		m3mconfig, infrastructureRef := newConfig("", map[string]string{},
			[]capm3.HostSelectorRequirement{},
		)
		m3mconfig2, infrastructureRef2 := newConfig("",
			map[string]string{"key1": "value1"}, []capm3.HostSelectorRequirement{},
		)
		m3mconfig3, infrastructureRef3 := newConfig("",
			map[string]string{"boguskey": "value"}, []capm3.HostSelectorRequirement{},
		)
		m3mconfig4, infrastructureRef4 := newConfig("", map[string]string{},
			[]capm3.HostSelectorRequirement{
				{
					Key:      "key1",
					Operator: "in",
					Values:   []string{"abc", "value1", "123"},
				},
			},
		)
		m3mconfig5, infrastructureRef5 := newConfig("", map[string]string{},
			[]capm3.HostSelectorRequirement{
				{
					Key:      "key1",
					Operator: "pancakes",
					Values:   []string{"abc", "value1", "123"},
				},
			},
		)

		type testCaseChooseHost struct {
			Machine          *capi.Machine
			Hosts            []runtime.Object
			M3Machine        *capm3.Metal3Machine
			ExpectedHostName string
		}

		DescribeTable("Test ChooseHost",
			func(tc testCaseChooseHost) {
				c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.Hosts...)
				machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
					tc.M3Machine, klogr.New(),
				)
				Expect(err).NotTo(HaveOccurred())

				result, _, err := machineMgr.chooseHost(context.TODO())

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
				M3Machine:        m3mconfig2,
				ExpectedHostName: host2.Name,
			}),
			Entry("Ignore discoveredHost and pick host2, which lacks a ConsumerRef",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef2),
					Hosts:            []runtime.Object{&discoveredHost, &host2, &host1},
					M3Machine:        m3mconfig2,
					ExpectedHostName: host2.Name,
				},
			),
			Entry("Ignore hostWithUnhealthyAnnotation and pick host2, which lacks a ConsumerRef",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef2),
					Hosts:            []runtime.Object{&hostWithUnhealthyAnnotation, &host1, &host2},
					M3Machine:        m3mconfig2,
					ExpectedHostName: host2.Name,
				},
			),
			Entry("Pick host3, which has a matching ConsumerRef", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef3),
				Hosts:            []runtime.Object{&host1, &host3, &host2},
				M3Machine:        m3mconfig3,
				ExpectedHostName: host3.Name,
			}),
			Entry("Two hosts already taken, third is in another namespace",
				testCaseChooseHost{
					Machine:          newMachine("machine2", "", infrastructureRef),
					Hosts:            []runtime.Object{&host1, &host3, &host4},
					M3Machine:        m3mconfig,
					ExpectedHostName: "",
				},
			),
			Entry("Choose hosts with a label, even without a label selector",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef),
					Hosts:            []runtime.Object{&hostWithLabel},
					M3Machine:        m3mconfig,
					ExpectedHostName: hostWithLabel.Name,
				},
			),
			Entry("Choose the host with the right label", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef2),
				Hosts:            []runtime.Object{&hostWithLabel, &host2},
				M3Machine:        m3mconfig2,
				ExpectedHostName: hostWithLabel.Name,
			}),
			Entry("No host that matches required label", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef3),
				Hosts:            []runtime.Object{&host2, &hostWithLabel},
				M3Machine:        m3mconfig3,
				ExpectedHostName: "",
			}),
			Entry("Host that matches a matchExpression", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef4),
				Hosts:            []runtime.Object{&host2, &hostWithLabel},
				M3Machine:        m3mconfig4,
				ExpectedHostName: hostWithLabel.Name,
			}),
			Entry("No Host available that matches a matchExpression",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef4),
					Hosts:            []runtime.Object{&host2},
					M3Machine:        m3mconfig4,
					ExpectedHostName: "",
				},
			),
			Entry("No host chosen, invalid match expression", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef5),
				Hosts:            []runtime.Object{&host2, &hostWithLabel, &host1},
				M3Machine:        m3mconfig5,
				ExpectedHostName: "",
			}),
		)
	})

	type testCaseSetPauseAnnotation struct {
		M3Machine           *capm3.Metal3Machine
		Host                *bmh.BareMetalHost
		ExpectPausePresent  bool
		ExpectStatusPresent bool
		ExpectError         bool
	}

	DescribeTable("Test Set BMH Pause Annotation",
		func(tc testCaseSetPauseAnnotation) {
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.Host, tc.M3Machine)
			machineMgr, err := NewMachineManager(c, nil, nil, nil, tc.M3Machine, klogr.New())
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.SetPauseAnnotation(context.TODO())
			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			savedHost := bmh.BareMetalHost{}
			err = c.Get(context.TODO(),
				client.ObjectKey{
					Name:      tc.Host.Name,
					Namespace: tc.Host.Namespace,
				},
				&savedHost,
			)
			Expect(err).NotTo(HaveOccurred())
			_, pausePresent := savedHost.Annotations[bmh.PausedAnnotation]
			if tc.ExpectPausePresent {
				Expect(pausePresent).To(BeTrue())
			} else {
				Expect(pausePresent).To(BeFalse())
			}
			status, statusPresent := savedHost.Annotations[bmh.StatusAnnotation]
			if tc.ExpectStatusPresent {
				Expect(statusPresent).To(BeTrue())
				annotation, err := json.Marshal(&tc.Host.Status)
				Expect(err).To(BeNil())
				Expect(status).To(Equal(string(annotation)))
			} else {
				Expect(statusPresent).To(BeFalse())
			}
		},
		Entry("Set BMH Pause Annotation, with valid CAPM3 Paused annotations, already paused", testCaseSetPauseAnnotation{
			Host: &bmh.BareMetalHost{
				ObjectMeta: *bmhObjectMetaWithValidCAPM3PausedAnnotations(),
				Spec: bmh.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       "mym3machine",
						Namespace:  "myns",
						Kind:       "M3Machine",
						APIVersion: capm3.GroupVersion.String(),
					},
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPausePresent: true,
			ExpectError:        false,
		}),
		Entry("Set BMH Pause Annotation, with valid Paused annotations, Empty Key, already paused", testCaseSetPauseAnnotation{
			Host: &bmh.BareMetalHost{
				ObjectMeta: *bmhObjectMetaWithValidEmptyPausedAnnotations(),
				Spec: bmh.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       "mym3machine",
						Namespace:  "myns",
						Kind:       "M3Machine",
						APIVersion: capm3.GroupVersion.String(),
					},
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPausePresent: true,
			ExpectError:        false,
		}),
		Entry("Set BMH Pause Annotation, with no Paused annotations", testCaseSetPauseAnnotation{
			Host: &bmh.BareMetalHost{
				ObjectMeta: *bmhObjectMetaEmptyAnnotations(),
				Spec: bmh.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       "mym3machine",
						Namespace:  "myns",
						Kind:       "M3Machine",
						APIVersion: capm3.GroupVersion.String(),
					},
				},
				Status: bmh.BareMetalHostStatus{
					OperationalStatus: "OK",
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPausePresent:  true,
			ExpectStatusPresent: true,
			ExpectError:         false,
		}),
	)

	type testCaseRemovePauseAnnotation struct {
		Cluster       *capi.Cluster
		M3Machine     *capm3.Metal3Machine
		Host          *bmh.BareMetalHost
		ExpectPresent bool
		ExpectError   bool
	}

	DescribeTable("Test Remove BMH Pause Annotation",
		func(tc testCaseRemovePauseAnnotation) {
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.Host, tc.M3Machine, tc.Cluster)
			machineMgr, err := NewMachineManager(c, tc.Cluster, nil, nil, tc.M3Machine, klogr.New())
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.RemovePauseAnnotation(context.TODO())
			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			savedHost := bmh.BareMetalHost{}
			err = c.Get(context.TODO(),
				client.ObjectKey{
					Name:      tc.Host.Name,
					Namespace: tc.Host.Namespace,
				},
				&savedHost,
			)
			Expect(err).NotTo(HaveOccurred())
			if tc.ExpectPresent {
				Expect(savedHost.Annotations[bmh.PausedAnnotation]).NotTo(BeNil())
			} else {
				Expect(savedHost.Annotations).To(BeNil())
			}
		},
		Entry("Remove BMH Pause Annotation, with valid CAPM3 Paused annotations", testCaseRemovePauseAnnotation{
			Cluster: newCluster(clusterName),
			Host: &bmh.BareMetalHost{
				ObjectMeta: *bmhObjectMetaWithValidCAPM3PausedAnnotations(),
				Spec: bmh.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       "mym3machine",
						Namespace:  "myns",
						Kind:       "M3Machine",
						APIVersion: capm3.GroupVersion.String(),
					},
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPresent: false,
			ExpectError:   false,
		}),
		Entry("Do not Remove Annotation, with valid Paused annotations, Empty Key", testCaseRemovePauseAnnotation{
			Cluster: newCluster(clusterName),
			Host: &bmh.BareMetalHost{
				ObjectMeta: *bmhObjectMetaWithValidEmptyPausedAnnotations(),
				Spec: bmh.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       "mym3machine",
						Namespace:  "myns",
						Kind:       "M3Machine",
						APIVersion: capm3.GroupVersion.String(),
					},
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPresent: true,
			ExpectError:   false,
		}),
		Entry("No Annotation, Should Not Error", testCaseRemovePauseAnnotation{
			Cluster: newCluster(clusterName),
			Host: &bmh.BareMetalHost{
				ObjectMeta: *bmhObjectMetaNoAnnotations(),
				Spec: bmh.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       "mym3machine",
						Namespace:  "myns",
						Kind:       "M3Machine",
						APIVersion: capm3.GroupVersion.String(),
					},
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPresent: false,
			ExpectError:   false,
		}),
	)

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

			m3mconfig, infrastructureRef := newConfig(tc.UserDataNamespace,
				map[string]string{}, []capm3.HostSelectorRequirement{},
			)
			machine := newMachine("machine1", "", infrastructureRef)

			machineMgr, err := NewMachineManager(c, nil, nil, machine, m3mconfig,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.setHostSpec(context.TODO(), tc.Host)
			Expect(err).NotTo(HaveOccurred())

			Expect(err).NotTo(HaveOccurred())

			// validate the saved host
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
			if tc.ExpectUserData {
				Expect(tc.Host.Spec.MetaData).NotTo(BeNil())
				Expect(tc.Host.Spec.MetaData.Namespace).
					To(Equal(tc.ExpectedUserDataNamespace))
				Expect(tc.Host.Spec.MetaData.Name).To(Equal(testMetaDataSecretName))
			} else {
				Expect(tc.Host.Spec.MetaData).To(BeNil())
			}
			if tc.ExpectUserData {
				Expect(tc.Host.Spec.NetworkData).NotTo(BeNil())
				Expect(tc.Host.Spec.NetworkData.Namespace).
					To(Equal(tc.ExpectedUserDataNamespace))
				Expect(tc.Host.Spec.NetworkData.Name).To(Equal(testNetworkDataSecretName))
			} else {
				Expect(tc.Host.Spec.NetworkData).To(BeNil())
			}
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

	DescribeTable("Test SetHostConsumerRef",
		func(tc testCaseSetHostSpec) {
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.Host)

			m3mconfig, infrastructureRef := newConfig(tc.UserDataNamespace,
				map[string]string{}, []capm3.HostSelectorRequirement{},
			)
			machine := newMachine("machine1", "", infrastructureRef)

			machineMgr, err := NewMachineManager(c, nil, nil, machine, m3mconfig,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.setHostConsumerRef(context.TODO(), tc.Host)
			Expect(err).NotTo(HaveOccurred())

			// validate the saved host
			Expect(tc.Host.Spec.ConsumerRef).NotTo(BeNil())
			Expect(tc.Host.Spec.ConsumerRef.Name).To(Equal(m3mconfig.Name))
			Expect(tc.Host.Spec.ConsumerRef.Namespace).
				To(Equal(m3mconfig.Namespace))
			Expect(tc.Host.Spec.ConsumerRef.Kind).To(Equal("Metal3Machine"))
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
			M3Machine *capm3.Metal3Machine
			Expected  bool
		}

		DescribeTable("Test Exists function",
			func(tc testCaseExists) {
				machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
					tc.M3Machine, klogr.New(),
				)
				Expect(err).NotTo(HaveOccurred())

				result, err := machineMgr.exists(context.TODO())
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(tc.Expected))
			},
			Entry("Failed to find the existing host", testCaseExists{
				Machine: &capi.Machine{},
				M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
					m3mObjectMetaWithSomeAnnotations(),
				),
				Expected: true,
			}),
			Entry("Found host even though annotation value is incorrect",
				testCaseExists{
					Machine: &capi.Machine{},
					M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
						m3mObjectMetaWithInvalidAnnotations(),
					),
					Expected: false,
				},
			),
			Entry("Found host even though annotation not present", testCaseExists{
				Machine: &capi.Machine{},
				M3Machine: newMetal3Machine("", nil, nil, nil,
					m3mObjectMetaEmptyAnnotations(),
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
			M3Machine     *capm3.Metal3Machine
			ExpectPresent bool
		}

		DescribeTable("Test GetHost",
			func(tc testCaseGetHost) {
				machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
					tc.M3Machine, klogr.New(),
				)
				Expect(err).NotTo(HaveOccurred())

				result, _, err := machineMgr.getHost(context.TODO())
				Expect(err).NotTo(HaveOccurred())
				if tc.ExpectPresent {
					Expect(result).NotTo(BeNil())
				} else {
					Expect(result).To(BeNil())
				}
			},
			Entry("Should find the expected host", testCaseGetHost{
				Machine: &capi.Machine{},
				M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
					m3mObjectMetaWithValidAnnotations(),
				),
				ExpectPresent: true,
			}),
			Entry("Should not find the host, annotation value incorrect",
				testCaseGetHost{
					Machine: &capi.Machine{},
					M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
						m3mObjectMetaWithInvalidAnnotations(),
					),
					ExpectPresent: false,
				},
			),
			Entry("Should not find the host, annotation not present", testCaseGetHost{
				Machine: &capi.Machine{},
				M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
					m3mObjectMetaEmptyAnnotations(),
				),
				ExpectPresent: false,
			}),
		)
	})

	type testCaseGetSetProviderID struct {
		Machine       *capi.Machine
		M3Machine     *capm3.Metal3Machine
		Host          *bmh.BareMetalHost
		ExpectPresent bool
		ExpectError   bool
	}

	DescribeTable("Test Get and Set Provider ID",
		func(tc testCaseGetSetProviderID) {
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.Host)
			machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
				tc.M3Machine, klogr.New(),
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
			Expect(*tc.M3Machine.Spec.ProviderID).To(Equal(providerID))
		},
		Entry("Set ProviderID, empty annotations", testCaseGetSetProviderID{
			Machine: newMachine("", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, m3mSpec(), nil,
				m3mObjectMetaEmptyAnnotations(),
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
			M3Machine: newMetal3Machine("mym3machine", nil, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations(),
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
			M3Machine: newMetal3Machine("mym3machine", nil, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations(),
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
			ExpectError:   false,
		}),
	)

	Describe("Test utility functions", func() {
		type testCaseSmallFunctions struct {
			Machine        *capi.Machine
			M3Machine      *capm3.Metal3Machine
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
					tc.M3Machine, klogr.New(),
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
				M3Machine: newMetal3Machine("mym3machine", nil, m3mSpec(), nil,
					m3mObjectMetaEmptyAnnotations(),
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
				M3Machine: newMetal3Machine("mym3machine", nil, m3mSpec(), nil,
					m3mObjectMetaEmptyAnnotations(),
				),
				ExpectCtrlNode: true,
			}),
		)
	})

	type testCaseEnsureAnnotation struct {
		Machine          capi.Machine
		Host             *bmh.BareMetalHost
		M3Machine        *capm3.Metal3Machine
		ExpectAnnotation bool
	}

	DescribeTable("Test EnsureAnnotation",
		func(tc testCaseEnsureAnnotation) {
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), tc.M3Machine)

			machineMgr, err := NewMachineManager(c, nil, nil, &tc.Machine,
				tc.M3Machine, klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.ensureAnnotation(context.TODO(), tc.Host)
			Expect(err).NotTo(HaveOccurred())

			annotations := tc.M3Machine.ObjectMeta.GetAnnotations()
			if !tc.ExpectAnnotation {
				Expect(annotations).To(BeNil())
			} else {
				Expect(annotations).NotTo(BeNil())
				Expect(annotations[HostAnnotation]).
					To(Equal(tc.M3Machine.Annotations[HostAnnotation]))
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
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
				m3mObjectMetaWithValidAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone, nil,
				false, false,
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotation exists but is wrong", testCaseEnsureAnnotation{
			Machine: capi.Machine{},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
				m3mObjectMetaWithInvalidAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone,
				nil, false, false,
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotations are empty", testCaseEnsureAnnotation{
			Machine: capi.Machine{},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
				m3mObjectMetaEmptyAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone,
				nil, false, false,
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotations are nil", testCaseEnsureAnnotation{
			Machine: capi.Machine{},
			M3Machine: newMetal3Machine("", nil, nil, nil,
				m3mObjectMetaNoAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone,
				nil, false, false,
			),
			ExpectAnnotation: true,
		}),
	)

	type testCaseDelete struct {
		Host                            *bmh.BareMetalHost
		Secret                          *corev1.Secret
		Machine                         *capi.Machine
		M3Machine                       *capm3.Metal3Machine
		BMCSecret                       *corev1.Secret
		ExpectedConsumerRef             *corev1.ObjectReference
		ExpectedResult                  error
		ExpectSecretDeleted             bool
		ExpectClusterLabelDeleted       bool
		ExpectedPausedAnnotationDeleted bool
	}

	DescribeTable("Test Delete function",
		func(tc testCaseDelete) {
			objects := []runtime.Object{tc.M3Machine}
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
				tc.M3Machine, klogr.New(),
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

				err := c.Get(context.TODO(), key, &host)
				Expect(err).NotTo(HaveOccurred())

				name := ""
				expectedName := ""
				if host.Spec.ConsumerRef != nil {
					name = host.Spec.ConsumerRef.Name
				}
				if tc.ExpectedConsumerRef != nil {
					expectedName = tc.ExpectedConsumerRef.Name
				}
				Expect(name).To(Equal(expectedName))
				if machineMgr.Metal3Machine.Status.MetaData == nil {
					Expect(host.Spec.MetaData).NotTo(BeNil())
				}
				if machineMgr.Metal3Machine.Status.NetworkData == nil {
					Expect(host.Spec.NetworkData).NotTo(BeNil())
				}
				if machineMgr.Metal3Machine.Status.UserData == nil {
					Expect(host.Spec.UserData).NotTo(BeNil())
				}
			}

			tmpBootstrapSecret := corev1.Secret{}
			if tc.M3Machine.Status.UserData != nil {
				key := client.ObjectKey{
					Name:      tc.M3Machine.Status.UserData.Name,
					Namespace: tc.M3Machine.Status.UserData.Namespace,
				}
				err = c.Get(context.TODO(), key, &tmpBootstrapSecret)
				if tc.ExpectSecretDeleted {
					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsNotFound(err)).To(BeTrue())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			if tc.ExpectedPausedAnnotationDeleted {
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
				Expect(savedHost.Annotations[bmh.PausedAnnotation]).NotTo(Equal(PausedAnnotationKey))
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
				bmh.StateProvisioned, bmhStatus(), false, true,
			),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef: consumerRef(),
			ExpectedResult:      &RequeueAfterError{},
			Secret:              newSecret(),
		}),
		Entry("No Host status, deprovisioning needed", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpec(), bmh.StateNone,
				nil, false, true,
			),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef: consumerRef(),
			ExpectedResult:      &RequeueAfterError{},
			Secret:              newSecret(),
		}),
		Entry("No Host status, no deprovisioning needed", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(), bmh.StateNone, nil,
				false, true,
			),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: true,
		}),
		Entry("Deprovisioning in progress", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(),
				bmh.StateDeprovisioning, bmhStatus(), false, true,
			),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef: consumerRef(),
			ExpectedResult:      &RequeueAfterError{RequeueAfter: time.Second * 30},
			Secret:              newSecret(),
		}),
		Entry("Externally provisioned host should be powered down", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(),
				bmh.StateExternallyProvisioned, bmhPowerStatus(), true, true,
			),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef: consumerRef(),
			ExpectedResult:      &RequeueAfterError{RequeueAfter: time.Second * 30},
			Secret:              newSecret(),
		}),
		Entry("Consumer ref should be removed from externally provisioned host",
			testCaseDelete{
				Host: newBareMetalHost("myhost", bmhSpecNoImg(),
					bmh.StateExternallyProvisioned, bmhPowerStatus(), false, true,
				),
				Machine: newMachine("mymachine", "", nil),
				M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
					m3mObjectMetaWithValidAnnotations(),
				),
				Secret:              newSecret(),
				ExpectSecretDeleted: true,
			},
		),
		Entry("Consumer ref should be removed from unmanaged host",
			testCaseDelete{
				Host: newBareMetalHost("myhost", bmhSpecNoImg(),
					bmh.StateUnmanaged, bmhPowerStatus(), false, true,
				),
				Machine: newMachine("mymachine", "", nil),
				M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
					m3mObjectMetaWithValidAnnotations(),
				),
				Secret:              newSecret(),
				ExpectSecretDeleted: true,
			},
		),
		Entry("Consumer ref should be removed, BMH state is available", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(), bmh.StateAvailable,
				bmhStatus(), false, true,
			),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: true,
		}),
		Entry("Consumer ref should be removed", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(), bmh.StateReady,
				bmhStatus(), false, true,
			),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: true,
		}),
		Entry("Consumer ref should be removed, secret not deleted", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(), bmh.StateReady,
				bmhStatus(), false, true,
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
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret: newSecret(),
		}),
		Entry("Consumer ref does not match, so it should not be removed",
			testCaseDelete{
				Host: newBareMetalHost("myhost", bmhSpecSomeImg(),
					bmh.StateProvisioned, bmhStatus(), false, true,
				),
				Machine: newMachine("", "", nil),
				M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
					m3mObjectMetaWithValidAnnotations(),
				),
				ExpectedConsumerRef: consumerRefSome(),
				Secret:              newSecret(),
			},
		),
		Entry("No consumer ref, so this is a no-op", testCaseDelete{
			Host:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, true),
			Machine: newMachine("", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: false,
		}),
		Entry("No host at all, so this is a no-op", testCaseDelete{
			Host:    nil,
			Machine: newMachine("", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: false,
		}),
		Entry("dataSecretName set, deleting secret", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(), bmh.StateNone, nil,
				false, true,
			),
			Machine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns",
				},
				Spec: capi.MachineSpec{
					Bootstrap: capi.Bootstrap{
						DataSecretName: pointer.StringPtr("mym3machine-user-data"),
					},
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: false,
		}),
		Entry("Clusterlabel should be removed", testCaseDelete{
			Machine:                   newMachine("mymachine", "mym3machine", nil),
			M3Machine:                 newMetal3Machine("mym3machine", nil, m3mSpecAll(), m3mSecretStatus(), m3mObjectMetaWithValidAnnotations()),
			Host:                      newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, true),
			BMCSecret:                 newBMCSecret("mycredentials", true),
			ExpectSecretDeleted:       true,
			ExpectClusterLabelDeleted: true,
		}),
		Entry("PausedAnnotation/CAPM3 should be removed", testCaseDelete{
			Machine:   newMachine("mymachine", "mym3machine", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, m3mSpecAll(), m3mSecretStatus(), m3mObjectMetaWithValidAnnotations()),
			Host: &bmh.BareMetalHost{
				ObjectMeta: *bmhObjectMetaWithValidCAPM3PausedAnnotations(),
				Spec:       *bmhSpecBMC(),
				Status: bmh.BareMetalHostStatus{
					Provisioning: bmh.ProvisionStatus{
						State: bmh.StateNone,
					},
					PoweredOn: false,
				},
			},
			BMCSecret:                       newBMCSecret("mycredentials", false),
			ExpectSecretDeleted:             true,
			ExpectedPausedAnnotationDeleted: true,
		}),
		Entry("No clusterLabel in BMH or BMC Secret so this is a no-op ", testCaseDelete{
			Machine:                   newMachine("mymachine", "mym3machine", nil),
			M3Machine:                 newMetal3Machine("mym3machine", nil, m3mSpecAll(), m3mSecretStatus(), m3mObjectMetaWithValidAnnotations()),
			Host:                      newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, false),
			BMCSecret:                 newBMCSecret("mycredentials", false),
			ExpectSecretDeleted:       true,
			ExpectClusterLabelDeleted: false,
		}),
		Entry("BMH MetaData, NetworkData and UserData should not be cleaned on deprovisioning", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecSomeImg(),
				bmh.StateProvisioned, bmhStatus(), false, true,
			),
			Machine: newMachine("mymachine", "mym3machine", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatusNil(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectedConsumerRef: consumerRefSome(),
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
			M3Machine       capm3.Metal3Machine
		}

		DescribeTable("Test UpdateMachineStatus",
			func(tc testCaseUpdateMachineStatus) {
				c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), &tc.M3Machine)

				machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
					&tc.M3Machine, klogr.New(),
				)
				Expect(err).NotTo(HaveOccurred())

				err = machineMgr.updateMachineStatus(context.TODO(), tc.Host)
				Expect(err).NotTo(HaveOccurred())

				key := client.ObjectKey{
					Name:      tc.M3Machine.ObjectMeta.Name,
					Namespace: tc.M3Machine.ObjectMeta.Namespace,
				}
				m3machine := capm3.Metal3Machine{}
				err = c.Get(context.TODO(), key, &m3machine)
				Expect(err).NotTo(HaveOccurred())

				if tc.M3Machine.Status.Addresses != nil {
					for i, address := range tc.ExpectedMachine.Status.Addresses {
						Expect(m3machine.Status.Addresses[i]).To(Equal(address))
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
							{
								Address: "192.168.1.255",
								Type:    "InternalIP",
							},
							{
								Address: "172.0.20.255",
								Type:    "InternalIP",
							},
						},
					},
				},
				M3Machine: capm3.Metal3Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mym3machine",
						Namespace: "myns",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "M3Machine",
						APIVersion: capi.GroupVersion.String(),
					},
					Status: capm3.Metal3MachineStatus{
						Addresses: []capi.MachineAddress{
							{
								Address: "192.168.1.1",
								Type:    "InternalIP",
							},
							{
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
							{
								Address: "192.168.1.1",
								Type:    "InternalIP",
							},
							{
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
							{
								Address: "192.168.1.1",
								Type:    "InternalIP",
							},
							{
								Address: "172.0.20.2",
								Type:    "InternalIP",
							},
						},
					},
				},
				M3Machine: capm3.Metal3Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mym3machine",
						Namespace: "myns",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "M3Machine",
						APIVersion: capi.GroupVersion.String(),
					},
					Status: capm3.Metal3MachineStatus{
						Addresses: []capi.MachineAddress{
							{
								Address: "192.168.1.1",
								Type:    "InternalIP",
							},
							{
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
							{
								Address: "192.168.1.1",
								Type:    "InternalIP",
							},
							{
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
					M3Machine: capm3.Metal3Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "mym3machine",
							Namespace: "myns",
						},
						TypeMeta: metav1.TypeMeta{
							Kind:       "M3Machine",
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
			M3Machine             capm3.Metal3Machine
			Host                  *bmh.BareMetalHost
			ExpectedNodeAddresses []capi.MachineAddress
		}

		DescribeTable("Test NodeAddress",
			func(tc testCaseNodeAddress) {
				var nodeAddresses []capi.MachineAddress

				c := fakeclient.NewFakeClientWithScheme(setupSchemeMm())
				machineMgr, err := NewMachineManager(c, nil, nil, &tc.Machine,
					&tc.M3Machine, klogr.New(),
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

	type testCaseGetProviderIDAndBMHID struct {
		providerID    *string
		expectedBMHID string
	}

	DescribeTable("Test GetProviderIDAndBMHID",
		func(tc testCaseGetProviderIDAndBMHID) {
			m3m := capm3.Metal3Machine{
				Spec: capm3.Metal3MachineSpec{
					ProviderID: tc.providerID,
				},
			}

			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &m3m, klogr.New())
			Expect(err).NotTo(HaveOccurred())

			providerID, bmhID := machineMgr.GetProviderIDAndBMHID()

			if tc.providerID != nil {
				Expect(providerID).To(Equal(*tc.providerID))
				Expect(bmhID).NotTo(BeNil())
				Expect(*bmhID).To(Equal(tc.expectedBMHID))
			} else {
				Expect(providerID).To(Equal(""))
				Expect(bmhID).To(BeNil())
			}
		},
		Entry("Empty providerID", testCaseGetProviderIDAndBMHID{}),
		Entry("Provider ID set", testCaseGetProviderIDAndBMHID{
			providerID:    pointer.StringPtr("metal3://abcd"),
			expectedBMHID: "abcd",
		}),
	)

	Describe("Test SetNodeProviderID", func() {
		scheme := runtime.NewScheme()
		err := capi.AddToScheme(scheme)
		if err != nil {
			log.Printf("AddToScheme failed: %v", err)
		}
		err = bmh.AddToScheme(scheme)
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
				node, err := corev1Client.Nodes().Get(tc.Node.Name,
					metav1.GetOptions{},
				)
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
		M3Machine   *capm3.Metal3Machine
		BMHost      *bmh.BareMetalHost
		Secret      *corev1.Secret
		ExpectError bool
	}

	DescribeTable("Test getUserData function",
		func(tc testCaseGetUserData) {
			objects := []runtime.Object{
				tc.M3Machine,
				tc.Machine,
			}
			if tc.Secret != nil {
				objects = append(objects, tc.Secret)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)

			machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
				tc.M3Machine, klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.getUserData(context.TODO(), tc.BMHost)
			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
				return
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			// Expect the reference to the secret to be passed through
			if tc.Machine.Spec.Bootstrap.DataSecretName != nil &&
				tc.Machine.Namespace == tc.BMHost.Namespace {
				Expect(tc.M3Machine.Status.UserData.Name).To(Equal(
					*tc.Machine.Spec.Bootstrap.DataSecretName,
				))
				Expect(tc.M3Machine.Status.UserData.Namespace).To(Equal(
					tc.BMHost.Namespace,
				))
			}

			// if we had to create an additional secret (dataSecretName not set and
			// Data set)
			if tc.Machine.Spec.Bootstrap.DataSecretName == nil &&
				tc.Machine.Spec.Bootstrap.Data != nil {

				Expect(tc.M3Machine.Status.UserData.Name).To(Equal(
					tc.M3Machine.Name + "-user-data",
				))
				Expect(tc.M3Machine.Status.UserData.Namespace).To(Equal(
					tc.M3Machine.Namespace,
				))
				tmpBootstrapSecret := corev1.Secret{}
				key := client.ObjectKey{
					Name:      tc.M3Machine.Status.UserData.Name,
					Namespace: tc.M3Machine.Namespace,
				}
				err = c.Get(context.TODO(), key, &tmpBootstrapSecret)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(tmpBootstrapSecret.Data["userData"])).To(Equal("FooBar\n"))
				Expect(len(tmpBootstrapSecret.OwnerReferences)).To(BeEquivalentTo(1))
				Expect(tmpBootstrapSecret.OwnerReferences[0].APIVersion).
					To(Equal(tc.M3Machine.APIVersion))
				Expect(tmpBootstrapSecret.OwnerReferences[0].Kind).
					To(Equal(tc.M3Machine.Kind))
				Expect(tmpBootstrapSecret.OwnerReferences[0].Name).
					To(Equal(tc.M3Machine.Name))
				Expect(tmpBootstrapSecret.OwnerReferences[0].UID).
					To(Equal(tc.M3Machine.UID))
				Expect(*tmpBootstrapSecret.OwnerReferences[0].Controller).
					To(BeTrue())
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
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
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
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
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
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
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
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
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
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
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
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
		}),
		Entry("No userData in Machine", testCaseGetUserData{
			Machine:   &capi.Machine{},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
		}),
	)

	type testCaseAssociate struct {
		Machine            *capi.Machine
		Host               *bmh.BareMetalHost
		M3Machine          *capm3.Metal3Machine
		BMCSecret          *corev1.Secret
		ExpectRequeue      bool
		ExpectClusterLabel bool
		ExpectOwnerRef     bool
	}

	DescribeTable("Test Associate function",
		func(tc testCaseAssociate) {
			objects := []runtime.Object{
				tc.M3Machine,
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
				tc.M3Machine, klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.Associate(context.TODO())
			if tc.ExpectRequeue {
				_, ok := errors.Cause(err).(HasRequeueAfterError)
				fmt.Println(errors.Cause(err))
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
				M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
					m3mObjectMetaWithValidAnnotations(),
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
				M3Machine: newMetal3Machine("mym3machine", nil, m3mSpecAll(), nil,
					m3mObjectMetaWithValidAnnotations(),
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
				M3Machine: newMetal3Machine("mym3machine", nil, m3mSpecAll(), nil,
					m3mObjectMetaWithValidAnnotations(),
				),
				Host:           newBareMetalHost("", nil, bmh.StateNone, nil, false, false),
				ExpectRequeue:  true,
				ExpectOwnerRef: false,
			},
		),
		Entry("Associate machine, host nil, Metal3 machine spec set, requeue",
			testCaseAssociate{
				Machine: newMachine("myUniqueMachine", "", nil),
				M3Machine: newMetal3Machine("mym3machine", nil, m3mSpecAll(), nil,
					m3mObjectMetaWithValidAnnotations(),
				),
				Host:          nil,
				ExpectRequeue: true,
			},
		),
		Entry("Associate machine, host set, Metal3 machine spec set, set clusterLabel",
			testCaseAssociate{
				Machine:            newMachine("mymachine", "mym3machine", nil),
				M3Machine:          newMetal3Machine("mym3machine", nil, m3mSpecAll(), nil, nil),
				Host:               newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, false),
				BMCSecret:          newBMCSecret("mycredentials", false),
				ExpectClusterLabel: true,
				ExpectRequeue:      false,
				ExpectOwnerRef:     true,
			},
		),
		Entry("Associate machine with DataTemplate missing",
			testCaseAssociate{
				Machine: newMachine("mymachine", "mym3machine", nil),
				M3Machine: newMetal3Machine("mym3machine", nil,
					&capm3.Metal3MachineSpec{
						DataTemplate: &corev1.ObjectReference{
							Name:      "abcd",
							Namespace: "myns",
						},
						UserData: &corev1.SecretReference{
							Name:      "mym3machine-user-data",
							Namespace: "myns",
						},
						Image: capm3.Image{
							URL:        testImageURL,
							Checksum:   testImageChecksumURL,
							DiskFormat: testImageDiskFormat,
						},
					}, nil, nil,
				),
				Host:               newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, false),
				BMCSecret:          newBMCSecret("mycredentials", false),
				ExpectClusterLabel: true,
				ExpectRequeue:      false,
				ExpectOwnerRef:     true,
			},
		),
		Entry("Associate machine with DataTemplate and Data ready",
			testCaseAssociate{
				Machine: newMachine("mymachine", "mym3machine", nil),
				M3Machine: newMetal3Machine("mym3machine", nil,
					&capm3.Metal3MachineSpec{
						DataTemplate: &corev1.ObjectReference{
							Name:      "abcd",
							Namespace: "myns",
						},
						UserData: &corev1.SecretReference{
							Name:      "mym3machine-user-data",
							Namespace: "myns",
						},
						Image: capm3.Image{
							URL:        testImageURL,
							Checksum:   testImageChecksumURL,
							DiskFormat: testImageDiskFormat,
						},
					}, &capm3.Metal3MachineStatus{
						RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: "myns"},
					}, nil,
				),
				Host:               newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, false),
				BMCSecret:          newBMCSecret("mycredentials", false),
				ExpectClusterLabel: true,
				ExpectRequeue:      false,
				ExpectOwnerRef:     true,
			},
		),
	)

	type testCaseUpdate struct {
		Machine     *capi.Machine
		Host        *bmh.BareMetalHost
		M3Machine   *capm3.Metal3Machine
		ExpectError bool
	}

	DescribeTable("Test Update function",
		func(tc testCaseUpdate) {
			objects := []runtime.Object{
				tc.Host,
				tc.M3Machine,
				tc.Machine,
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)

			machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine,
				tc.M3Machine, klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.Update(context.TODO())
			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Update machine", testCaseUpdate{
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
				m3mObjectMetaWithValidAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
		}),
		Entry("Update machine, DataTemplate missing", testCaseUpdate{
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name: "abcd",
				},
			}, nil,
				m3mObjectMetaWithValidAnnotations(),
			),
			Host:        newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, false),
			ExpectError: true,
		}),
	)

	type testCaseFindOwnerRef struct {
		M3Machine     capm3.Metal3Machine
		OwnerRefs     []metav1.OwnerReference
		ExpectError   bool
		ExpectedIndex int
	}

	DescribeTable("Test FindOwnerRef",
		func(tc testCaseFindOwnerRef) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &tc.M3Machine,
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
			M3Machine:   *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs:   []metav1.OwnerReference{},
			ExpectError: true,
		}),
		Entry("Absent", testCaseFindOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
			ExpectError: true,
		}),
		Entry("Present 0", testCaseFindOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					Kind:       "M3Machine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
				{
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
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				{
					Kind:       "M3Machine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
			ExpectError:   false,
			ExpectedIndex: 1,
		}),
		Entry("Present but different versions", testCaseFindOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				{
					Kind:       "M3Machine",
					APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
			ExpectError:   false,
			ExpectedIndex: 1,
		}),
		Entry("Wrong group", testCaseFindOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				{
					Kind:       "M3Machine",
					APIVersion: "nfrastructure.cluster.x-k8s.io/v1alpha1",
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
			ExpectError: true,
		}),
	)

	type testCaseOwnerRef struct {
		M3Machine  capm3.Metal3Machine
		OwnerRefs  []metav1.OwnerReference
		Controller bool
	}

	DescribeTable("Test DeleteOwnerRef",
		func(tc testCaseOwnerRef) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &tc.M3Machine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			refList, err := machineMgr.DeleteOwnerRef(tc.OwnerRefs)
			Expect(err).To(BeNil())
			_, err = machineMgr.FindOwnerRef(refList)
			Expect(err).NotTo(BeNil())
		},
		Entry("Empty list", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{},
		}),
		Entry("Absent", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 0", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					Kind:       "M3Machine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 1", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				{
					Kind:       "M3Machine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					Kind:       "M3Machine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
		}),
	)

	DescribeTable("Test SetOwnerRef",
		func(tc testCaseOwnerRef) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &tc.M3Machine,
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
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{},
		}),
		Entry("Absent", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 0", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					Kind:       "M3Machine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 1", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				{
					Kind:       "M3Machine",
					APIVersion: capm3.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
		}),
	)

	type testCaseM3MetaData struct {
		M3Machine          *capm3.Metal3Machine
		Machine            *capi.Machine
		DataClaim          *capm3.Metal3DataClaim
		Data               *capm3.Metal3Data
		ExpectError        bool
		ExpectRequeue      bool
		ExpectDataStatus   bool
		ExpectSecretStatus bool
		expectClaim        bool
	}

	DescribeTable("Test AssociateM3MetaData",
		func(tc testCaseM3MetaData) {
			objects := []runtime.Object{}
			if tc.DataClaim != nil {
				objects = append(objects, tc.DataClaim)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine, tc.M3Machine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.AssociateM3Metadata(context.TODO())
			if tc.ExpectError || tc.ExpectRequeue {
				Expect(err).To(HaveOccurred())
				if tc.ExpectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.M3Machine.Spec.MetaData != nil {
				Expect(tc.M3Machine.Status.MetaData).To(Equal(tc.M3Machine.Spec.MetaData))
			}
			if tc.M3Machine.Spec.NetworkData != nil {
				Expect(tc.M3Machine.Status.NetworkData).To(Equal(tc.M3Machine.Spec.NetworkData))
			}
			if tc.expectClaim {
				dataTemplate := capm3.Metal3DataClaim{}
				err = c.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.M3Machine.Name,
						Namespace: tc.M3Machine.Namespace,
					},
					&dataTemplate,
				)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("No Spec", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, nil, nil),
		}),
		Entry("MetaData and NetworkData set in spec", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				MetaData:    &corev1.SecretReference{Name: "abcd"},
				NetworkData: &corev1.SecretReference{Name: "defg"},
			}, nil, nil),
		}),
		Entry("RenderedData set in status", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd"},
			}, nil),
		}),
		Entry("DataClaim does not exist yet", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine:     newMachine("myName", "myName", nil),
			expectClaim: true,
		}),
		Entry("DataClaim exists", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: "myns",
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "myName",
							Kind:       "M3Machine",
							APIVersion: capm3.GroupVersion.String(),
						},
					},
				},
			},
			expectClaim: true,
		}),
		Entry("DataClaim ready", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: "myns",
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "myName",
							Kind:       "M3Machine",
							APIVersion: capm3.GroupVersion.String(),
						},
					},
				},
				Status: capm3.Metal3DataClaimStatus{
					RenderedData: &corev1.ObjectReference{
						Name:      "abc",
						Namespace: "myns",
					},
				},
			},
			expectClaim: true,
		}),
	)

	DescribeTable("Test WaitForM3MetaData",
		func(tc testCaseM3MetaData) {
			objects := []runtime.Object{}
			if tc.DataClaim != nil {
				objects = append(objects, tc.DataClaim)
			}
			if tc.Data != nil {
				objects = append(objects, tc.Data)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine, tc.M3Machine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.WaitForM3Metadata(context.TODO())
			if tc.ExpectError || tc.ExpectRequeue {
				Expect(err).To(HaveOccurred())
				if tc.ExpectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.ExpectDataStatus {
				Expect(tc.M3Machine.Status.RenderedData).To(Equal(&corev1.ObjectReference{
					Name:      "abcd-0",
					Namespace: "myns",
				}))
			} else {
				Expect(tc.M3Machine.Status.RenderedData).To(BeNil())
			}
			if tc.ExpectSecretStatus {
				if tc.Data.Spec.MetaData != nil {
					Expect(tc.M3Machine.Status.MetaData).To(Equal(&corev1.SecretReference{
						Name:      tc.Data.Spec.MetaData.Name,
						Namespace: tc.Data.Namespace,
					}))
				} else {
					Expect(tc.M3Machine.Status.MetaData).To(BeNil())
				}
				if tc.Data.Spec.NetworkData != nil {
					Expect(tc.M3Machine.Status.NetworkData).To(Equal(&corev1.SecretReference{
						Name:      tc.Data.Spec.NetworkData.Name,
						Namespace: tc.Data.Namespace,
					}))
				} else {
					Expect(tc.M3Machine.Status.NetworkData).To(BeNil())
				}
			} else {
				Expect(tc.M3Machine.Status.MetaData).To(BeNil())
				Expect(tc.M3Machine.Status.NetworkData).To(BeNil())
			}
		},
		Entry("No Spec", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, nil, nil),
			Machine:   nil,
		}),
		Entry("No Data template", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine:       newMachine("myName", "myName", nil),
			ExpectRequeue: true,
		}),
		Entry("Data claim without status", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: "myns",
				},
			},
			ExpectRequeue: true,
		}),
		Entry("Data claim with empty status", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: "myns",
				},
				Status: capm3.Metal3DataClaimStatus{
					RenderedData: &corev1.ObjectReference{},
				},
			},
			ExpectRequeue: true,
		}),
		Entry("Data does not exist", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: "myns",
				},
				Spec: capm3.Metal3DataClaimSpec{
					Template: corev1.ObjectReference{
						Name:      "abcd",
						Namespace: "myns",
					},
				},
				Status: capm3.Metal3DataClaimStatus{
					RenderedData: &corev1.ObjectReference{
						Name:      "abcd-0",
						Namespace: "myns",
					},
				},
			},
			ExpectRequeue:    true,
			ExpectDataStatus: true,
		}),
		Entry("M3M status set, Data does not exist", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: "myns"},
			}, nil),
			Machine:          newMachine("myName", "myName", nil),
			ExpectRequeue:    true,
			ExpectDataStatus: true,
		}),
		Entry("Data not ready", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: "myns"},
			}, nil),
			Machine: newMachine("myName", "myName", nil),
			Data: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abcd-0",
					Namespace: "myns",
				},
			},
			ExpectRequeue:    true,
			ExpectDataStatus: true,
		}),
		Entry("Data ready, no secrets", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: "myns"},
			}, nil),
			Machine: newMachine("myName", "myName", nil),
			Data: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abcd-0",
					Namespace: "myns",
				},
				Status: capm3.Metal3DataStatus{
					Ready: true,
				},
			},
			ExpectDataStatus:   true,
			ExpectSecretStatus: true,
		}),
		Entry("Data ready with secrets", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: "myns"},
			}, nil),
			Machine: newMachine("myName", "myName", nil),
			Data: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abcd-0",
					Namespace: "myns",
				},
				Spec: capm3.Metal3DataSpec{
					MetaData: &corev1.SecretReference{
						Name: "metadata",
					},
					NetworkData: &corev1.SecretReference{
						Name: "networkdata",
					},
				},
				Status: capm3.Metal3DataStatus{
					Ready: true,
				},
			},
			ExpectDataStatus:   true,
			ExpectSecretStatus: true,
		}),
	)

	DescribeTable("Test DissociateM3MetaData",
		func(tc testCaseM3MetaData) {
			objects := []runtime.Object{}
			if tc.DataClaim != nil {
				objects = append(objects, tc.DataClaim)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			machineMgr, err := NewMachineManager(c, nil, nil, tc.Machine, tc.M3Machine,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.DissociateM3Metadata(context.TODO())
			if tc.ExpectError || tc.ExpectRequeue {
				Expect(err).To(HaveOccurred())
				if tc.ExpectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(tc.M3Machine.Status.RenderedData).To(BeNil())
			if tc.ExpectSecretStatus {
				Expect(tc.M3Machine.Status.MetaData).NotTo(BeNil())
				Expect(tc.M3Machine.Status.NetworkData).NotTo(BeNil())
			} else {
				Expect(tc.M3Machine.Status.MetaData).To(BeNil())
				Expect(tc.M3Machine.Status.NetworkData).To(BeNil())
			}

			if tc.DataClaim == nil {
				return
			}
			dataTemplate := capm3.Metal3DataClaim{}
			err = c.Get(context.TODO(),
				client.ObjectKey{
					Name:      tc.M3Machine.Name,
					Namespace: tc.M3Machine.Namespace,
				},
				&dataTemplate,
			)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		},
		Entry("No Spec", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				MetaData:    &corev1.SecretReference{Name: "abcd"},
				NetworkData: &corev1.SecretReference{Name: "defg"},
			}, nil),
			Machine: newMachine("myName", "myName", nil),
		}),
		Entry("MetaData and NetworkData set in spec and status", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				MetaData:    &corev1.SecretReference{Name: "abcd"},
				NetworkData: &corev1.SecretReference{Name: "defg"},
			}, &capm3.Metal3MachineStatus{
				MetaData:    &corev1.SecretReference{Name: "abcd"},
				NetworkData: &corev1.SecretReference{Name: "defg"},
			}, nil),
			Machine:            newMachine("myName", "myName", nil),
			ExpectSecretStatus: true,
		}),
		Entry("DataClaim not found", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name:      "abc",
					Namespace: "myns",
				},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
		}),
		Entry("DataClaim found", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name:      "abcd",
					Namespace: "myns",
				},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: "myns",
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
	if err := bmh.AddToScheme(s); err != nil {
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
				URL:        testImageURL,
				Checksum:   testImageChecksumURL,
				DiskFormat: testImageDiskFormat,
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
		Status: capm3.Metal3MachineStatus{
			UserData: &corev1.SecretReference{
				Name:      testUserDataSecretName,
				Namespace: UserDataNamespace,
			},
			MetaData: &corev1.SecretReference{
				Name:      testMetaDataSecretName,
				Namespace: UserDataNamespace,
			},
			NetworkData: &corev1.SecretReference{
				Name:      testNetworkDataSecretName,
				Namespace: UserDataNamespace,
			},
		},
	}

	infrastructureRef := &corev1.ObjectReference{
		Name:       "someothermachine",
		Namespace:  "myns",
		Kind:       "M3Machine",
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
		Kind:       "M3Machine",
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
			Name:       "mym3machine-user-data",
			Namespace:  "myns",
			Finalizers: []string{"abcd"},
		},
		Data: map[string][]byte{
			"userData": []byte("QmFyRm9vCg=="),
		},
		Type: "Opaque",
	}
}
