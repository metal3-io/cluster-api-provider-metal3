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
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-logr/logr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientfake "k8s.io/client-go/kubernetes/fake"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testImageURL              = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2"
	testImageChecksumURL      = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum"
	testUserDataSecretName    = "worker-user-data"
	testMetaDataSecretName    = "worker-metadata"
	testNetworkDataSecretName = "worker-network-data"
	kcpName                   = "kcp-pool1"
)

var Bmhuid = types.UID("4d25a2c2-46e4-11ec-81d3-0242ac130003")
var ProviderID = fmt.Sprintf("metal3://%s", Bmhuid)

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
			Namespace: namespaceName,
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
			Namespace: namespaceName,
		},
		MetaData: &corev1.SecretReference{
			Name:      "mym3machine-meta-data",
			Namespace: namespaceName,
		},
		NetworkData: &corev1.SecretReference{
			Name:      "mym3machine-network-data",
			Namespace: namespaceName,
		},
	}
}

func m3mSecretStatusNil() *capm3.Metal3MachineStatus {
	return &capm3.Metal3MachineStatus{}
}

func consumerRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name:       "mym3machine",
		Namespace:  namespaceName,
		Kind:       "M3Machine",
		APIVersion: capm3.GroupVersion.String(),
	}
}

func consumerRefSome() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name:       "someoneelsesmachine",
		Namespace:  namespaceName,
		Kind:       "M3Machine",
		APIVersion: clusterv1.GroupVersion.String(),
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
		Online: true,
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
		Online:      true,
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
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
		Labels: map[string]string{
			clusterv1.ClusterLabelName: clusterName,
		},
		Annotations: map[string]string{
			HostAnnotation: namespaceName + "/myhost",
		},
	}
}

func bmhObjectMetaWithValidCAPM3PausedAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "myhost",
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
		Labels: map[string]string{
			clusterv1.ClusterLabelName: clusterName,
		},
		Annotations: map[string]string{
			bmh.PausedAnnotation: PausedAnnotationKey,
		},
	}
}

func bmhObjectMetaWithValidEmptyPausedAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "myhost",
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
		Annotations: map[string]string{
			bmh.PausedAnnotation: "",
		},
	}
}

func bmhObjectMetaEmptyAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "myhost",
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
		Annotations:     map[string]string{},
	}
}

func bmhObjectMetaNoAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "myhost",
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
	}
}

func m3mObjectMetaWithInvalidAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "foobarm3machine",
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
		Annotations: map[string]string{
			HostAnnotation: namespaceName + "/wrongvalue",
		},
	}
}

func m3mObjectMetaWithSomeAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "m3machine",
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
		Annotations: map[string]string{
			HostAnnotation: namespaceName + "/somehost",
		},
	}
}

func m3mObjectMetaEmptyAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "m3machine",
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
		Annotations:     map[string]string{},
	}
}

func m3mObjectMetaNoAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            "m3machine",
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
	}
}

func machineOwnedByMachineSet() *clusterv1.Machine {
	return &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Machine",
		},
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "MachineSet",
					Name:       "ms-test1",
				},
			},
		},
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
				logr.Discard(),
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
				logr.Discard(),
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
				logr.Discard(),
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
		Machine    clusterv1.Machine
		ExpectTrue bool
	}

	DescribeTable("Test BootstrapReady",
		func(tc testCaseBootstrapReady) {
			machineMgr, err := NewMachineManager(nil, nil, nil, &tc.Machine, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			bootstrapState := machineMgr.IsBootstrapReady()

			Expect(bootstrapState).To(Equal(tc.ExpectTrue))
		},
		Entry("ready", testCaseBootstrapReady{
			Machine: clusterv1.Machine{
				Status: clusterv1.MachineStatus{
					BootstrapReady: true,
				},
			},
			ExpectTrue: true,
		}),
		Entry("not ready", testCaseBootstrapReady{
			Machine:    clusterv1.Machine{},
			ExpectTrue: false,
		}),
	)

	DescribeTable("Test setting and clearing errors",
		func(bmMachine capm3.Metal3Machine) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &bmMachine,
				logr.Discard(),
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

		// Creating the hosts
		host1 := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "host1",
				Namespace: namespaceName,
			},
			Spec: bmh.BareMetalHostSpec{
				ConsumerRef: &corev1.ObjectReference{
					Name:       "someothermachine",
					Namespace:  namespaceName,
					Kind:       "M3Machine",
					APIVersion: capm3.GroupVersion.String(),
				},
			},
		}
		host2 := *newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, "metadata", false)

		host3 := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "host3",
				Namespace: namespaceName,
			},
			Spec: bmh.BareMetalHostSpec{
				ConsumerRef: &corev1.ObjectReference{
					Name:       "machine1",
					Namespace:  namespaceName,
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
				Namespace: namespaceName,
			},
			Status: bmh.BareMetalHostStatus{
				ErrorMessage: "this host is discovered but not usable",
			},
		}
		hostWithLabel := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostWithLabel",
				Namespace: namespaceName,
				Labels:    map[string]string{"key1": "value1"},
			},
		}
		hostWithUnhealthyAnnotation := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "hostWithUnhealthyAnnotation",
				Namespace:   namespaceName,
				Annotations: map[string]string{capm3.UnhealthyAnnotation: "unhealthy"},
			},
		}
		hostWithNodeReuseLabelSetToKCP := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostWithNodeReuseLabelSetToKCP",
				Namespace: namespaceName,
				Labels:    map[string]string{nodeReuseLabelName: "kcp-pool1"},
			},
		}
		hostWithNodeReuseLabelSetToMD := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostWithNodeReuseLabelSetToMD",
				Namespace: namespaceName,
				Labels:    map[string]string{nodeReuseLabelName: "md-pool1"},
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
			Machine          *clusterv1.Machine
			Hosts            []client.Object
			M3Machine        *capm3.Metal3Machine
			ExpectedHostName string
		}

		DescribeTable("Test ChooseHost",
			func(tc testCaseChooseHost) {
				fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(tc.Hosts...).Build()
				machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
					tc.M3Machine, logr.Discard(),
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
				Hosts:            []client.Object{&host2, &host1},
				M3Machine:        m3mconfig2,
				ExpectedHostName: host2.Name,
			}),
			Entry("Pick hostWithNodeReuseLabelSetToKCP, which has a matching nodeReuseLabelName", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef2),
				Hosts:            []client.Object{&hostWithNodeReuseLabelSetToKCP, &host3, &host2},
				M3Machine:        m3mconfig2,
				ExpectedHostName: hostWithNodeReuseLabelSetToKCP.Name,
			}),
			Entry("Pick hostWithNodeReuseLabelSetToMD, which has a matching nodeReuseLabelName", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef2),
				Hosts:            []client.Object{&hostWithNodeReuseLabelSetToMD, &host3, &host2},
				M3Machine:        m3mconfig2,
				ExpectedHostName: hostWithNodeReuseLabelSetToMD.Name,
			}),
			Entry("Ignore discoveredHost and pick host2, which lacks a ConsumerRef",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef2),
					Hosts:            []client.Object{&discoveredHost, &host2, &host1},
					M3Machine:        m3mconfig2,
					ExpectedHostName: host2.Name,
				},
			),
			Entry("Ignore hostWithUnhealthyAnnotation and pick host2, which lacks a ConsumerRef",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef2),
					Hosts:            []client.Object{&hostWithUnhealthyAnnotation, &host1, &host2},
					M3Machine:        m3mconfig2,
					ExpectedHostName: host2.Name,
				},
			),
			Entry("Pick host3, which has a matching ConsumerRef", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef3),
				Hosts:            []client.Object{&host1, &host3, &host2},
				M3Machine:        m3mconfig3,
				ExpectedHostName: host3.Name,
			}),
			Entry("Two hosts already taken, third is in another namespace",
				testCaseChooseHost{
					Machine:          newMachine("machine2", "", infrastructureRef),
					Hosts:            []client.Object{&host1, &host3, &host4},
					M3Machine:        m3mconfig,
					ExpectedHostName: "",
				},
			),
			Entry("Choose hosts with a label, even without a label selector",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef),
					Hosts:            []client.Object{&hostWithLabel},
					M3Machine:        m3mconfig,
					ExpectedHostName: hostWithLabel.Name,
				},
			),
			Entry("Choose hosts with a nodeReuseLabelName set to KCP, even without a label selector",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef),
					Hosts:            []client.Object{&hostWithNodeReuseLabelSetToKCP},
					M3Machine:        m3mconfig,
					ExpectedHostName: hostWithNodeReuseLabelSetToKCP.Name,
				},
			),
			Entry("Choose the host with the right label", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef2),
				Hosts:            []client.Object{&hostWithLabel, &host2},
				M3Machine:        m3mconfig2,
				ExpectedHostName: hostWithLabel.Name,
			}),
			Entry("No host that matches required label", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef3),
				Hosts:            []client.Object{&host2, &hostWithLabel},
				M3Machine:        m3mconfig3,
				ExpectedHostName: "",
			}),
			Entry("Host that matches a matchExpression", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef4),
				Hosts:            []client.Object{&host2, &hostWithLabel},
				M3Machine:        m3mconfig4,
				ExpectedHostName: hostWithLabel.Name,
			}),
			Entry("No Host available that matches a matchExpression",
				testCaseChooseHost{
					Machine:          newMachine("machine1", "", infrastructureRef4),
					Hosts:            []client.Object{&host2},
					M3Machine:        m3mconfig4,
					ExpectedHostName: "",
				},
			),
			Entry("No host chosen, invalid match expression", testCaseChooseHost{
				Machine:          newMachine("machine1", "", infrastructureRef5),
				Hosts:            []client.Object{&host2, &hostWithLabel, &host1},
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
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(
				tc.Host,
				tc.M3Machine).Build()

			machineMgr, err := NewMachineManager(fakeClient, nil, nil, nil, tc.M3Machine, logr.Discard())
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.SetPauseAnnotation(context.TODO())
			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			savedHost := bmh.BareMetalHost{}
			err = fakeClient.Get(context.TODO(),
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
						Namespace:  namespaceName,
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
						Namespace:  namespaceName,
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
						Namespace:  namespaceName,
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
		Cluster       *clusterv1.Cluster
		M3Machine     *capm3.Metal3Machine
		Host          *bmh.BareMetalHost
		ExpectPresent bool
		ExpectError   bool
	}

	DescribeTable("Test Remove BMH Pause Annotation",
		func(tc testCaseRemovePauseAnnotation) {
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(
				tc.Host,
				tc.M3Machine,
				tc.Cluster).Build()
			machineMgr, err := NewMachineManager(fakeClient,
				tc.Cluster,
				nil,
				nil,
				tc.M3Machine,
				logr.Discard())

			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.RemovePauseAnnotation(context.TODO())
			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			savedHost := bmh.BareMetalHost{}
			err = fakeClient.Get(context.TODO(),
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
						Namespace:  namespaceName,
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
						Namespace:  namespaceName,
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
						Namespace:  namespaceName,
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
		UserDataNamespace           string
		ExpectedUserDataNamespace   string
		Host                        *bmh.BareMetalHost
		ExpectedImage               *bmh.Image
		ExpectUserData              bool
		expectNodeReuseLabelDeleted bool
	}

	DescribeTable("Test SetHostSpec",
		func(tc testCaseSetHostSpec) {
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(tc.Host).Build()

			m3mconfig, infrastructureRef := newConfig(tc.UserDataNamespace,
				map[string]string{}, []capm3.HostSelectorRequirement{},
			)
			machine := newMachine("machine1", "", infrastructureRef)

			machineMgr, err := NewMachineManager(fakeClient, nil, nil, machine, m3mconfig,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.setHostSpec(context.TODO(), tc.Host)
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
				nil, false, "metadata", false,
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("User data has no namespace", testCaseSetHostSpec{
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: namespaceName,
			Host: newBareMetalHost("host2", nil, bmh.StateNone,
				nil, false, "metadata", false,
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("Externally provisioned, same machine", testCaseSetHostSpec{
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: namespaceName,
			Host: newBareMetalHost("host2", nil, bmh.StateNone,
				nil, false, "metadata", false,
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("Previously provisioned, different image",
			testCaseSetHostSpec{
				UserDataNamespace:         "",
				ExpectedUserDataNamespace: namespaceName,
				Host: newBareMetalHost("host2", bmhSpecTestImg(),
					bmh.StateNone, nil, false, "metadata", false,
				),
				ExpectedImage:  expectedImgTest(),
				ExpectUserData: false,
			},
		),
	)

	DescribeTable("Test SetHostConsumerRef",
		func(tc testCaseSetHostSpec) {
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(tc.Host).Build()

			m3mconfig, infrastructureRef := newConfig(tc.UserDataNamespace,
				map[string]string{}, []capm3.HostSelectorRequirement{},
			)
			machine := newMachine("machine1", "", infrastructureRef)

			machineMgr, err := NewMachineManager(fakeClient, nil, nil, machine, m3mconfig,
				logr.Discard(),
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

			if tc.expectNodeReuseLabelDeleted {
				Expect(tc.Host.Labels[nodeReuseLabelName]).To(Equal(""))
			}
		},
		Entry("User data has explicit alternate namespace", testCaseSetHostSpec{
			UserDataNamespace:         "otherns",
			ExpectedUserDataNamespace: "otherns",
			Host: newBareMetalHost("host2", nil, bmh.StateNone,
				nil, false, "metadata", false,
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("User data has no namespace", testCaseSetHostSpec{
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: namespaceName,
			Host: newBareMetalHost("host2", nil, bmh.StateNone,
				nil, false, "metadata", false,
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("Externally provisioned, same machine", testCaseSetHostSpec{
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: namespaceName,
			Host: newBareMetalHost("host2", nil, bmh.StateNone,
				nil, false, "metadata", false,
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("Previously provisioned, different image",
			testCaseSetHostSpec{
				UserDataNamespace:         "",
				ExpectedUserDataNamespace: namespaceName,
				Host: newBareMetalHost("host2", bmhSpecTestImg(),
					bmh.StateNone, nil, false, "metadata", false,
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
				Namespace: namespaceName,
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(&host).Build()

		type testCaseExists struct {
			Machine   *clusterv1.Machine
			M3Machine *capm3.Metal3Machine
			Expected  bool
		}

		DescribeTable("Test Exists function",
			func(tc testCaseExists) {
				machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
					tc.M3Machine, logr.Discard(),
				)
				Expect(err).NotTo(HaveOccurred())

				result, err := machineMgr.exists(context.TODO())
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(tc.Expected))
			},
			Entry("Failed to find the existing host", testCaseExists{
				Machine: &clusterv1.Machine{},
				M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
					m3mObjectMetaWithSomeAnnotations(),
				),
				Expected: true,
			}),
			Entry("Found host even though annotation value is incorrect",
				testCaseExists{
					Machine: &clusterv1.Machine{},
					M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
						m3mObjectMetaWithInvalidAnnotations(),
					),
					Expected: false,
				},
			),
			Entry("Found host even though annotation not present", testCaseExists{
				Machine: &clusterv1.Machine{},
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
				Namespace: namespaceName,
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(&host).Build()

		type testCaseGetHost struct {
			Machine       *clusterv1.Machine
			M3Machine     *capm3.Metal3Machine
			ExpectPresent bool
		}

		DescribeTable("Test GetHost",
			func(tc testCaseGetHost) {
				machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
					tc.M3Machine, logr.Discard(),
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
				Machine: &clusterv1.Machine{},
				M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
					m3mObjectMetaWithValidAnnotations(),
				),
				ExpectPresent: true,
			}),
			Entry("Should not find the host, annotation value incorrect",
				testCaseGetHost{
					Machine: &clusterv1.Machine{},
					M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
						m3mObjectMetaWithInvalidAnnotations(),
					),
					ExpectPresent: false,
				},
			),
			Entry("Should not find the host, annotation not present", testCaseGetHost{
				Machine: &clusterv1.Machine{},
				M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
					m3mObjectMetaEmptyAnnotations(),
				),
				ExpectPresent: false,
			}),
		)
	})

	type testCaseGetSetProviderID struct {
		Machine       *clusterv1.Machine
		M3Machine     *capm3.Metal3Machine
		Host          *bmh.BareMetalHost
		ExpectPresent bool
		ExpectError   bool
	}

	DescribeTable("Test Get and Set Provider ID",
		func(tc testCaseGetSetProviderID) {
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(tc.Host).Build()
			machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
				tc.M3Machine, logr.Discard(),
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

			providerID := fmt.Sprintf("%s%s", ProviderIDPrefix, *bmhID)
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
					Namespace: namespaceName,
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
					Namespace: namespaceName,
					UID:       Bmhuid,
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
					Namespace: namespaceName,
					UID:       Bmhuid,
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
			Machine        *clusterv1.Machine
			M3Machine      *capm3.Metal3Machine
			ExpectCtrlNode bool
		}

		host := bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myhost",
				Namespace: namespaceName,
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(&host).Build()

		DescribeTable("Test small functions",
			func(tc testCaseSmallFunctions) {
				machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
					tc.M3Machine, logr.Discard(),
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
				Machine: &clusterv1.Machine{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Machine",
						APIVersion: clusterv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mymachine",
						Namespace: namespaceName,
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabelName: "labelHere",
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
		Machine          clusterv1.Machine
		Host             *bmh.BareMetalHost
		M3Machine        *capm3.Metal3Machine
		ExpectAnnotation bool
	}

	DescribeTable("Test EnsureAnnotation",
		func(tc testCaseEnsureAnnotation) {
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(tc.M3Machine).Build()
			machineMgr, err := NewMachineManager(fakeClient, nil, nil, &tc.Machine,
				tc.M3Machine, logr.Discard(),
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
			Machine: clusterv1.Machine{},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
				m3mObjectMetaWithValidAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone, nil,
				false, "metadata", false,
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotation exists but is wrong", testCaseEnsureAnnotation{
			Machine: clusterv1.Machine{},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
				m3mObjectMetaWithInvalidAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone,
				nil, false, "metadata", false,
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotations are empty", testCaseEnsureAnnotation{
			Machine: clusterv1.Machine{},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
				m3mObjectMetaEmptyAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone,
				nil, false, "metadata", false,
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotations are nil", testCaseEnsureAnnotation{
			Machine: clusterv1.Machine{},
			M3Machine: newMetal3Machine("", nil, nil, nil,
				m3mObjectMetaNoAnnotations(),
			),
			Host: newBareMetalHost("myhost", nil, bmh.StateNone,
				nil, false, "metadata", false,
			),
			ExpectAnnotation: true,
		}),
	)

	type testCaseDelete struct {
		Host                            *bmh.BareMetalHost
		Secret                          *corev1.Secret
		Machine                         *clusterv1.Machine
		M3Machine                       *capm3.Metal3Machine
		BMCSecret                       *corev1.Secret
		ExpectedConsumerRef             *corev1.ObjectReference
		ExpectedResult                  error
		ExpectSecretDeleted             bool
		ExpectClusterLabelDeleted       bool
		ExpectedPausedAnnotationDeleted bool
		NodeReuseEnabled                bool
		MachineIsControlPlane           bool
		MachineIsNotControlPlane        bool
		ExpectedBMHOnlineStatus         bool
		capm3fasttrack                  string
	}

	DescribeTable("Test Delete function",
		func(tc testCaseDelete) {
			objects := []client.Object{tc.M3Machine}
			if tc.Host != nil {
				objects = append(objects, tc.Host)
			}
			if tc.Secret != nil {
				objects = append(objects, tc.Secret)
			}
			if tc.BMCSecret != nil {
				objects = append(objects, tc.BMCSecret)
			}

			Capm3FastTrack = tc.capm3fasttrack

			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()

			machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
				tc.M3Machine, logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.Delete(context.TODO())

			if tc.ExpectedResult == nil {
				Expect(err).NotTo(HaveOccurred())
			} else {
				ok := errors.As(err, &requeueAfterError)

				Expect(ok).To(BeTrue())
				Expect(requeueAfterError.Error()).To(Equal(tc.ExpectedResult.Error()))
			}

			if tc.Host != nil {
				key := client.ObjectKey{
					Name:      tc.Host.Name,
					Namespace: tc.Host.Namespace,
				}
				host := bmh.BareMetalHost{}

				err := fakeClient.Get(context.TODO(), key, &host)
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
				err = fakeClient.Get(context.TODO(), key, &tmpBootstrapSecret)
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
				err = fakeClient.Get(context.TODO(),
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
				err = fakeClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.Host.Name,
						Namespace: tc.Host.Namespace,
					},
					&savedHost,
				)
				Expect(err).NotTo(HaveOccurred())
				// get the BMC credential
				savedCred := corev1.Secret{}
				err = fakeClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      savedHost.Spec.BMC.CredentialsName,
						Namespace: savedHost.Namespace,
					},
					&savedCred,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedHost.Labels[clusterv1.ClusterLabelName]).To(Equal(""))
				Expect(savedCred.Labels[clusterv1.ClusterLabelName]).To(Equal(""))
				// Other labels are not removed
				Expect(savedHost.Labels["foo"]).To(Equal("bar"))
				Expect(savedCred.Labels["foo"]).To(Equal("bar"))
			}
			if tc.NodeReuseEnabled {
				m3mTemplate := capm3.Metal3MachineTemplate{}
				err = fakeClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.M3Machine.ObjectMeta.GetAnnotations()[clusterv1.TemplateClonedFromNameAnnotation],
						Namespace: tc.M3Machine.Namespace,
					},
					&m3mTemplate,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(m3mTemplate.Spec.NodeReuse).To(BeTrue())
			}

			if tc.Host != nil {
				savedbmh := bmh.BareMetalHost{}
				err = fakeClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.Host.Name,
						Namespace: tc.Host.Namespace,
					},
					&savedbmh,
				)
				Expect(err).NotTo(HaveOccurred())
				if tc.capm3fasttrack == "" {
					Expect(Capm3FastTrack).To(Equal("false"))
				}
				Expect(savedbmh.Spec.Online).To(Equal(tc.ExpectedBMHOnlineStatus))
			}
		},
		Entry("Deprovisioning needed", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpec(),
				bmh.StateProvisioned, bmhStatus(), false, "metadata", true,
			),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef:     consumerRef(),
			ExpectedResult:          &RequeueAfterError{},
			Secret:                  newSecret(),
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("No Host status, deprovisioning needed", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpec(), bmh.StateNone,
				nil, false, "metadata", true,
			),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef:     consumerRef(),
			ExpectedResult:          &RequeueAfterError{},
			Secret:                  newSecret(),
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("No Host status, no deprovisioning needed", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecNoImg(), bmh.StateNone, nil,
				false, "metadata", true,
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
				bmh.StateDeprovisioning, bmhStatus(), false, "metadata", true,
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
				bmh.StateExternallyProvisioned, bmhPowerStatus(), true, "metadata", true,
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
					bmh.StateExternallyProvisioned, bmhPowerStatus(), false, "metadata", true,
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
					bmh.StateUnmanaged, bmhPowerStatus(), false, "metadata", true,
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
				bmhStatus(), false, "metadata", true,
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
				bmhStatus(), false, "metadata", true,
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
				bmhStatus(), false, "metadata", true,
			),
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns2",
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
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
					bmh.StateProvisioned, bmhStatus(), false, "metadata", true,
				),
				Machine: newMachine("", "", nil),
				M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
					m3mObjectMetaWithValidAnnotations(),
				),
				ExpectedConsumerRef:     consumerRefSome(),
				Secret:                  newSecret(),
				ExpectedBMHOnlineStatus: true,
			},
		),
		Entry("No consumer ref, so this is a no-op", testCaseDelete{
			Host:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, "metadata", true),
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
				false, "metadata", true,
			),
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
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
			Host:                      newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, "metadata", true),
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
			Host:                      newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, "metadata", false),
			BMCSecret:                 newBMCSecret("mycredentials", false),
			ExpectSecretDeleted:       true,
			ExpectClusterLabelDeleted: false,
		}),
		Entry("BMH MetaData, NetworkData and UserData should not be cleaned on deprovisioning", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpecSomeImg(),
				bmh.StateProvisioned, bmhStatus(), false, "metadata", true,
			),
			Machine: newMachine("mymachine", "mym3machine", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatusNil(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:                  newSecret(),
			ExpectedConsumerRef:     consumerRefSome(),
			capm3fasttrack:          "false",
			ExpectedBMHOnlineStatus: true,
		}),
		Entry("Capm3FastTrack is set to false, AutomatedCleaning mode is set to metadata, set bmh online field to false", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpec(),
				bmh.StateDeprovisioning, bmhStatus(), false, "metadata", true),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          &RequeueAfterError{},
			ExpectedConsumerRef:     consumerRef(),
			Secret:                  newSecret(),
			capm3fasttrack:          "false",
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("Capm3FastTrack is set to true, AutomatedCleaning mode is set to metadata, set bmh online field to true", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpec(),
				bmh.StateDeprovisioning, bmhStatus(), false, "metadata", true),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          &RequeueAfterError{},
			ExpectedConsumerRef:     consumerRef(),
			Secret:                  newSecret(),
			capm3fasttrack:          "true",
			ExpectedBMHOnlineStatus: true,
		}),
		Entry("Capm3FastTrack is set to false, AutomatedCleaning mode is set to disabled, set bmh online field to false", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpec(),
				bmh.StateDeprovisioning, bmhStatus(), false, "disabled", true),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          &RequeueAfterError{},
			ExpectedConsumerRef:     consumerRef(),
			Secret:                  newSecret(),
			capm3fasttrack:          "false",
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("Capm3FastTrack is set to true, AutomatedCleaning mode is set to disabled, set bmh online field to false", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpec(),
				bmh.StateDeprovisioning, bmhStatus(), false, "disabled", true),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          &RequeueAfterError{},
			ExpectedConsumerRef:     consumerRef(),
			capm3fasttrack:          "true",
			Secret:                  newSecret(),
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("Capm3FastTrack is empty, AutomatedCleaning mode is set to disabled, set bmh online field to false", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpec(),
				bmh.StateDeprovisioning, bmhStatus(), false, "disabled", true),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          &RequeueAfterError{},
			ExpectedConsumerRef:     consumerRef(),
			capm3fasttrack:          "",
			Secret:                  newSecret(),
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("Capm3FastTrack is empty, AutomatedCleaning mode is set to metadata, set bmh online field to false", testCaseDelete{
			Host: newBareMetalHost("myhost", bmhSpec(),
				bmh.StateDeprovisioning, bmhStatus(), false, "metadata", true),
			Machine: newMachine("mymachine", "", nil),
			M3Machine: newMetal3Machine("mym3machine", nil, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          &RequeueAfterError{},
			ExpectedConsumerRef:     consumerRef(),
			capm3fasttrack:          "",
			Secret:                  newSecret(),
			ExpectedBMHOnlineStatus: false,
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
			Machine         *clusterv1.Machine
			ExpectedMachine clusterv1.Machine
			M3Machine       capm3.Metal3Machine
		}

		DescribeTable("Test UpdateMachineStatus",
			func(tc testCaseUpdateMachineStatus) {
				fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(&tc.M3Machine).Build()

				machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
					&tc.M3Machine, logr.Discard(),
				)
				Expect(err).NotTo(HaveOccurred())

				err = machineMgr.updateMachineStatus(context.TODO(), tc.Host)
				Expect(err).NotTo(HaveOccurred())

				key := client.ObjectKey{
					Name:      tc.M3Machine.ObjectMeta.Name,
					Namespace: tc.M3Machine.ObjectMeta.Namespace,
				}
				m3machine := capm3.Metal3Machine{}
				err = fakeClient.Get(context.TODO(), key, &m3machine)
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
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mymachine",
						Namespace: namespaceName,
					},
					Status: clusterv1.MachineStatus{
						Addresses: []clusterv1.MachineAddress{
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
						Namespace: namespaceName,
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "M3Machine",
						APIVersion: clusterv1.GroupVersion.String(),
					},
					Status: capm3.Metal3MachineStatus{
						Addresses: []clusterv1.MachineAddress{
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
				ExpectedMachine: clusterv1.Machine{
					Status: clusterv1.MachineStatus{
						Addresses: []clusterv1.MachineAddress{
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
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mymachine",
						Namespace: namespaceName,
					},
					Status: clusterv1.MachineStatus{
						Addresses: []clusterv1.MachineAddress{
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
						Namespace: namespaceName,
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "M3Machine",
						APIVersion: clusterv1.GroupVersion.String(),
					},
					Status: capm3.Metal3MachineStatus{
						Addresses: []clusterv1.MachineAddress{
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
				ExpectedMachine: clusterv1.Machine{
					Status: clusterv1.MachineStatus{
						Addresses: []clusterv1.MachineAddress{
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
					Machine: &clusterv1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "mymachine",
							Namespace: namespaceName,
						},
						Status: clusterv1.MachineStatus{},
					},
					M3Machine: capm3.Metal3Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "mym3machine",
							Namespace: namespaceName,
						},
						TypeMeta: metav1.TypeMeta{
							Kind:       "M3Machine",
							APIVersion: clusterv1.GroupVersion.String(),
						},
						Status: capm3.Metal3MachineStatus{
							Addresses: []clusterv1.MachineAddress{},
							Ready:     true,
						},
					},
					ExpectedMachine: clusterv1.Machine{
						Status: clusterv1.MachineStatus{},
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

		addr1 := clusterv1.MachineAddress{
			Type:    clusterv1.MachineInternalIP,
			Address: "192.168.1.1",
		}

		addr2 := clusterv1.MachineAddress{
			Type:    clusterv1.MachineInternalIP,
			Address: "172.0.20.2",
		}

		addr3 := clusterv1.MachineAddress{
			Type:    clusterv1.MachineHostName,
			Address: "mygreathost",
		}

		type testCaseNodeAddress struct {
			Machine               clusterv1.Machine
			M3Machine             capm3.Metal3Machine
			Host                  *bmh.BareMetalHost
			ExpectedNodeAddresses []clusterv1.MachineAddress
		}

		DescribeTable("Test NodeAddress",
			func(tc testCaseNodeAddress) {
				var nodeAddresses []clusterv1.MachineAddress

				fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).Build()
				machineMgr, err := NewMachineManager(fakeClient, nil, nil, &tc.Machine,
					&tc.M3Machine, logr.Discard(),
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
				ExpectedNodeAddresses: []clusterv1.MachineAddress{addr1},
			}),
			Entry("Two NICs", testCaseNodeAddress{
				Host: &bmh.BareMetalHost{
					Status: bmh.BareMetalHostStatus{
						HardwareDetails: &bmh.HardwareDetails{
							NIC: []bmh.NIC{nic1, nic2},
						},
					},
				},
				ExpectedNodeAddresses: []clusterv1.MachineAddress{addr1, addr2},
			}),
			Entry("Hostname is set", testCaseNodeAddress{
				Host: &bmh.BareMetalHost{
					Status: bmh.BareMetalHostStatus{
						HardwareDetails: &bmh.HardwareDetails{
							Hostname: "mygreathost",
						},
					},
				},
				ExpectedNodeAddresses: []clusterv1.MachineAddress{addr3},
			}),
			Entry("Empty Hostname", testCaseNodeAddress{
				Host: &bmh.BareMetalHost{
					Status: bmh.BareMetalHostStatus{
						HardwareDetails: &bmh.HardwareDetails{
							Hostname: "",
						},
					},
				},
				ExpectedNodeAddresses: []clusterv1.MachineAddress{},
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

			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &m3m, logr.Discard())
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
			providerID:    pointer.StringPtr(ProviderID),
			expectedBMHID: string(Bmhuid),
		}),
	)

	Describe("Test SetNodeProviderID", func() {
		s := runtime.NewScheme()
		err := clusterv1.AddToScheme(s)
		if err != nil {
			log.Printf("AddToScheme failed: %v", err)
		}
		err = bmh.AddToScheme(s)
		if err != nil {
			log.Printf("AddToScheme failed: %v", err)
		}

		type testCaseSetNodePoviderID struct {
			Node               corev1.Node
			HostID             string
			ExpectedError      bool
			ExpectedProviderID string
		}

		DescribeTable("Test SetNodeProviderID",
			func(tc testCaseSetNodePoviderID) {
				fakeClient := fake.NewClientBuilder().WithScheme(s).Build()
				corev1Client := clientfake.NewSimpleClientset(&tc.Node).CoreV1()
				m := func(ctx context.Context, client client.Client, cluster *clusterv1.Cluster) (
					clientcorev1.CoreV1Interface, error,
				) {
					return corev1Client, nil
				}

				machineMgr, err := NewMachineManager(fakeClient, newCluster(clusterName),
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef,
						&capm3.Metal3ClusterSpec{NoCloudProvider: true}, nil,
					),
					&clusterv1.Machine{}, &capm3.Metal3Machine{}, logr.Discard(),
				)
				Expect(err).NotTo(HaveOccurred())

				err = machineMgr.SetNodeProviderID(context.TODO(), tc.HostID,
					tc.ExpectedProviderID, m,
				)

				if tc.ExpectedError {
					Expect(err).To(HaveOccurred())
					return
				}
				Expect(err).NotTo(HaveOccurred())

				ctx := context.Background()
				// get the node
				node, err := corev1Client.Nodes().Get(ctx, tc.Node.Name,
					metav1.GetOptions{},
				)
				Expect(err).NotTo(HaveOccurred())

				Expect(node.Spec.ProviderID).To(Equal(tc.ExpectedProviderID))
			},
			Entry("Set target ProviderID, No matching node", testCaseSetNodePoviderID{
				Node:               corev1.Node{},
				HostID:             string(Bmhuid),
				ExpectedError:      true,
				ExpectedProviderID: ProviderID,
			}),
			Entry("Set target ProviderID, matching node", testCaseSetNodePoviderID{
				Node: corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							ProviderLabelPrefix: string(bmhuid),
						},
					},
				},
				HostID:             string(bmhuid),
				ExpectedError:      false,
				ExpectedProviderID: fmt.Sprintf("%s%s", ProviderIDPrefix, string(bmhuid)),
			}),
			Entry("Set target ProviderID, providerID set", testCaseSetNodePoviderID{
				Node: corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							ProviderLabelPrefix: string(Bmhuid),
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: ProviderID,
					},
				},
				HostID:             string(Bmhuid),
				ExpectedError:      false,
				ExpectedProviderID: ProviderID,
			}),
		)
	})

	type testCaseGetUserDataSecretName struct {
		Machine     *clusterv1.Machine
		M3Machine   *capm3.Metal3Machine
		BMHost      *bmh.BareMetalHost
		Secret      *corev1.Secret
		ExpectError bool
	}

	DescribeTable("Test getUserDataSecretName function",
		func(tc testCaseGetUserDataSecretName) {
			objects := []client.Object{
				tc.M3Machine,
				tc.Machine,
			}
			if tc.Secret != nil {
				objects = append(objects, tc.Secret)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()

			machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
				tc.M3Machine, logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.getUserDataSecretName(context.TODO(), tc.BMHost)
			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())

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

			// if we had to create an additional secret (dataSecretName not set)
			if tc.Machine.Spec.Bootstrap.DataSecretName == nil {

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
				err = fakeClient.Get(context.TODO(), key, &tmpBootstrapSecret)
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
		Entry("Secret set in Machine", testCaseGetUserDataSecretName{
			Secret: &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Foobar",
					Namespace: namespaceName,
				},
				Data: map[string][]byte{
					"value": []byte("FooBar\n"),
				},
				Type: "Opaque",
			},
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: pointer.StringPtr("Foobar"),
					},
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, "metadata", false),
		}),
		Entry("Secret set in Machine, different namespace", testCaseGetUserDataSecretName{
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
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns2",
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: pointer.StringPtr("Foobar"),
					},
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, "metadata", false),
		}),
		Entry("Secret in other namespace set in Machine", testCaseGetUserDataSecretName{
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
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns2",
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							Name:      "abc",
							Namespace: "def",
						},
						DataSecretName: pointer.StringPtr("Foobar"),
					},
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, "metadata", false),
		}),
		Entry("UserDataSecretName set in Machine, secret exists", testCaseGetUserDataSecretName{
			Secret: newSecret(),
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: pointer.StringPtr("test-data-secret-name"),
					},
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, "metadata", false),
		}),
		Entry("UserDataSecretName set in Machine, no secret", testCaseGetUserDataSecretName{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: pointer.StringPtr("test-data-secret-name"),
					},
				},
			},
			M3Machine: newMetal3Machine("mym3machine", nil, nil, nil, nil),
			BMHost:    newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, "metadata", false),
		}),
	)

	type testCaseAssociate struct {
		Machine            *clusterv1.Machine
		Host               *bmh.BareMetalHost
		M3Machine          *capm3.Metal3Machine
		BMCSecret          *corev1.Secret
		DataTemplate       *capm3.Metal3DataTemplate
		Data               *capm3.Metal3Data
		ExpectRequeue      bool
		ExpectClusterLabel bool
		ExpectOwnerRef     bool
	}

	DescribeTable("Test Associate function",
		func(tc testCaseAssociate) {
			objects := []client.Object{
				tc.M3Machine,
				tc.Machine,
			}
			if tc.Host != nil {
				objects = append(objects, tc.Host)
			}
			if tc.DataTemplate != nil {
				objects = append(objects, tc.DataTemplate)
			}
			if tc.Data != nil {
				objects = append(objects, tc.Data)
			}
			if tc.BMCSecret != nil {
				objects = append(objects, tc.BMCSecret)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()

			machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
				tc.M3Machine, logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.Associate(context.TODO())
			if tc.ExpectRequeue {
				ok := errors.As(err, &hasRequeueAfterError)
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
			err = fakeClient.Get(context.TODO(),
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
				err = fakeClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      savedHost.Spec.BMC.CredentialsName,
						Namespace: savedHost.Namespace,
					},
					&savedCred,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedHost.Labels[clusterv1.ClusterLabelName]).To(Equal(tc.Machine.Spec.ClusterName))
				Expect(savedCred.Labels[clusterv1.ClusterLabelName]).To(Equal(tc.Machine.Spec.ClusterName))
			}
		},
		Entry("Associate empty machine, Metal3 machine spec nil",
			testCaseAssociate{
				Machine: newMachine("", "", nil),
				M3Machine: newMetal3Machine("mym3machine", nil, nil, nil,
					m3mObjectMetaWithValidAnnotations(),
				),
				Host: newBareMetalHost("myhost", nil, bmh.StateNone, nil,
					false, "metadata", false,
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
					false, "metadata", false,
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
				Host:           newBareMetalHost("", nil, bmh.StateNone, nil, false, "metadata", false),
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
				Host:               newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, "metadata", false),
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
							Namespace: namespaceName,
						},
						UserData: &corev1.SecretReference{
							Name:      "mym3machine-user-data",
							Namespace: namespaceName,
						},
						Image: capm3.Image{
							URL:        testImageURL,
							Checksum:   testImageChecksumURL,
							DiskFormat: testImageDiskFormat,
						},
					}, nil, nil,
				),
				Host:               newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, "metadata", false),
				BMCSecret:          newBMCSecret("mycredentials", false),
				ExpectClusterLabel: true,
				ExpectRequeue:      true,
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
							Namespace: namespaceName,
						},
						UserData: &corev1.SecretReference{
							Name:      "mym3machine-user-data",
							Namespace: namespaceName,
						},
						Image: capm3.Image{
							URL:        testImageURL,
							Checksum:   testImageChecksumURL,
							DiskFormat: testImageDiskFormat,
						},
					}, &capm3.Metal3MachineStatus{
						RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: namespaceName},
					}, nil,
				),
				Host:      newBareMetalHost("myhost", bmhSpecBMC(), bmh.StateNone, nil, false, "metadata", false),
				BMCSecret: newBMCSecret("mycredentials", false),
				Data: &capm3.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcd-0",
						Namespace: namespaceName,
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
				ExpectClusterLabel: true,
				ExpectRequeue:      false,
				ExpectOwnerRef:     true,
			},
		),
	)

	type testCaseUpdate struct {
		Machine     *clusterv1.Machine
		Host        *bmh.BareMetalHost
		M3Machine   *capm3.Metal3Machine
		ExpectError bool
	}

	DescribeTable("Test Update function",
		func(tc testCaseUpdate) {
			objects := []client.Object{
				tc.Host,
				tc.M3Machine,
				tc.Machine,
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()

			machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
				tc.M3Machine, logr.Discard(),
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
			Host: newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, "metadata", false),
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
			Host:        newBareMetalHost("myhost", nil, bmh.StateNone, nil, false, "metadata", false),
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
				logr.Discard(),
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
				logr.Discard(),
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
				logr.Discard(),
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
		Machine            *clusterv1.Machine
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
			objects := []client.Object{}
			if tc.DataClaim != nil {
				objects = append(objects, tc.DataClaim)
			}
			fakeCleint := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			machineMgr, err := NewMachineManager(fakeCleint, nil, nil, tc.Machine, tc.M3Machine,
				logr.Discard(),
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
				err = fakeCleint.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.M3Machine.Name,
						Namespace: tc.M3Machine.Namespace,
					},
					&dataTemplate,
				)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Should return nil if No Spec available", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, nil, nil),
		}),
		Entry("MetaData and NetworkData should be set in spec", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				MetaData:    &corev1.SecretReference{Name: "abcd"},
				NetworkData: &corev1.SecretReference{Name: "defg"},
			}, nil, nil),
		}),
		Entry("RenderedData should be set in status", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd"},
			}, nil),
		}),
		Entry("Should expect DataClaim if it does not exist yet", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine:     newMachine("myName", "myName", nil),
			expectClaim: true,
		}),
		Entry("Should not be an error if DataClaim exists", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
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
		Entry("Should not be an error if DataClaim is empty", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine:     newMachine("myName", "myName", nil),
			DataClaim:   &capm3.Metal3DataClaim{},
			expectClaim: true,
		}),
		Entry("Should expect claim if DataClaim ready", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
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
						Namespace: namespaceName,
					},
				},
			},
			expectClaim: true,
		}),
	)

	DescribeTable("Test WaitForM3MetaData",
		func(tc testCaseM3MetaData) {
			objects := []client.Object{}
			if tc.DataClaim != nil {
				objects = append(objects, tc.DataClaim)
			}
			if tc.Data != nil {
				objects = append(objects, tc.Data)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine, tc.M3Machine,
				logr.Discard(),
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
					Namespace: namespaceName,
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
		Entry("Should return nil if Data Spec is empty", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, nil, nil),
			Machine:   nil,
		}),
		Entry("Should requeue if there is no Data template", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine:       newMachine("myName", "myName", nil),
			ExpectRequeue: true,
		}),
		Entry("Should requeue if Data claim without status", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
				},
			},
			ExpectRequeue: true,
		}),
		Entry("Should requeue if Data claim with empty status", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
				},
				Status: capm3.Metal3DataClaimStatus{
					RenderedData: &corev1.ObjectReference{},
				},
			},
			ExpectRequeue: true,
		}),
		Entry("Should requeue if Data does not exist", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataClaimSpec{
					Template: corev1.ObjectReference{
						Name:      "abcd",
						Namespace: namespaceName,
					},
				},
				Status: capm3.Metal3DataClaimStatus{
					RenderedData: &corev1.ObjectReference{
						Name:      "abcd-0",
						Namespace: namespaceName,
					},
				},
			},
			ExpectRequeue:    true,
			ExpectDataStatus: true,
		}),
		Entry("Should requeue if M3M status set but Data does not exist", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: namespaceName},
			}, nil),
			Machine:          newMachine("myName", "myName", nil),
			ExpectRequeue:    true,
			ExpectDataStatus: true,
		}),
		Entry("Should requeue if Data is not ready", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: namespaceName},
			}, nil),
			Machine: newMachine("myName", "myName", nil),
			Data: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abcd-0",
					Namespace: namespaceName,
				},
			},
			ExpectRequeue:    true,
			ExpectDataStatus: true,
		}),
		Entry("Should not error if Data is ready but no secrets", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: namespaceName},
			}, nil),
			Machine: newMachine("myName", "myName", nil),
			Data: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abcd-0",
					Namespace: namespaceName,
				},
				Status: capm3.Metal3DataStatus{
					Ready: true,
				},
			},
			ExpectDataStatus:   true,
			ExpectSecretStatus: true,
		}),
		Entry("Should not error if Data is ready with secrets", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: namespaceName},
			}, nil),
			Machine: newMachine("myName", "myName", nil),
			Data: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abcd-0",
					Namespace: namespaceName,
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
			objects := []client.Object{}
			if tc.DataClaim != nil {
				objects = append(objects, tc.DataClaim)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine, tc.M3Machine,
				logr.Discard(),
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
			err = fakeClient.Get(context.TODO(),
				client.ObjectKey{
					Name:      tc.M3Machine.Name,
					Namespace: tc.M3Machine.Namespace,
				},
				&dataTemplate,
			)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		},
		Entry("Should return nil if No Spec", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, nil, &capm3.Metal3MachineStatus{
				MetaData:    &corev1.SecretReference{Name: "abcd"},
				NetworkData: &corev1.SecretReference{Name: "defg"},
			}, nil),
			Machine: newMachine("myName", "myName", nil),
		}),
		Entry("MetaData and NetworkData should be set in spec and status", testCaseM3MetaData{
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
		Entry("Should return nil if DataClaim not found", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name:      "abc",
					Namespace: namespaceName,
				},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
		}),
		Entry("Should not error if DataClaim found", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name:      "abcd",
					Namespace: namespaceName,
				},
			}, nil, nil),
			Machine: newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
				},
			},
		}),
		Entry("Should return nil if DataClaim is empty", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &capm3.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name:      "abc",
					Namespace: namespaceName,
				},
			}, nil, nil),
			Machine:   newMachine("myName", "myName", nil),
			DataClaim: &capm3.Metal3DataClaim{},
		}),
	)

	type testCaseNodeReuseLabelMatches struct {
		Machine                  *clusterv1.Machine
		Host                     *bmh.BareMetalHost
		expectNodeReuseLabelName string
		expectNodeReuseLabel     bool
		expectMatch              bool
	}
	DescribeTable("Test NodeReuseLabelMatches",
		func(tc testCaseNodeReuseLabelMatches) {
			objects := []client.Object{}
			if tc.Machine != nil {
				objects = append(objects, tc.Machine)
			}
			if tc.Host != nil {
				objects = append(objects, tc.Host)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()

			machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
				nil, logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			check := machineMgr.nodeReuseLabelMatches(context.TODO(), tc.Host)
			if tc.expectNodeReuseLabel {
				Expect(tc.Host.Labels[nodeReuseLabelName]).To(Equal(tc.expectNodeReuseLabelName))
			}
			if tc.expectMatch {
				Expect(check).To(BeTrue())
			} else {
				Expect(check).To(BeFalse())
			}
		},
		Entry("Should return false if host is nil", testCaseNodeReuseLabelMatches{
			Host:        nil,
			expectMatch: false,
		}),
		Entry("Should return false if host labels is nil", testCaseNodeReuseLabelMatches{
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
			},
			expectMatch: false,
		}),
		Entry("Should match, if machine is controlplane  and nodeReuseLabelName matches KubeadmControlPlane name on the host", testCaseNodeReuseLabelMatches{
			Machine: &clusterv1.Machine{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: controlplanev1.GroupVersion.String(),
							Kind:       "KubeadmControlPlane",
							Name:       "test1",
						},
					},
					Labels: map[string]string{
						clusterv1.MachineControlPlaneLabelName: "cluster.x-k8s.io/control-plane",
					},
				},
			},
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kcp-test1",
					Labels: map[string]string{
						nodeReuseLabelName: "kcp-test1",
						"foo":              "bar",
					},
				},
			},
			expectNodeReuseLabel:     true,
			expectNodeReuseLabelName: "kcp-test1",
			expectMatch:              true,
		}),
		Entry("Should return false if nodeReuseLabelName is empty for host", testCaseNodeReuseLabelMatches{
			Machine: &clusterv1.Machine{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: controlplanev1.GroupVersion.String(),
							Kind:       "KubeadmControlPlane",
							Name:       "test1",
						},
					},
					Labels: map[string]string{
						clusterv1.MachineControlPlaneLabelName: "cluster.x-k8s.io/control-plane",
					},
				},
			},
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kcp-test1",
					Labels: map[string]string{
						nodeReuseLabelName: "",
					},
				},
			},
			expectNodeReuseLabel: false,
			expectMatch:          false,
		}),
		Entry("Should return false if nodeReuseLabelName on the host does not match with KubeadmControlPlane name", testCaseNodeReuseLabelMatches{
			Machine: &clusterv1.Machine{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: controlplanev1.GroupVersion.String(),
							Kind:       "KubeadmControlPlane",
							Name:       "test1",
						},
					},
					Labels: map[string]string{
						clusterv1.MachineControlPlaneLabelName: "cluster.x-k8s.io/control-plane",
					},
				},
			},
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kcp-test1",
					Labels: map[string]string{
						nodeReuseLabelName: "abc",
					},
				},
			},
			expectNodeReuseLabel: false,
			expectMatch:          false,
		}),
		Entry("Should return false if nodeReuseLabelName on host is empty and machine is not owned by KubeadmControlPlane", testCaseNodeReuseLabelMatches{
			Machine: &clusterv1.Machine{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.MachineControlPlaneLabelName: "",
					},
				},
			},
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "md-test1",
					Labels: map[string]string{
						nodeReuseLabelName: "",
					},
				},
			},
			expectNodeReuseLabel: false,
			expectMatch:          false,
		}),
		Entry("Should return false if nodeReuseLabelName on the host does not match with MachineDeployment name", testCaseNodeReuseLabelMatches{
			Machine: &clusterv1.Machine{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.MachineDeploymentLabelName: "cluster.x-k8s.io/deployment-name",
					},
				},
			},
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "md-test1",
					Labels: map[string]string{
						nodeReuseLabelName: "abc",
					},
				},
			},
			expectNodeReuseLabel: false,
			expectMatch:          false,
		}),
	)

	type testCaseNodeReuseLabelExists struct {
		Host                 *bmh.BareMetalHost
		expectNodeReuseLabel bool
	}
	DescribeTable("Test NodeReuseLabelExists",
		func(tc testCaseNodeReuseLabelExists) {
			objects := []client.Object{}
			if tc.Host != nil {
				objects = append(objects, tc.Host)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()

			machineMgr, err := NewMachineManager(fakeClient, nil, nil, nil,
				nil, logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			check := machineMgr.nodeReuseLabelExists(context.TODO(), tc.Host)
			Expect(err).NotTo(HaveOccurred())
			if tc.expectNodeReuseLabel {
				Expect(check).To(BeTrue())
			} else {
				Expect(check).To(BeFalse())
			}
		},
		Entry("Node reuse label exists on the host", testCaseNodeReuseLabelExists{
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						nodeReuseLabelName: kcpName,
						"foo":              "bar",
					},
				},
			},
			expectNodeReuseLabel: true,
		}),
		Entry("Node reuse label does not exist on the host", testCaseNodeReuseLabelExists{
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			expectNodeReuseLabel: false,
		}),
		Entry("Should return false if host is nil", testCaseNodeReuseLabelExists{
			Host:                 nil,
			expectNodeReuseLabel: false,
		}),
		Entry("Should return false if host Labels is nil", testCaseNodeReuseLabelExists{
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
			},
			expectNodeReuseLabel: false,
		}),
	)

	type testCaseGetKubeadmControlPlaneName struct {
		Machine         *clusterv1.Machine
		expectedKcp     bool
		expectedKcpName string
		expectError     bool
	}

	DescribeTable("Test getKubeadmControlPlaneName",
		func(tc testCaseGetKubeadmControlPlaneName) {
			objects := []client.Object{}
			if tc.Machine != nil {
				objects = append(objects, tc.Machine)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
				nil, logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			result, err := machineMgr.getKubeadmControlPlaneName(context.TODO())
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			if tc.expectedKcp {
				Expect(result).To(Equal(tc.expectedKcpName))
			}

		},
		Entry("Should find the expected kcp", testCaseGetKubeadmControlPlaneName{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: controlplanev1.GroupVersion.String(),
							Kind:       "KubeadmControlPlane",
							Name:       "test1",
						},
					},
				},
			},
			expectError:     false,
			expectedKcp:     true,
			expectedKcpName: "kcp-test1",
		}),
		Entry("Should not find the expected kcp, kind is not correct", testCaseGetKubeadmControlPlaneName{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: controlplanev1.GroupVersion.String(),
							Kind:       "kcp",
							Name:       "test1",
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("Should not find the expected kcp, API version is not correct", testCaseGetKubeadmControlPlaneName{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capm3.GroupVersion.String(),
							Kind:       "KubeadmControlPlane",
							Name:       "test1",
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("Should not find the expected kcp and error when machine is nil", testCaseGetKubeadmControlPlaneName{
			Machine:     nil,
			expectError: true,
		}),
		Entry("Should give an error if Machine.ObjectMeta.OwnerReferences is nil", testCaseGetKubeadmControlPlaneName{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: nil,
				},
			},
			expectError: true,
		}),
	)

	type testCaseGetMachineDeploymentName struct {
		Machine            *clusterv1.Machine
		MachineSetList     *clusterv1.MachineSetList
		expectedMachineSet *clusterv1.MachineSet
		expectedMD         bool
		expectedMDName     string
		expectError        bool
	}
	DescribeTable("Test GetMachineDeploymentName",
		func(tc testCaseGetMachineDeploymentName) {
			objects := []client.Object{}
			if tc.Machine != nil {
				objects = append(objects, tc.Machine)
			}
			if tc.MachineSetList != nil {
				for i := range tc.MachineSetList.Items {
					objects = append(objects, &tc.MachineSetList.DeepCopy().Items[i])
				}
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			machineMgr, err := NewMachineSetManager(fakeClient, tc.Machine,
				tc.MachineSetList, logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			result, err := machineMgr.getMachineDeploymentName(context.TODO())
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				Expect(tc.expectedMachineSet).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.expectedMD {
				Expect(result).To(Equal(tc.expectedMDName))
			} else {
				Expect(tc.expectedMDName).To(BeEmpty())
			}
		},
		Entry("Should find the expected MachineDeployment name", testCaseGetMachineDeploymentName{
			Machine: machineOwnedByMachineSet(),
			MachineSetList: &clusterv1.MachineSetList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "MachineSetList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []clusterv1.MachineSet{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "MachineSet",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "ms-test1",
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: clusterv1.GroupVersion.String(),
									Kind:       "MachineDeployment",
									Name:       "test1",
								},
							},
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test2",
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test3",
						},
					},
				},
			},
			expectedMachineSet: &clusterv1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ms-test1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "MachineDeployment",
							Name:       "test1",
						},
					},
				},
			},
			expectError:    false,
			expectedMD:     true,
			expectedMDName: "md-test1",
		}),
		Entry("Should not find the expected MachineDeployment name, MachineSet OwnerRef Kind is not correct", testCaseGetMachineDeploymentName{
			Machine: machineOwnedByMachineSet(),
			MachineSetList: &clusterv1.MachineSetList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "MachineSetList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []clusterv1.MachineSet{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "MachineSet",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "ms-test1",
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: clusterv1.GroupVersion.String(),
									Kind:       "WrongMachineSetOwnerRefKind",
									Name:       "test1",
								},
							},
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test2",
						},
					},
				},
			},
			expectedMachineSet: nil,
			expectError:        true,
			expectedMD:         false,
			expectedMDName:     "",
		}),
		Entry("Should not find the expected MachineDeployment name, MachineSet OwnerRef APIVersion is not correct", testCaseGetMachineDeploymentName{
			Machine: machineOwnedByMachineSet(),
			MachineSetList: &clusterv1.MachineSetList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "MachineSetList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []clusterv1.MachineSet{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "MachineSet",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "ms-test1",
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: capm3.GroupVersion.String(),
									Kind:       "MachineDeployment",
									Name:       "test1",
								},
							},
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test2",
						},
					},
				},
			},
			expectedMachineSet: nil,
			expectError:        true,
			expectedMD:         false,
			expectedMDName:     "",
		}),
		Entry("Should not find the expected MachineDeployment name, MachineSet OwnerRef is empty", testCaseGetMachineDeploymentName{
			Machine: machineOwnedByMachineSet(),
			MachineSetList: &clusterv1.MachineSetList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "MachineSetList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []clusterv1.MachineSet{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "MachineSet",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            "ms-test1",
							OwnerReferences: []metav1.OwnerReference{},
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test2",
						},
					},
				},
			},
			expectedMachineSet: nil,
			expectError:        true,
			expectedMD:         false,
			expectedMDName:     "",
		}),
	)

	type testCaseGetMachineSet struct {
		Machine            *clusterv1.Machine
		MachineSetList     *clusterv1.MachineSetList
		expectedMachineSet *clusterv1.MachineSet
		expectError        bool
	}
	DescribeTable("Test GetMachineSet",
		func(tc testCaseGetMachineSet) {
			objects := []client.Object{}
			if tc.MachineSetList != nil {
				for i := range tc.MachineSetList.Items {
					objects = append(objects, &tc.MachineSetList.DeepCopy().Items[i])
				}
			}
			if tc.Machine != nil {
				objects = append(objects, tc.Machine)
			}

			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			machineMgr, err := NewMachineSetManager(fakeClient, tc.Machine,
				tc.MachineSetList,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			result, err := machineMgr.getMachineSet(context.TODO())
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				Expect(tc.expectedMachineSet).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Name).To(Equal(tc.expectedMachineSet.Name))
			}
		},
		Entry("Should find the expected Machineset", testCaseGetMachineSet{
			Machine: machineOwnedByMachineSet(),
			MachineSetList: &clusterv1.MachineSetList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "MachineSetList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []clusterv1.MachineSet{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "MachineSet",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "ms-test1",
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test2",
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test3",
						},
					},
				},
			},
			expectedMachineSet: &clusterv1.MachineSet{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "ms-test1",
				},
			},
		}),
		Entry("Should not find the Machineset and error when machine is nil", testCaseGetMachineSet{
			Machine:            nil,
			expectedMachineSet: nil,
			MachineSetList:     nil,
			expectError:        true,
		}),
		Entry("Should not find the Machineset and error when machine is a controlplane", testCaseGetMachineSet{
			Machine: &clusterv1.Machine{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.MachineControlPlaneLabelName: "cluster.x-k8s.io/control-plane",
					},
				},
			},
			expectedMachineSet: nil,
			MachineSetList:     nil,
			expectError:        true,
		}),
		Entry("Should not find the Machineset and error when machine ownerRef is empty", testCaseGetMachineSet{
			Machine: &clusterv1.Machine{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{},
				},
			},
			expectedMachineSet: nil,
			MachineSetList:     nil,
			expectError:        true,
		}),
		Entry("Should not find the Machineset when machine ownerRef Kind is not correct", testCaseGetMachineSet{
			Machine: &clusterv1.Machine{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "TestMachine",
						},
					},
				},
			},
			MachineSetList: &clusterv1.MachineSetList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "MachineSetList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []clusterv1.MachineSet{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "MachineSet",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "ms-test1",
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test2",
						},
					},
				},
			},
			expectedMachineSet: nil,
			expectError:        true,
		}),
		Entry("Should not find the Machineset when machine ownerRef APIVersion is not correct", testCaseGetMachineSet{
			Machine: &clusterv1.Machine{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capm3.GroupVersion.String(),
						},
					},
				},
			},
			MachineSetList: &clusterv1.MachineSetList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "MachineSetList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []clusterv1.MachineSet{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "MachineSet",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "ms-test1",
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test2",
						},
					},
				},
			},
			expectedMachineSet: nil,
			expectError:        true,
		}),
		Entry("Should not find the Machineset when UID is not correct", testCaseGetMachineSet{
			Machine: &clusterv1.Machine{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							UID: "foo",
						},
					},
				},
			},
			MachineSetList: &clusterv1.MachineSetList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "MachineSetList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []clusterv1.MachineSet{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "MachineSet",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "ms-test1",
							UID:  "foo1",
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test2",
							UID:  "foo2",
						},
					},
				},
			},
			expectedMachineSet: nil,
			expectError:        true,
		}),
	)
})

/*-----------------------------------
---------Helper functions------------
------------------------------------*/

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
	if err := clusterv1.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
}

func newConfig(userDataNamespace string,
	labels map[string]string, reqs []capm3.HostSelectorRequirement,
) (*capm3.Metal3Machine, *corev1.ObjectReference) {
	config := capm3.Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
		},
		Spec: capm3.Metal3MachineSpec{
			Image: capm3.Image{
				URL:        testImageURL,
				Checksum:   testImageChecksumURL,
				DiskFormat: testImageDiskFormat,
			},
			UserData: &corev1.SecretReference{
				Name:      testUserDataSecretName,
				Namespace: userDataNamespace,
			},
			HostSelector: capm3.HostSelector{
				MatchLabels:      labels,
				MatchExpressions: reqs,
			},
		},
		Status: capm3.Metal3MachineStatus{
			UserData: &corev1.SecretReference{
				Name:      testUserDataSecretName,
				Namespace: userDataNamespace,
			},
			MetaData: &corev1.SecretReference{
				Name:      testMetaDataSecretName,
				Namespace: userDataNamespace,
			},
			NetworkData: &corev1.SecretReference{
				Name:      testNetworkDataSecretName,
				Namespace: userDataNamespace,
			},
		},
	}

	infrastructureRef := &corev1.ObjectReference{
		Name:       "someothermachine",
		Namespace:  namespaceName,
		Kind:       "M3Machine",
		APIVersion: capm3.GroupVersion.String(),
	}
	return &config, infrastructureRef
}

func newMachine(machineName string, metal3machineName string,
	infraRef *corev1.ObjectReference,
) *clusterv1.Machine {
	if machineName == "" {
		return &clusterv1.Machine{}
	}

	if infraRef == nil {
		infraRef = &corev1.ObjectReference{}
	}

	machine := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Machine",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: namespaceName,
		},
		Spec: clusterv1.MachineSpec{
			ClusterName:       clusterName,
			InfrastructureRef: *infraRef,
			Bootstrap: clusterv1.Bootstrap{
				ConfigRef:      &corev1.ObjectReference{},
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
			Namespace: namespaceName,
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
	autoCleanMode string,
	clusterlabel bool) *bmh.BareMetalHost {
	if name == "" {
		return &bmh.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespaceName,
			},
		}
	}

	objMeta := &metav1.ObjectMeta{
		Name:      name,
		Namespace: namespaceName,
	}

	if clusterlabel == true {
		objMeta = &metav1.ObjectMeta{
			Name:      name,
			Namespace: namespaceName,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
				"foo":                      "bar",
			},
		}
	}

	if spec == nil {
		return &bmh.BareMetalHost{
			ObjectMeta: *objMeta,
		}
	}
	spec.AutomatedCleaningMode = bmh.AutomatedCleaningMode(autoCleanMode)

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
	objMeta := &metav1.ObjectMeta{
		Name:      name,
		Namespace: namespaceName,
	}
	if clusterlabel == true {
		objMeta = &metav1.ObjectMeta{
			Name:      name,
			Namespace: namespaceName,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
				"foo":                      "bar",
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
			Namespace:  namespaceName,
			Finalizers: []string{"abcd"},
		},
		Data: map[string][]byte{
			"userData": []byte("QmFyRm9vCg=="),
		},
		Type: "Opaque",
	}
}
