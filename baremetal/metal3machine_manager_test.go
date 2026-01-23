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
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/go-logr/logr"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testImageURL              = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2"
	testImageChecksumURL      = "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.sha256sum"
	testUserDataSecretName    = "worker-user-data"
	testMetaDataSecretName    = "worker-metadata"
	testNetworkDataSecretName = "worker-network-data"
	cpName                    = "cp-pool1"
)

var Bmhuid = types.UID("4d25a2c2-46e4-11ec-81d3-0242ac130003")
var ProviderID = fmt.Sprintf("metal3://%s", Bmhuid)

var testImageDiskFormat = ptr.To("raw")

func m3mSpec() *infrav1.Metal3MachineSpec {
	return &infrav1.Metal3MachineSpec{
		ProviderID: &ProviderID,
	}
}

func m3mSpecAll() *infrav1.Metal3MachineSpec {
	return &infrav1.Metal3MachineSpec{
		ProviderID: &ProviderID,
		UserData: &corev1.SecretReference{
			Name:      metal3machineName + "-user-data",
			Namespace: namespaceName,
		},
		Image: infrav1.Image{
			URL:          testImageURL,
			Checksum:     testImageChecksumURL,
			ChecksumType: ptr.To("sha512"),
			DiskFormat:   testImageDiskFormat,
		},
		HostSelector: infrav1.HostSelector{},
	}
}

func m3mSecretStatus() *infrav1.Metal3MachineStatus {
	return &infrav1.Metal3MachineStatus{
		UserData: &corev1.SecretReference{
			Name:      metal3machineName + "-user-data",
			Namespace: namespaceName,
		},
		MetaData: &corev1.SecretReference{
			Name:      metal3machineName + "-meta-data",
			Namespace: namespaceName,
		},
		NetworkData: &corev1.SecretReference{
			Name:      metal3machineName + "-network-data",
			Namespace: namespaceName,
		},
	}
}

func m3mSecretStatusNil() *infrav1.Metal3MachineStatus {
	return &infrav1.Metal3MachineStatus{}
}

func consumerRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name:       metal3machineName,
		Namespace:  namespaceName,
		Kind:       metal3MachineKind,
		APIVersion: infrav1.GroupVersion.String(),
	}
}

func consumerRefSome() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name:       "someoneelsesmachine",
		Namespace:  namespaceName,
		Kind:       metal3MachineKind,
		APIVersion: clusterv1beta1.GroupVersion.String(),
	}
}

func expectedImg() *bmov1alpha1.Image {
	return &bmov1alpha1.Image{
		URL:        testImageURL,
		Checksum:   testImageChecksumURL,
		DiskFormat: testImageDiskFormat,
	}
}

func expectedImgTest() *bmov1alpha1.Image {
	return &bmov1alpha1.Image{
		URL:      testImageURL + "test",
		Checksum: testImageChecksumURL + "test",
	}
}

func expectedCustomDeployTest() *bmov1alpha1.CustomDeploy {
	return &bmov1alpha1.CustomDeploy{
		Method: "test_test",
	}
}

func bmhSpec() *bmov1alpha1.BareMetalHostSpec {
	return &bmov1alpha1.BareMetalHostSpec{
		ConsumerRef: consumerRef(),
		Image: &bmov1alpha1.Image{
			URL: "myimage",
		},
		Online: true,
	}
}

func bmhSpecBMC() *bmov1alpha1.BareMetalHostSpec {
	return &bmov1alpha1.BareMetalHostSpec{
		ConsumerRef: consumerRef(),
		BMC: bmov1alpha1.BMCDetails{
			Address:         "myAddress",
			CredentialsName: "mycredentials",
		},
	}
}

func bmhSpecTestImg() *bmov1alpha1.BareMetalHostSpec {
	return &bmov1alpha1.BareMetalHostSpec{
		ConsumerRef: consumerRef(),
		Image:       expectedImgTest(),
	}
}

func bmhSpecTestCustomDeploy() *bmov1alpha1.BareMetalHostSpec {
	return &bmov1alpha1.BareMetalHostSpec{
		ConsumerRef:  consumerRef(),
		CustomDeploy: expectedCustomDeployTest(),
	}
}

func bmhSpecSomeImg() *bmov1alpha1.BareMetalHostSpec {
	return &bmov1alpha1.BareMetalHostSpec{
		ConsumerRef: consumerRefSome(),
		Image: &bmov1alpha1.Image{
			URL: "someoneelsesimage",
		},
		MetaData:    &corev1.SecretReference{},
		NetworkData: &corev1.SecretReference{},
		UserData:    &corev1.SecretReference{},
		Online:      true,
	}
}

func bmhSpecNoImg() *bmov1alpha1.BareMetalHostSpec {
	return &bmov1alpha1.BareMetalHostSpec{
		ConsumerRef: consumerRef(),
	}
}

func m3mObjectMetaWithValidAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            metal3machineName,
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
		Labels: map[string]string{
			clusterv1.ClusterNameLabel: clusterName,
		},
		Annotations: map[string]string{
			HostAnnotation: namespaceName + "/" + baremetalhostName,
		},
	}
}

func bmhObjectMetaWithValidCAPM3PausedAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            baremetalhostName,
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
		Labels: map[string]string{
			clusterv1.ClusterNameLabel: clusterName,
		},
		Annotations: map[string]string{
			bmov1alpha1.PausedAnnotation: PausedAnnotationKey,
		},
	}
}

func bmhObjectMetaWithValidEmptyPausedAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            baremetalhostName,
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
		Annotations: map[string]string{
			bmov1alpha1.PausedAnnotation: "",
		},
	}
}

func bmhObjectMetaEmptyAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            baremetalhostName,
		Namespace:       namespaceName,
		OwnerReferences: []metav1.OwnerReference{},
		Annotations:     map[string]string{},
	}
}

func bmhObjectMetaNoAnnotations() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:            baremetalhostName,
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

func bmhPowerStatus() *bmov1alpha1.BareMetalHostStatus {
	return &bmov1alpha1.BareMetalHostStatus{
		Provisioning: bmov1alpha1.ProvisionStatus{
			State: bmov1alpha1.StateNone,
		},
		PoweredOn: true,
	}
}

func bmhStatus() *bmov1alpha1.BareMetalHostStatus {
	return &bmov1alpha1.BareMetalHostStatus{
		Provisioning: bmov1alpha1.ProvisionStatus{
			State: bmov1alpha1.StateNone,
		},
	}
}

var _ = Describe("Metal3Machine manager", func() {
	DescribeTable("Test Finalizers",
		func(bmMachine infrav1.Metal3Machine) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &bmMachine,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			machineMgr.SetFinalizer()

			Expect(bmMachine.ObjectMeta.Finalizers).To(ContainElement(
				infrav1.MachineFinalizer,
			))

			machineMgr.UnsetFinalizer()

			Expect(bmMachine.ObjectMeta.Finalizers).NotTo(ContainElement(
				infrav1.MachineFinalizer,
			))
		},
		Entry("No finalizers", infrav1.Metal3Machine{}),
		Entry("Additional Finalizers", infrav1.Metal3Machine{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"foo"},
			},
		}),
	)

	DescribeTable("Test SetProviderID",
		func(bmMachine infrav1.Metal3Machine) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &bmMachine,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			machineMgr.SetProviderID("correct")

			Expect(*bmMachine.Spec.ProviderID).To(Equal("correct"))
		},
		Entry("no ProviderID", infrav1.Metal3Machine{}),
		Entry("existing ProviderID", infrav1.Metal3Machine{
			Spec: infrav1.Metal3MachineSpec{
				ProviderID: ptr.To("wrong"),
			},
			Status: infrav1.Metal3MachineStatus{
				Ready: true,
			},
		}),
	)

	type testCaseProvisioned struct {
		M3Machine  infrav1.Metal3Machine
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
			M3Machine: infrav1.Metal3Machine{
				Spec: infrav1.Metal3MachineSpec{
					ProviderID: ptr.To("abc"),
				},
				Status: infrav1.Metal3MachineStatus{
					Ready: true,
					Conditions: clusterv1beta1.Conditions{
						*v1beta1conditions.TrueCondition(infrav1.KubernetesNodeReadyCondition),
					},
				},
			},
			ExpectTrue: true,
		}),
		Entry("missing ready", testCaseProvisioned{
			M3Machine: infrav1.Metal3Machine{
				Spec: infrav1.Metal3MachineSpec{
					ProviderID: ptr.To("abc"),
				},
			},
			ExpectTrue: false,
		}),
		Entry("missing providerID", testCaseProvisioned{
			M3Machine: infrav1.Metal3Machine{
				Status: infrav1.Metal3MachineStatus{
					Ready: true,
				},
			},
			ExpectTrue: false,
		}),
		Entry("missing ProviderID and ready", testCaseProvisioned{
			M3Machine:  infrav1.Metal3Machine{},
			ExpectTrue: false,
		}),
	)

	type testCaseBootstrapReady struct {
		Machine    clusterv1.Machine
		ExpectTrue bool
	}

	testCaseBootstrapReadySecretName := "secret"
	v1beta1BootstrapReadyTrue := clusterv1.Conditions{
		clusterv1.Condition{Type: clusterv1.BootstrapReadyV1Beta1Condition, Status: corev1.ConditionTrue},
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
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: clusterv1.ContractVersionedObjectReference{
							Name: "abc",
						},
					},
				},
				Status: clusterv1.MachineStatus{
					Deprecated: &clusterv1.MachineDeprecatedStatus{
						V1Beta1: &clusterv1.MachineV1Beta1DeprecatedStatus{
							Conditions: v1beta1BootstrapReadyTrue,
						},
					},
				},
			},
			ExpectTrue: true,
		}),
		Entry("not ready", testCaseBootstrapReady{
			Machine:    clusterv1.Machine{},
			ExpectTrue: false,
		}),
		Entry("ConfigRef defined but condition not set", testCaseBootstrapReady{
			Machine: clusterv1.Machine{
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: clusterv1.ContractVersionedObjectReference{
							Name: "abc",
						},
					},
				},
				// No Status.Deprecated.V1Beta1.Conditions set - should return nil from Get()
			},
			ExpectTrue: false,
		}),
		Entry("ready data secret", testCaseBootstrapReady{
			Machine: clusterv1.Machine{
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: &testCaseBootstrapReadySecretName,
					},
				},
			},
			ExpectTrue: true,
		}),
	)

	DescribeTable("Test setting errors",
		func(bmMachine infrav1.Metal3Machine) {
			machineMgr, err := NewMachineManager(nil, nil, nil, nil, &bmMachine,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			machineMgr.SetError("abc", capierrors.InvalidConfigurationMachineError)

			Expect(*bmMachine.Status.FailureReason).To(Equal(
				capierrors.InvalidConfigurationMachineError,
			))
			Expect(*bmMachine.Status.FailureMessage).To(Equal("abc"))
		},
		Entry("No errors", infrav1.Metal3Machine{}),
		Entry("Overwrite existing error message", infrav1.Metal3Machine{
			Status: infrav1.Metal3MachineStatus{
				FailureMessage: ptr.To("cba"),
			},
		}),
	)

	Describe("Test ChooseHost", func() {

		// Creating the hosts
		hostWithOtherConsRef := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostWithOtherConsRef",
				Namespace: namespaceName,
			},
			Spec: bmov1alpha1.BareMetalHostSpec{
				ConsumerRef: &corev1.ObjectReference{
					Name:       "someothermachine",
					Namespace:  namespaceName,
					Kind:       metal3MachineKind,
					APIVersion: infrav1.GroupVersion.String(),
				},
			},
		}

		availableHost := newBareMetalHost("availableHost", &bmov1alpha1.BareMetalHostSpec{}, bmov1alpha1.StateReady, &bmov1alpha1.BareMetalHostStatus{}, true, "metadata", false, "")

		hostWithConRef := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostWithConRef",
				Namespace: namespaceName,
			},
			Spec: bmov1alpha1.BareMetalHostSpec{
				ConsumerRef: &corev1.ObjectReference{
					Name:       metal3machineName,
					Namespace:  namespaceName,
					Kind:       metal3MachineKind,
					APIVersion: infrav1.GroupVersion.String(),
				},
			},
		}
		hostInOtherNS := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostInOtherNS",
				Namespace: "someotherns",
			},
			Status: bmov1alpha1.BareMetalHostStatus{
				Provisioning: bmov1alpha1.ProvisionStatus{
					State: bmov1alpha1.StateAvailable,
				},
			},
		}
		discoveredHost := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "discoveredHost",
				Namespace: namespaceName,
				Labels:    map[string]string{"key1": "value1"},
			},
			Status: bmov1alpha1.BareMetalHostStatus{
				ErrorMessage: "this host is discovered but not usable",
			},
		}
		hostWithLabel := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostWithLabel",
				Namespace: namespaceName,
				Labels:    map[string]string{"key1": "value1"},
			},
			Status: bmov1alpha1.BareMetalHostStatus{
				Provisioning: bmov1alpha1.ProvisionStatus{
					State: bmov1alpha1.StateAvailable,
				},
			},
		}
		hostWithFailureDomainLabel := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostWithFailureDomainLabel",
				Namespace: namespaceName,
				Labels:    map[string]string{FailureDomainLabelName: "my-fd-1"},
			},
			Status: bmov1alpha1.BareMetalHostStatus{
				Provisioning: bmov1alpha1.ProvisionStatus{
					State: bmov1alpha1.StateAvailable,
				},
			},
		}
		hostWithUnhealthyAnnotation := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "hostWithUnhealthyAnnotation",
				Namespace:   namespaceName,
				Annotations: map[string]string{infrav1.UnhealthyAnnotation: "unhealthy"},
			},
			Status: bmov1alpha1.BareMetalHostStatus{
				Provisioning: bmov1alpha1.ProvisionStatus{
					State: bmov1alpha1.StateAvailable,
				},
			},
		}
		hostWithPausedAnnotation := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "hostWithPausedAnnotation",
				Namespace:   namespaceName,
				Annotations: map[string]string{bmov1alpha1.PausedAnnotation: PausedAnnotationKey},
			},
			Status: bmov1alpha1.BareMetalHostStatus{
				Provisioning: bmov1alpha1.ProvisionStatus{
					State: bmov1alpha1.StateAvailable,
				},
			},
		}
		hostWithNodeReuseLabelSetToCP := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostWithNodeReuseLabelSetToCP",
				Namespace: namespaceName,
				Labels:    map[string]string{nodeReuseLabelName: "cp-test1"},
			},
			Spec: bmov1alpha1.BareMetalHostSpec{},
			Status: bmov1alpha1.BareMetalHostStatus{
				Provisioning: bmov1alpha1.ProvisionStatus{
					State: bmov1alpha1.StateAvailable,
				},
			},
		}
		hostWithNodeReuseLabelSetToCPinFailureDomain := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostWithNodeReuseLabelSetToCPinFailureDomain",
				Namespace: namespaceName,
				Labels:    map[string]string{nodeReuseLabelName: "cp-test1", FailureDomainLabelName: "my-fd-1"},
			},
			Spec: bmov1alpha1.BareMetalHostSpec{},
			Status: bmov1alpha1.BareMetalHostStatus{
				Provisioning: bmov1alpha1.ProvisionStatus{
					State: bmov1alpha1.StateAvailable,
				},
			},
		}
		hostWithNodeReuseLabelStateNone := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hostWithNodeReuseLabelStateNone",
				Namespace: namespaceName,
				Labels:    map[string]string{nodeReuseLabelName: "cp-test1"},
			},
			Spec: bmov1alpha1.BareMetalHostSpec{},
			Status: bmov1alpha1.BareMetalHostStatus{
				Provisioning: bmov1alpha1.ProvisionStatus{
					State: bmov1alpha1.StateNone,
				},
			},
		}

		m3mconfig, infrastructureRef := newConfig("", map[string]string{},
			[]infrav1.HostSelectorRequirement{}, "",
		)
		m3mconfig2, infrastructureRef2 := newConfig("",
			map[string]string{"key1": "value1"}, []infrav1.HostSelectorRequirement{}, "",
		)
		m3mconfig3, infrastructureRef3 := newConfig("",
			map[string]string{"boguskey": "value"}, []infrav1.HostSelectorRequirement{}, "",
		)
		m3mconfig4, infrastructureRef4 := newConfig("", map[string]string{},
			[]infrav1.HostSelectorRequirement{
				{
					Key:      "key1",
					Operator: "in",
					Values:   []string{"abc", "value1", "123"},
				},
			},
			"",
		)
		m3mconfig5, infrastructureRef5 := newConfig("", map[string]string{},
			[]infrav1.HostSelectorRequirement{
				{
					Key:      "key1",
					Operator: "pancakes",
					Values:   []string{"abc", "value1", "123"},
				},
			},
			"",
		)
		m3mconfig6, infrastructureRef6 := newConfig("", map[string]string{},
			[]infrav1.HostSelectorRequirement{}, "my-fd-1",
		)

		type testCaseChooseHost struct {
			Machine          *clusterv1.Machine
			Hosts            *bmov1alpha1.BareMetalHostList
			M3Machine        *infrav1.Metal3Machine
			ExpectedHostName string
		}

		DescribeTable("Test ChooseHost",
			func(tc testCaseChooseHost) {
				objects := []client.Object{}
				if tc.Hosts != nil {
					for i := range tc.Hosts.Items {
						objects = append(objects, &tc.Hosts.DeepCopy().Items[i])
					}
				}
				if tc.Machine != nil {
					objects = append(objects, tc.Machine)
				}
				if tc.M3Machine != nil {
					objects = append(objects, tc.M3Machine)
				}
				fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
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
				if tc.ExpectedHostName != "" {
					Expect(result.Name).To(Equal(tc.ExpectedHostName))
				}
			},
			Entry("Pick hostWithNodeReuseLabelSetToCP, which has a matching nodeReuseLabelName", testCaseChooseHost{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      machineName,
						Namespace: namespaceName,

						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "controlplane.cluster.x-k8s.io/v1beta2",
								Name:       "test1",
								Kind:       "KubeadmControlPlane",
							},
						},
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "cluster.x-k8s.io/control-plane",
						},
					},
					Spec: clusterv1.MachineSpec{
						InfrastructureRef: *infrastructureRef,
					},
				},
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{hostWithOtherConsRef, *availableHost, hostWithNodeReuseLabelSetToCP}},
				M3Machine:        m3mconfig,
				ExpectedHostName: hostWithNodeReuseLabelSetToCP.Name,
			}),
			Entry("Requeu hostWithNodeReuseLabelStateNone, which has a matching nodeReuseLabelName", testCaseChooseHost{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      machineName,
						Namespace: namespaceName,

						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "controlplane.cluster.x-k8s.io/v1beta2",
								Name:       "test1",
								Kind:       "KubeadmControlPlane",
							},
						},
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "cluster.x-k8s.io/control-plane",
						},
					},
					Spec: clusterv1.MachineSpec{
						InfrastructureRef: *infrastructureRef,
					},
				},
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{hostWithOtherConsRef, *availableHost, hostWithNodeReuseLabelStateNone}},
				M3Machine:        m3mconfig,
				ExpectedHostName: "",
			}),
			Entry("Pick availableHost which lacks ConsumerRef", testCaseChooseHost{
				Machine:          newMachine(machineName, infrastructureRef),
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{hostWithOtherConsRef, *availableHost}},
				M3Machine:        m3mconfig,
				ExpectedHostName: availableHost.Name,
			}),
			Entry("Ignore discoveredHost and pick availableHost, which lacks a ConsumerRef",
				testCaseChooseHost{
					Machine:          newMachine(machineName, infrastructureRef),
					Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{discoveredHost, hostWithOtherConsRef, *availableHost}},
					M3Machine:        m3mconfig,
					ExpectedHostName: availableHost.Name,
				},
			),
			Entry("Ignore hostWithUnhealthyAnnotation and pick availableHost, which lacks a ConsumerRef",
				testCaseChooseHost{
					Machine:          newMachine(machineName, infrastructureRef),
					Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{hostWithUnhealthyAnnotation, hostWithOtherConsRef, *availableHost}},
					M3Machine:        m3mconfig,
					ExpectedHostName: availableHost.Name,
				},
			),
			Entry("Ignore hostWithPausedAnnotation and pick availableHost, which lacks a ConsumerRef",
				testCaseChooseHost{
					Machine:          newMachine(machineName, infrastructureRef),
					Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{hostWithPausedAnnotation, hostWithOtherConsRef, *availableHost}},
					M3Machine:        m3mconfig,
					ExpectedHostName: availableHost.Name,
				},
			),
			Entry("Pick hostWithConRef, which has a matching ConsumerRef", testCaseChooseHost{
				Machine:          newMachine(machineName, infrastructureRef2),
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{hostWithOtherConsRef, *availableHost, hostWithConRef}},
				M3Machine:        newMetal3Machine(metal3machineName, nil, m3mSecretStatus(), nil),
				ExpectedHostName: hostWithConRef.Name,
			}),

			Entry("Two hosts already taken, third is in another namespace",
				testCaseChooseHost{
					Machine:          newMachine("machine2", infrastructureRef),
					Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{hostWithOtherConsRef, hostWithConRef, hostInOtherNS}},
					M3Machine:        m3mconfig,
					ExpectedHostName: "",
				}),

			Entry("Choose hosts with a label, even without a label selector",
				testCaseChooseHost{
					Machine:          newMachine(machineName, infrastructureRef),
					Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{hostWithLabel}},
					M3Machine:        m3mconfig,
					ExpectedHostName: hostWithLabel.Name,
				},
			),
			Entry("Choose hosts with a nodeReuseLabelName set to CP, even without a label selector",
				testCaseChooseHost{
					Machine: &clusterv1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      machineName,
							Namespace: namespaceName,

							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "controlplane.cluster.x-k8s.io/v1beta2",
									Name:       "test1",
									Kind:       "KubeadmControlPlane",
								},
							},
							Labels: map[string]string{
								clusterv1.MachineControlPlaneLabel: "cluster.x-k8s.io/control-plane",
							},
						},
						Spec: clusterv1.MachineSpec{
							InfrastructureRef: *infrastructureRef,
						},
					},
					Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{hostWithNodeReuseLabelSetToCP}},
					M3Machine:        m3mconfig,
					ExpectedHostName: hostWithNodeReuseLabelSetToCP.Name,
				},
			),
			Entry("Choose the host with the right label", testCaseChooseHost{
				Machine:          newMachine(machineName, infrastructureRef2),
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{hostWithLabel, *availableHost}},
				M3Machine:        m3mconfig2,
				ExpectedHostName: hostWithLabel.Name,
			}),
			Entry("No host that matches required label", testCaseChooseHost{
				Machine:          newMachine(machineName, infrastructureRef3),
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{*availableHost, hostWithLabel}},
				M3Machine:        m3mconfig3,
				ExpectedHostName: "",
			}),
			Entry("Host that matches a matchExpression", testCaseChooseHost{
				Machine:          newMachine(machineName, infrastructureRef4),
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{*availableHost, hostWithLabel}},
				M3Machine:        m3mconfig4,
				ExpectedHostName: hostWithLabel.Name,
			}),
			Entry("No Host available that matches a matchExpression",
				testCaseChooseHost{
					Machine:          newMachine(machineName, infrastructureRef4),
					Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{*availableHost}},
					M3Machine:        m3mconfig4,
					ExpectedHostName: "",
				},
			),
			Entry("No host chosen, invalid match expression", testCaseChooseHost{
				Machine:          newMachine(machineName, infrastructureRef5),
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{*availableHost, hostWithLabel, hostWithOtherConsRef}},
				M3Machine:        m3mconfig5,
				ExpectedHostName: "",
			}),
			Entry("Choose a host in Failure Domain", testCaseChooseHost{
				Machine:          newMachine(machineName, infrastructureRef6),
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{*availableHost, hostWithLabel, hostWithOtherConsRef, hostWithFailureDomainLabel}},
				M3Machine:        m3mconfig6,
				ExpectedHostName: hostWithFailureDomainLabel.Name,
			}),
			Entry("Choose available host, when hosts in FailureDomain not available", testCaseChooseHost{
				Machine:          newMachine(machineName, infrastructureRef6),
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{*availableHost, hostWithOtherConsRef, hostWithNodeReuseLabelSetToCP}},
				M3Machine:        m3mconfig6,
				ExpectedHostName: availableHost.Name,
			}),
			Entry("Choose a host in Failure Domain, when NodeReuse is set", testCaseChooseHost{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      machineName,
						Namespace: namespaceName,

						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "controlplane.cluster.x-k8s.io/v1beta2",
								Name:       "test1",
								Kind:       "KubeadmControlPlane",
							},
						},
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "cluster.x-k8s.io/control-plane",
						},
					},
					Spec: clusterv1.MachineSpec{
						InfrastructureRef: *infrastructureRef,
					},
				},
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{*availableHost, hostWithLabel, hostWithFailureDomainLabel, hostWithNodeReuseLabelSetToCPinFailureDomain}},
				M3Machine:        m3mconfig6,
				ExpectedHostName: hostWithNodeReuseLabelSetToCPinFailureDomain.Name,
			}),
			Entry("Choose host is not in Failure Domain, when NodeReuse is set", testCaseChooseHost{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      machineName,
						Namespace: namespaceName,

						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "controlplane.cluster.x-k8s.io/v1beta2",
								Name:       "test1",
								Kind:       "KubeadmControlPlane",
							},
						},
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "cluster.x-k8s.io/control-plane",
						},
					},
					Spec: clusterv1.MachineSpec{
						InfrastructureRef: *infrastructureRef,
					},
				},
				Hosts:            &bmov1alpha1.BareMetalHostList{Items: []bmov1alpha1.BareMetalHost{*availableHost, hostWithLabel, hostWithFailureDomainLabel, hostWithNodeReuseLabelSetToCP}},
				M3Machine:        m3mconfig6,
				ExpectedHostName: hostWithNodeReuseLabelSetToCP.Name,
			}),
		)
	})

	type testCaseSetPauseAnnotation struct {
		M3Machine           *infrav1.Metal3Machine
		Host                *bmov1alpha1.BareMetalHost
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

			savedHost := bmov1alpha1.BareMetalHost{}
			err = fakeClient.Get(context.TODO(),
				client.ObjectKey{
					Name:      tc.Host.Name,
					Namespace: tc.Host.Namespace,
				},
				&savedHost,
			)
			Expect(err).NotTo(HaveOccurred())
			_, pausePresent := savedHost.Annotations[bmov1alpha1.PausedAnnotation]
			if tc.ExpectPausePresent {
				Expect(pausePresent).To(BeTrue())
			} else {
				Expect(pausePresent).To(BeFalse())
			}
			status, statusPresent := savedHost.Annotations[bmov1alpha1.StatusAnnotation]
			if tc.ExpectStatusPresent {
				Expect(statusPresent).To(BeTrue())
				annotation, err := json.Marshal(&tc.Host.Status)
				Expect(err).ToNot(HaveOccurred())
				// (Note) manager code marshals the inspection data stored in annotation,
				// which causes alphabetically reordering of keys. Since we are marshaling
				// only the annotation, the status value here doesn't match the marshaled
				// annotation data, because it wasn't re-ordered by the JSON marshaller.
				// That's why we are marshaling status data as well so that its fields are
				// also alphabetically reordered to match the annotation keys style..
				obj := map[string]any{}
				err = json.Unmarshal(annotation, &obj)
				Expect(err).ToNot(HaveOccurred())
				annotation, _ = json.Marshal(obj)
				Expect(status).To(Equal(string(annotation)))
			} else {
				Expect(statusPresent).To(BeFalse())
			}
		},
		Entry("Set BMH Pause Annotation, with valid CAPM3 Paused annotations, already paused", testCaseSetPauseAnnotation{
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: *bmhObjectMetaWithValidCAPM3PausedAnnotations(),
				Spec: bmov1alpha1.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       metal3machineName,
						Namespace:  namespaceName,
						Kind:       metal3MachineKind,
						APIVersion: infrav1.GroupVersion.String(),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPausePresent: true,
			ExpectError:        false,
		}),
		Entry("Set BMH Pause Annotation, with valid Paused annotations, Empty Key, already paused", testCaseSetPauseAnnotation{
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: *bmhObjectMetaWithValidEmptyPausedAnnotations(),
				Spec: bmov1alpha1.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       metal3machineName,
						Namespace:  namespaceName,
						Kind:       metal3MachineKind,
						APIVersion: infrav1.GroupVersion.String(),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPausePresent: true,
			ExpectError:        false,
		}),
		Entry("Set BMH Pause Annotation, with no Paused annotations", testCaseSetPauseAnnotation{
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: *bmhObjectMetaEmptyAnnotations(),
				Spec: bmov1alpha1.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       metal3machineName,
						Namespace:  namespaceName,
						Kind:       metal3MachineKind,
						APIVersion: infrav1.GroupVersion.String(),
					},
				},
				Status: bmov1alpha1.BareMetalHostStatus{
					OperationalStatus: "OK",
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPausePresent:  true,
			ExpectStatusPresent: true,
			ExpectError:         false,
		}),
	)

	type testCaseRemovePauseAnnotation struct {
		Cluster       *clusterv1.Cluster
		M3Machine     *infrav1.Metal3Machine
		Host          *bmov1alpha1.BareMetalHost
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

			savedHost := bmov1alpha1.BareMetalHost{}
			err = fakeClient.Get(context.TODO(),
				client.ObjectKey{
					Name:      tc.Host.Name,
					Namespace: tc.Host.Namespace,
				},
				&savedHost,
			)
			Expect(err).NotTo(HaveOccurred())
			if tc.ExpectPresent {
				Expect(savedHost.Annotations[bmov1alpha1.PausedAnnotation]).NotTo(BeNil())
			} else {
				Expect(savedHost.Annotations).To(BeNil())
			}
		},
		Entry("Remove BMH Pause Annotation, with valid CAPM3 Paused annotations", testCaseRemovePauseAnnotation{
			Cluster: newCluster(clusterName),
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: *bmhObjectMetaWithValidCAPM3PausedAnnotations(),
				Spec: bmov1alpha1.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       metal3machineName,
						Namespace:  namespaceName,
						Kind:       metal3MachineKind,
						APIVersion: infrav1.GroupVersion.String(),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPresent: false,
			ExpectError:   false,
		}),
		Entry("Do not Remove Annotation, with valid Paused annotations, Empty Key", testCaseRemovePauseAnnotation{
			Cluster: newCluster(clusterName),
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: *bmhObjectMetaWithValidEmptyPausedAnnotations(),
				Spec: bmov1alpha1.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       metal3machineName,
						Namespace:  namespaceName,
						Kind:       metal3MachineKind,
						APIVersion: infrav1.GroupVersion.String(),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPresent: true,
			ExpectError:   false,
		}),
		Entry("No Annotation, Should Not Error", testCaseRemovePauseAnnotation{
			Cluster: newCluster(clusterName),
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: *bmhObjectMetaNoAnnotations(),
				Spec: bmov1alpha1.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       metal3machineName,
						Namespace:  namespaceName,
						Kind:       metal3MachineKind,
						APIVersion: infrav1.GroupVersion.String(),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, m3mSpec(), nil,
				m3mObjectMetaWithValidAnnotations()),
			ExpectPresent: false,
			ExpectError:   false,
		}),
	)

	type testCaseSetHostSpec struct {
		UserDataNamespace           string
		UseCustomDeploy             *bmov1alpha1.CustomDeploy
		ExpectedUserDataNamespace   string
		Host                        *bmov1alpha1.BareMetalHost
		ExpectedImage               *bmov1alpha1.Image
		ExpectedCustomDeploy        *bmov1alpha1.CustomDeploy
		ExpectUserData              bool
		expectNodeReuseLabelDeleted bool
	}

	DescribeTable("Test SetHostSpec",
		func(tc testCaseSetHostSpec) {
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(tc.Host).Build()

			m3mconfig, infrastructureRef := newConfig(tc.UserDataNamespace,
				map[string]string{}, []infrav1.HostSelectorRequirement{}, "",
			)
			if tc.UseCustomDeploy != nil {
				m3mconfig.Spec.Image = infrav1.Image{}
				m3mconfig.Spec.CustomDeploy = &infrav1.CustomDeploy{
					Method: tc.UseCustomDeploy.Method,
				}
			}
			machine := newMachine(machineName, infrastructureRef)

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
			if tc.ExpectedCustomDeploy == nil {
				Expect(tc.Host.Spec.CustomDeploy).To(BeNil())
			} else {
				Expect(*tc.Host.Spec.CustomDeploy).To(Equal(*tc.ExpectedCustomDeploy))
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
			Host: newBareMetalHost("host2", nil, bmov1alpha1.StateNone,
				nil, false, "metadata", false, "",
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("User data has no namespace", testCaseSetHostSpec{
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: namespaceName,
			Host: newBareMetalHost("host2", nil, bmov1alpha1.StateNone,
				nil, false, "metadata", false, "",
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("Externally provisioned, same machine", testCaseSetHostSpec{
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: namespaceName,
			Host: newBareMetalHost("host2", nil, bmov1alpha1.StateNone,
				nil, false, "metadata", false, "",
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("Using custom deploy", testCaseSetHostSpec{
			UseCustomDeploy:           expectedCustomDeployTest(),
			ExpectedUserDataNamespace: namespaceName,
			Host: newBareMetalHost("host2", nil, bmov1alpha1.StateNone,
				nil, false, "metadata", false, "",
			),
			ExpectedCustomDeploy: expectedCustomDeployTest(),
			ExpectUserData:       true,
		}),
		Entry("Previously provisioned, different image",
			testCaseSetHostSpec{
				UserDataNamespace:         "",
				ExpectedUserDataNamespace: namespaceName,
				Host: newBareMetalHost("host2", bmhSpecTestImg(),
					bmov1alpha1.StateNone, nil, false, "metadata", false, "",
				),
				ExpectedImage:  expectedImgTest(),
				ExpectUserData: false,
			},
		),
		Entry("Previously provisioned, different custom deploy",
			testCaseSetHostSpec{
				UserDataNamespace:         "",
				ExpectedUserDataNamespace: namespaceName,
				Host: newBareMetalHost("host2", bmhSpecTestCustomDeploy(),
					bmov1alpha1.StateNone, nil, false, "metadata", false, "",
				),
				ExpectedCustomDeploy: expectedCustomDeployTest(),
				ExpectUserData:       false,
			},
		),
	)

	DescribeTable("Test SetHostConsumerRef",
		func(tc testCaseSetHostSpec) {
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(tc.Host).Build()

			m3mconfig, infrastructureRef := newConfig(tc.UserDataNamespace,
				map[string]string{}, []infrav1.HostSelectorRequirement{}, "",
			)
			machine := newMachine(machineName, infrastructureRef)

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
			Expect(tc.Host.Spec.ConsumerRef.Kind).To(Equal(metal3MachineKind))
			_, err = machineMgr.FindOwnerRef(tc.Host.OwnerReferences)
			Expect(err).NotTo(HaveOccurred())

			if tc.expectNodeReuseLabelDeleted {
				Expect(tc.Host.Labels[nodeReuseLabelName]).To(Equal(""))
			}
		},
		Entry("User data has explicit alternate namespace", testCaseSetHostSpec{
			UserDataNamespace:         "otherns",
			ExpectedUserDataNamespace: "otherns",
			Host: newBareMetalHost("host2", nil, bmov1alpha1.StateNone,
				nil, false, "metadata", false, "",
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("User data has no namespace", testCaseSetHostSpec{
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: namespaceName,
			Host: newBareMetalHost("host2", nil, bmov1alpha1.StateNone,
				nil, false, "metadata", false, "",
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("Externally provisioned, same machine", testCaseSetHostSpec{
			UserDataNamespace:         "",
			ExpectedUserDataNamespace: namespaceName,
			Host: newBareMetalHost("host2", nil, bmov1alpha1.StateNone,
				nil, false, "metadata", false, "",
			),
			ExpectedImage:  expectedImg(),
			ExpectUserData: true,
		}),
		Entry("Previously provisioned, different image",
			testCaseSetHostSpec{
				UserDataNamespace:         "",
				ExpectedUserDataNamespace: namespaceName,
				Host: newBareMetalHost("host2", bmhSpecTestImg(),
					bmov1alpha1.StateNone, nil, false, "metadata", false, "",
				),
				ExpectedImage:  expectedImgTest(),
				ExpectUserData: false,
			},
		),
		Entry("Previously provisioned, different custom deploy",
			testCaseSetHostSpec{
				UserDataNamespace:         "",
				ExpectedUserDataNamespace: namespaceName,
				Host: newBareMetalHost("host2", bmhSpecTestCustomDeploy(),
					bmov1alpha1.StateNone, nil, false, "metadata", false, "",
				),
				ExpectedCustomDeploy: expectedCustomDeployTest(),
				ExpectUserData:       false,
			},
		),
	)

	Describe("Test Exists function", func() {
		host := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "somehost",
				Namespace: namespaceName,
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(&host).Build()

		type testCaseExists struct {
			Machine   *clusterv1.Machine
			M3Machine *infrav1.Metal3Machine
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
				M3Machine: newMetal3Machine(metal3machineName, nil, nil,
					m3mObjectMetaWithSomeAnnotations(),
				),
				Expected: true,
			}),
			Entry("Found host even though annotation value is incorrect",
				testCaseExists{
					Machine: &clusterv1.Machine{},
					M3Machine: newMetal3Machine(metal3machineName, nil, nil,
						m3mObjectMetaWithInvalidAnnotations(),
					),
					Expected: false,
				},
			),
			Entry("Found host even though annotation not present", testCaseExists{
				Machine: &clusterv1.Machine{},
				M3Machine: newMetal3Machine("", nil, nil,
					m3mObjectMetaEmptyAnnotations(),
				),
				Expected: false,
			}),
		)
	})

	Describe("Test GetHost", func() {
		host := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      baremetalhostName,
				Namespace: namespaceName,
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(&host).Build()

		type testCaseGetHost struct {
			Machine       *clusterv1.Machine
			M3Machine     *infrav1.Metal3Machine
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
				M3Machine: newMetal3Machine(metal3machineName, nil, nil,
					m3mObjectMetaWithValidAnnotations(),
				),
				ExpectPresent: true,
			}),
			Entry("Should not find the host, annotation value incorrect",
				testCaseGetHost{
					Machine: &clusterv1.Machine{},
					M3Machine: newMetal3Machine(metal3machineName, nil, nil,
						m3mObjectMetaWithInvalidAnnotations(),
					),
					ExpectPresent: false,
				},
			),
			Entry("Should not find the host, annotation not present", testCaseGetHost{
				Machine: &clusterv1.Machine{},
				M3Machine: newMetal3Machine(metal3machineName, nil, nil,
					m3mObjectMetaEmptyAnnotations(),
				),
				ExpectPresent: false,
			}),
		)
	})

	type testCaseGetSetProviderID struct {
		Machine       *clusterv1.Machine
		M3Machine     *infrav1.Metal3Machine
		Host          *bmov1alpha1.BareMetalHost
		ExpectPresent bool
		ExpectError   bool
	}

	Describe("Test utility functions", func() {
		type testCaseSmallFunctions struct {
			Machine        *clusterv1.Machine
			M3Machine      *infrav1.Metal3Machine
			ExpectCtrlNode bool
		}

		host := bmov1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      baremetalhostName,
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
				Machine: newMachine("", nil),
				M3Machine: newMetal3Machine(metal3machineName, m3mSpec(), nil,
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
						Name:      machineName,
						Namespace: namespaceName,
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "labelHere",
						},
					},
				},
				M3Machine: newMetal3Machine(metal3machineName, m3mSpec(), nil,
					m3mObjectMetaEmptyAnnotations(),
				),
				ExpectCtrlNode: true,
			}),
		)
	})

	type testCaseEnsureAnnotation struct {
		Machine          clusterv1.Machine
		Host             *bmov1alpha1.BareMetalHost
		M3Machine        *infrav1.Metal3Machine
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
			M3Machine: newMetal3Machine(metal3machineName, nil, nil,
				m3mObjectMetaWithValidAnnotations(),
			),
			Host: newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone, nil,
				false, "metadata", false, "",
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotation exists but is wrong", testCaseEnsureAnnotation{
			Machine: clusterv1.Machine{},
			M3Machine: newMetal3Machine(metal3machineName, nil, nil,
				m3mObjectMetaWithInvalidAnnotations(),
			),
			Host: newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone,
				nil, false, "metadata", false, "",
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotations are empty", testCaseEnsureAnnotation{
			Machine: clusterv1.Machine{},
			M3Machine: newMetal3Machine(metal3machineName, nil, nil,
				m3mObjectMetaEmptyAnnotations(),
			),
			Host: newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone,
				nil, false, "metadata", false, "",
			),
			ExpectAnnotation: true,
		}),
		Entry("Annotations are nil", testCaseEnsureAnnotation{
			Machine: clusterv1.Machine{},
			M3Machine: newMetal3Machine("", nil, nil,
				m3mObjectMetaNoAnnotations(),
			),
			Host: newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone,
				nil, false, "metadata", false, "",
			),
			ExpectAnnotation: true,
		}),
	)

	type testCaseDelete struct {
		Host                            *bmov1alpha1.BareMetalHost
		Secret                          *corev1.Secret
		Machine                         *clusterv1.Machine
		M3Machine                       *infrav1.Metal3Machine
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
		Cluster                         *clusterv1.Cluster
		Metal3MachineTemplate           *infrav1.Metal3MachineTemplate
		MachineSet                      *clusterv1.MachineSet
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
			if tc.Metal3MachineTemplate != nil {
				objects = append(objects, tc.Metal3MachineTemplate)
			}
			if tc.MachineSet != nil {
				objects = append(objects, tc.MachineSet)
			}

			Capm3FastTrack = tc.capm3fasttrack

			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()

			machineMgr, err := NewMachineManager(fakeClient, tc.Cluster, nil, tc.Machine,
				tc.M3Machine, logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.Delete(context.TODO())

			if tc.ExpectedResult == nil {
				Expect(err).NotTo(HaveOccurred())
			} else {
				var reconcileError ReconcileError
				Expect(errors.As(err, &reconcileError)).To(BeTrue())
				Expect(reconcileError.IsTransient()).To(BeTrue())
			}

			if tc.Host != nil {
				key := client.ObjectKey{
					Name:      tc.Host.Name,
					Namespace: tc.Host.Namespace,
				}
				host := bmov1alpha1.BareMetalHost{}

				err = fakeClient.Get(context.TODO(), key, &host)
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
				savedHost := bmov1alpha1.BareMetalHost{}
				err = fakeClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.Host.Name,
						Namespace: tc.Host.Namespace,
					},
					&savedHost,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedHost.Annotations[bmov1alpha1.PausedAnnotation]).NotTo(Equal(PausedAnnotationKey))
			}

			if tc.ExpectClusterLabelDeleted {
				// get the saved host
				savedHost := bmov1alpha1.BareMetalHost{}
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
				Expect(savedHost.Labels[clusterv1.ClusterNameLabel]).To(Equal(""))
				Expect(savedCred.Labels[clusterv1.ClusterNameLabel]).To(Equal(""))
				// Other labels are not removed
				Expect(savedHost.Labels["foo"]).To(Equal("bar"))
				Expect(savedCred.Labels["foo"]).To(Equal("bar"))
			}
			if tc.NodeReuseEnabled {
				m3mTemplate := infrav1.Metal3MachineTemplate{}
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
				savedbmh := bmov1alpha1.BareMetalHost{}
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
			Host: newBareMetalHost(baremetalhostName, bmhSpec(),
				bmov1alpha1.StateProvisioned, bmhStatus(), false, "metadata", true, "",
			),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef:     consumerRef(),
			ExpectedResult:          ReconcileError{},
			Secret:                  newSecret(),
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("No Host status, deprovisioning needed", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpec(), bmov1alpha1.StateNone,
				nil, false, "metadata", true, "",
			),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef:     consumerRef(),
			ExpectedResult:          ReconcileError{},
			Secret:                  newSecret(),
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("No Host status, no deprovisioning needed", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpecNoImg(), bmov1alpha1.StateNone, nil,
				false, "metadata", true, "",
			),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: true,
		}),
		Entry("Deprovisioning in progress", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpecNoImg(),
				bmov1alpha1.StateDeprovisioning, bmhStatus(), false, "metadata", true, "",
			),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef: consumerRef(),
			ExpectedResult:      ReconcileError{nil, TransientErrorType, time.Second * 30},
			Secret:              newSecret(),
		}),
		Entry("Externally provisioned host should be powered down", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpecNoImg(),
				bmov1alpha1.StateExternallyProvisioned, bmhPowerStatus(), true, "metadata", true, "",
			),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedConsumerRef: consumerRef(),
			ExpectedResult:      ReconcileError{nil, TransientErrorType, time.Second * 30},
			Secret:              newSecret(),
		}),
		Entry("Consumer ref should be removed from externally provisioned host",
			testCaseDelete{
				Host: newBareMetalHost(baremetalhostName, bmhSpecNoImg(),
					bmov1alpha1.StateExternallyProvisioned, bmhPowerStatus(), false, "metadata", true, "",
				),
				Machine: newMachine(machineName, nil),
				M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
					m3mObjectMetaWithValidAnnotations(),
				),
				Secret:              newSecret(),
				ExpectSecretDeleted: true,
			},
		),
		Entry("Consumer ref should be removed from unmanaged host",
			testCaseDelete{
				Host: newBareMetalHost(baremetalhostName, bmhSpecNoImg(),
					bmov1alpha1.StateUnmanaged, bmhPowerStatus(), false, "metadata", true, "",
				),
				Machine: newMachine(machineName, nil),
				M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
					m3mObjectMetaWithValidAnnotations(),
				),
				Secret:              newSecret(),
				ExpectSecretDeleted: true,
			},
		),
		Entry("Consumer ref should be removed, BMH state is available", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpecNoImg(), bmov1alpha1.StateAvailable,
				bmhStatus(), false, "metadata", true, "",
			),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: true,
		}),
		Entry("Consumer ref should be removed", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpecNoImg(), bmov1alpha1.StateReady,
				bmhStatus(), false, "metadata", true, "",
			),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: true,
		}),
		Entry("Consumer ref should be removed, secret not deleted", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpecNoImg(), bmov1alpha1.StateReady,
				bmhStatus(), false, "metadata", true, "",
			),
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns2",
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("Foobar"),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret: newSecret(),
		}),
		Entry("Consumer ref does not match, so it should not be removed",
			testCaseDelete{
				Host: newBareMetalHost(baremetalhostName, bmhSpecSomeImg(),
					bmov1alpha1.StateProvisioned, bmhStatus(), false, "metadata", true, "",
				),
				Machine: newMachine("", nil),
				M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
					m3mObjectMetaWithValidAnnotations(),
				),
				ExpectedConsumerRef:     consumerRefSome(),
				Secret:                  newSecret(),
				ExpectedBMHOnlineStatus: true,
			},
		),
		Entry("No consumer ref, so this is a no-op", testCaseDelete{
			Host:    newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone, nil, false, "metadata", true, ""),
			Machine: newMachine("", nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: false,
		}),
		Entry("No host at all, so this is a no-op", testCaseDelete{
			Host:    nil,
			Machine: newMachine("", nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: false,
		}),
		Entry("dataSecretName set, deleting secret", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpecNoImg(), bmov1alpha1.StateNone, nil,
				false, "metadata", true, "",
			),
			Machine: &clusterv1.Machine{
				ObjectMeta: testObjectMeta("", namespaceName, ""),
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To(metal3machineName + "-user-data"),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:              newSecret(),
			ExpectSecretDeleted: false,
		}),
		Entry("Clusterlabel should be removed", testCaseDelete{
			Machine:                   newMachine(machineName, nil),
			M3Machine:                 newMetal3Machine(metal3machineName, m3mSpecAll(), m3mSecretStatus(), m3mObjectMetaWithValidAnnotations()),
			Host:                      newBareMetalHost(baremetalhostName, bmhSpecBMC(), bmov1alpha1.StateNone, nil, false, "metadata", true, ""),
			BMCSecret:                 newBMCSecret("mycredentials", true),
			ExpectSecretDeleted:       true,
			ExpectClusterLabelDeleted: true,
		}),
		Entry("PausedAnnotation/CAPM3 should be removed", testCaseDelete{
			Machine:   newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, m3mSpecAll(), m3mSecretStatus(), m3mObjectMetaWithValidAnnotations()),
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: *bmhObjectMetaWithValidCAPM3PausedAnnotations(),
				Spec:       *bmhSpecBMC(),
				Status: bmov1alpha1.BareMetalHostStatus{
					Provisioning: bmov1alpha1.ProvisionStatus{
						State: bmov1alpha1.StateNone,
					},
					PoweredOn: false,
				},
			},
			BMCSecret:                       newBMCSecret("mycredentials", false),
			ExpectSecretDeleted:             true,
			ExpectedPausedAnnotationDeleted: true,
		}),
		Entry("No clusterLabel in BMH or BMC Secret so this is a no-op ", testCaseDelete{
			Machine:                   newMachine(machineName, nil),
			M3Machine:                 newMetal3Machine(metal3machineName, m3mSpecAll(), m3mSecretStatus(), m3mObjectMetaWithValidAnnotations()),
			Host:                      newBareMetalHost(baremetalhostName, bmhSpecBMC(), bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
			BMCSecret:                 newBMCSecret("mycredentials", false),
			ExpectSecretDeleted:       true,
			ExpectClusterLabelDeleted: false,
		}),
		Entry("BMH MetaData, NetworkData and UserData should not be cleaned on deprovisioning", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpecSomeImg(),
				bmov1alpha1.StateProvisioned, bmhStatus(), false, "metadata", true, "",
			),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatusNil(),
				m3mObjectMetaWithValidAnnotations(),
			),
			Secret:                  newSecret(),
			ExpectedConsumerRef:     consumerRefSome(),
			capm3fasttrack:          "false",
			ExpectedBMHOnlineStatus: true,
		}),
		Entry("Capm3FastTrack is set to false, AutomatedCleaning mode is set to metadata, set bmh online field to false", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpec(),
				bmov1alpha1.StateDeprovisioning, bmhStatus(), false, "metadata", true, ""),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          ReconcileError{},
			ExpectedConsumerRef:     consumerRef(),
			Secret:                  newSecret(),
			capm3fasttrack:          "false",
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("Capm3FastTrack is set to true, AutomatedCleaning mode is set to metadata, set bmh online field to true", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpec(),
				bmov1alpha1.StateDeprovisioning, bmhStatus(), false, "metadata", true, ""),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          ReconcileError{},
			ExpectedConsumerRef:     consumerRef(),
			Secret:                  newSecret(),
			capm3fasttrack:          "true",
			ExpectedBMHOnlineStatus: true,
		}),
		Entry("Capm3FastTrack is set to false, AutomatedCleaning mode is set to disabled, set bmh online field to false", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpec(),
				bmov1alpha1.StateDeprovisioning, bmhStatus(), false, "disabled", true, ""),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          ReconcileError{},
			ExpectedConsumerRef:     consumerRef(),
			Secret:                  newSecret(),
			capm3fasttrack:          "false",
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("Capm3FastTrack is set to true, AutomatedCleaning mode is set to disabled, set bmh online field to false", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpec(),
				bmov1alpha1.StateDeprovisioning, bmhStatus(), false, "disabled", true, ""),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          ReconcileError{},
			ExpectedConsumerRef:     consumerRef(),
			capm3fasttrack:          "true",
			Secret:                  newSecret(),
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("Capm3FastTrack is empty, AutomatedCleaning mode is set to disabled, set bmh online field to false", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpec(),
				bmov1alpha1.StateDeprovisioning, bmhStatus(), false, "disabled", true, ""),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          ReconcileError{},
			ExpectedConsumerRef:     consumerRef(),
			capm3fasttrack:          "",
			Secret:                  newSecret(),
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("Capm3FastTrack is empty, AutomatedCleaning mode is set to metadata, set bmh online field to false", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName, bmhSpec(),
				bmov1alpha1.StateDeprovisioning, bmhStatus(), false, "metadata", true, ""),
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				m3mObjectMetaWithValidAnnotations(),
			),
			ExpectedResult:          ReconcileError{},
			ExpectedConsumerRef:     consumerRef(),
			capm3fasttrack:          "",
			Secret:                  newSecret(),
			ExpectedBMHOnlineStatus: false,
		}),
		Entry("NodeReuse enabled, machine is worker, no error expected", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName,
				&bmov1alpha1.BareMetalHostSpec{
					ConsumerRef: consumerRef(),
					Online:      false,
				},
				bmov1alpha1.StateNone,
				&bmov1alpha1.BareMetalHostStatus{
					Provisioning: bmov1alpha1.ProvisionStatus{
						State: bmov1alpha1.StateNone,
					},
				}, false, "metadata", true, ""),
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      machineName,
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Name:       "myMachineSet",
							UID:        "123456789",
							Kind:       "MachineSet",
						},
					},
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To(metal3machineName + "-user-data")},
				},
			},
			MachineSet: &clusterv1.MachineSet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "MachineSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myMachineSet",
					Namespace: namespaceName,
					UID:       "123456789",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "MachineDeployment",
							Name:       "test1",
						},
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				&metav1.ObjectMeta{
					Name:      metal3machineName,
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "MachineSet",
							APIVersion: infrav1.GroupVersion.String(),
							Name:       "test1",
							UID:        "123456789",
						},
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "test1-bwjsg",
							UID:        "070f03fb-b0e1-4863-9d95-7fd681d8d257",
						},
					},
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: clusterName,
					},
					Annotations: map[string]string{
						HostAnnotation:                           namespaceName + "/" + baremetalhostName,
						"cluster.x-k8s.io/cloned-from-name":      "abc",
						"cluster.x-k8s.io/cloned-from-groupkind": infrav1.ClonedFromGroupKind,
					},
				}),
			Cluster:             newCluster(clusterName),
			Secret:              newSecret(),
			ExpectSecretDeleted: false,
			Metal3MachineTemplate: &infrav1.Metal3MachineTemplate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "Metal3MachineTemplate",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
				Spec: infrav1.Metal3MachineTemplateSpec{
					Template: infrav1.Metal3MachineTemplateResource{
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To(infrav1.CleaningModeDisabled),
						},
					},
					NodeReuse: true,
				}},
		}),
		Entry("NodeReuse enabled, machine is controlplane, no error expected", testCaseDelete{
			Host: newBareMetalHost(baremetalhostName,
				&bmov1alpha1.BareMetalHostSpec{
					ConsumerRef: consumerRef(),
					Online:      false,
				},
				bmov1alpha1.StateNone,
				&bmov1alpha1.BareMetalHostStatus{
					Provisioning: bmov1alpha1.ProvisionStatus{
						State: bmov1alpha1.StateNone,
					},
				}, false, "metadata", false, ""),
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      machineName,
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "controlplane.cluster.x-k8s.io/v1beta2",
							Name:       "test1",
							UID:        "123456789",
							Kind:       "KubeadmControlPlane",
						},
					},
					Labels: map[string]string{
						clusterv1.MachineControlPlaneLabel: "cluster.x-k8s.io/control-plane",
					},
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To(metal3machineName + "-user-data")},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, nil, m3mSecretStatus(),
				&metav1.ObjectMeta{
					Name:      metal3machineName,
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "KubeadmControlPlane",
							APIVersion: infrav1.GroupVersion.String(),
							Name:       "test1",
							UID:        "2a72b7d2-b5bd-4074-a1fe-7d417bdcd95b",
						},
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "test1-bwjsg",
							UID:        "070f03fb-b0e1-4863-9d95-7fd681d8d257",
						},
					},
					Labels: map[string]string{
						clusterv1beta1.ClusterNameLabel: clusterName,
					},
					Annotations: map[string]string{
						HostAnnotation:                           namespaceName + "/" + baremetalhostName,
						"cluster.x-k8s.io/cloned-from-name":      "abc",
						"cluster.x-k8s.io/cloned-from-groupkind": infrav1.ClonedFromGroupKind,
					},
				}),
			Cluster:             newCluster(clusterName),
			Secret:              newSecret(),
			ExpectSecretDeleted: false,
			Metal3MachineTemplate: &infrav1.Metal3MachineTemplate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "Metal3MachineTemplate",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
				Spec: infrav1.Metal3MachineTemplateSpec{
					Template: infrav1.Metal3MachineTemplateResource{
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To(infrav1.CleaningModeDisabled),
						},
					},
					NodeReuse: true,
				}},
		}),
	)

	Describe("Test UpdateMachineStatus", func() {
		nic1 := bmov1alpha1.NIC{
			IP: "192.168.1.1",
		}

		nic2 := bmov1alpha1.NIC{
			IP: "172.0.20.2",
		}

		type testCaseUpdateMachineStatus struct {
			Host            *bmov1alpha1.BareMetalHost
			Machine         *clusterv1.Machine
			ExpectedMachine clusterv1.Machine
			M3Machine       infrav1.Metal3Machine
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
				m3machine := infrav1.Metal3Machine{}
				err = fakeClient.Get(context.TODO(), key, &m3machine)
				Expect(err).NotTo(HaveOccurred())

				if tc.M3Machine.Status.Addresses != nil {
					for i, address := range tc.ExpectedMachine.Status.Addresses {
						Expect(m3machine.Status.Addresses[i].Address).To(Equal(address.Address))
					}
				}
			},
			Entry("Machine status updated", testCaseUpdateMachineStatus{
				Host: &bmov1alpha1.BareMetalHost{
					Status: bmov1alpha1.BareMetalHostStatus{
						HardwareDetails: &bmov1alpha1.HardwareDetails{
							NIC: []bmov1alpha1.NIC{nic1, nic2},
						},
					},
				},
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      machineName,
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
				M3Machine: infrav1.Metal3Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      metal3machineName,
						Namespace: namespaceName,
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       metal3MachineKind,
						APIVersion: clusterv1beta1.GroupVersion.String(),
					},
					Status: infrav1.Metal3MachineStatus{
						Addresses: []clusterv1beta1.MachineAddress{
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
				Host: &bmov1alpha1.BareMetalHost{
					Status: bmov1alpha1.BareMetalHostStatus{
						HardwareDetails: &bmov1alpha1.HardwareDetails{
							NIC: []bmov1alpha1.NIC{nic1, nic2},
						},
					},
				},
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      machineName,
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
				M3Machine: infrav1.Metal3Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      metal3machineName,
						Namespace: namespaceName,
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       metal3MachineKind,
						APIVersion: clusterv1beta1.GroupVersion.String(),
					},
					Status: infrav1.Metal3MachineStatus{
						Addresses: []clusterv1beta1.MachineAddress{
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
					Host: &bmov1alpha1.BareMetalHost{},
					Machine: &clusterv1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      machineName,
							Namespace: namespaceName,
						},
						Status: clusterv1.MachineStatus{},
					},
					M3Machine: infrav1.Metal3Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      metal3machineName,
							Namespace: namespaceName,
						},
						TypeMeta: metav1.TypeMeta{
							Kind:       metal3MachineKind,
							APIVersion: clusterv1beta1.GroupVersion.String(),
						},
						Status: infrav1.Metal3MachineStatus{
							Addresses: []clusterv1beta1.MachineAddress{},
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
		nic1 := bmov1alpha1.NIC{
			IP: "192.168.1.1",
		}

		nic2 := bmov1alpha1.NIC{
			IP: "172.0.20.2",
		}

		addr1 := clusterv1beta1.MachineAddress{
			Type:    clusterv1beta1.MachineInternalIP,
			Address: "192.168.1.1",
		}

		addr2 := clusterv1beta1.MachineAddress{
			Type:    clusterv1beta1.MachineInternalIP,
			Address: "172.0.20.2",
		}

		addr3 := clusterv1beta1.MachineAddress{
			Type:    clusterv1beta1.MachineHostName,
			Address: "mygreathost",
		}

		type testCaseNodeAddress struct {
			Machine               clusterv1.Machine
			M3Machine             infrav1.Metal3Machine
			Host                  *bmov1alpha1.BareMetalHost
			ExpectedNodeAddresses []clusterv1beta1.MachineAddress
		}

		DescribeTable("Test NodeAddress",
			func(tc testCaseNodeAddress) {
				var nodeAddresses []clusterv1beta1.MachineAddress

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
				Host: &bmov1alpha1.BareMetalHost{
					Status: bmov1alpha1.BareMetalHostStatus{
						HardwareDetails: &bmov1alpha1.HardwareDetails{
							NIC: []bmov1alpha1.NIC{nic1},
						},
					},
				},
				ExpectedNodeAddresses: []clusterv1beta1.MachineAddress{addr1},
			}),
			Entry("Two NICs", testCaseNodeAddress{
				Host: &bmov1alpha1.BareMetalHost{
					Status: bmov1alpha1.BareMetalHostStatus{
						HardwareDetails: &bmov1alpha1.HardwareDetails{
							NIC: []bmov1alpha1.NIC{nic1, nic2},
						},
					},
				},
				ExpectedNodeAddresses: []clusterv1beta1.MachineAddress{addr1, addr2},
			}),
			Entry("Hostname is set", testCaseNodeAddress{
				Host: &bmov1alpha1.BareMetalHost{
					Status: bmov1alpha1.BareMetalHostStatus{
						HardwareDetails: &bmov1alpha1.HardwareDetails{
							Hostname: "mygreathost",
						},
					},
				},
				ExpectedNodeAddresses: []clusterv1beta1.MachineAddress{addr3},
			}),
			Entry("Empty Hostname", testCaseNodeAddress{
				Host: &bmov1alpha1.BareMetalHost{
					Status: bmov1alpha1.BareMetalHostStatus{
						HardwareDetails: &bmov1alpha1.HardwareDetails{
							Hostname: "",
						},
					},
				},
				ExpectedNodeAddresses: []clusterv1beta1.MachineAddress{},
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
			m3m := infrav1.Metal3Machine{
				Spec: infrav1.Metal3MachineSpec{
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
			providerID:    ptr.To(ProviderID),
			expectedBMHID: string(Bmhuid),
		}),
	)

	Describe("Test SetNodeProviderID", func() {
		s := runtime.NewScheme()
		err := clusterv1beta1.AddToScheme(s)
		if err != nil {
			log.Printf("AddToScheme failed: %v", err)
		}
		err = bmov1alpha1.AddToScheme(s)
		if err != nil {
			log.Printf("AddToScheme failed: %v", err)
		}

		type testCaseSetNodePoviderID struct {
			TargetObjects        []runtime.Object
			M3MHasHostAnnotation bool
			HostID               string
			ExpectedError        bool
			ExpectedProviderID   string
		}
	})

	type testCaseGetUserDataSecretName struct {
		Machine     *clusterv1.Machine
		M3Machine   *infrav1.Metal3Machine
		BMHost      *bmov1alpha1.BareMetalHost
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

			machineMgr.getUserDataSecretName(context.TODO())

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
				ObjectMeta: testObjectMeta("", namespaceName, ""),
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("Foobar"),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, nil, nil, nil),
			BMHost:    newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
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
						DataSecretName: ptr.To("Foobar"),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, nil, nil, nil),
			BMHost:    newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
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
						ConfigRef: clusterv1.ContractVersionedObjectReference{
							Name: "abc",
						},
						DataSecretName: ptr.To("Foobar"),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, nil, nil, nil),
			BMHost:    newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
		}),
		Entry("UserDataSecretName set in Machine, secret exists", testCaseGetUserDataSecretName{
			Secret: newSecret(),
			Machine: &clusterv1.Machine{
				ObjectMeta: testObjectMeta("", namespaceName, ""),
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("test-data-secret-name"),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, nil, nil, nil),
			BMHost:    newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
		}),
		Entry("UserDataSecretName set in Machine, no secret", testCaseGetUserDataSecretName{
			Machine: &clusterv1.Machine{
				ObjectMeta: testObjectMeta("", namespaceName, ""),
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("test-data-secret-name"),
					},
				},
			},
			M3Machine: newMetal3Machine(metal3machineName, nil, nil, nil),
			BMHost:    newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
		}),
	)

	type testCaseAssociate struct {
		Machine            *clusterv1.Machine
		Host               *bmov1alpha1.BareMetalHost
		M3Machine          *infrav1.Metal3Machine
		BMCSecret          *corev1.Secret
		DataTemplate       *infrav1.Metal3DataTemplate
		Data               *infrav1.Metal3Data
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
				var reconcileError ReconcileError
				ok := errors.As(err, &reconcileError)
				log.Println(RootCause(err))
				Expect(ok).To(BeTrue())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			if tc.Host == nil {
				return
			}
			// get the saved host
			savedHost := bmov1alpha1.BareMetalHost{}
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
				Expect(savedHost.Labels[clusterv1beta1.ClusterNameLabel]).To(Equal(tc.Machine.Spec.ClusterName))
				Expect(savedCred.Labels[clusterv1beta1.ClusterNameLabel]).To(Equal(tc.Machine.Spec.ClusterName))
			}
		},
		Entry("Associate empty machine, Metal3 machine spec nil",
			testCaseAssociate{
				Machine: newMachine("", nil),
				M3Machine: newMetal3Machine(metal3machineName, nil, nil,
					m3mObjectMetaWithValidAnnotations(),
				),
				Host: newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone, nil,
					false, "metadata", false, "",
				),
				ExpectRequeue:  false,
				ExpectOwnerRef: true,
			},
		),
		Entry("Associate empty machine, Metal3 machine spec set",
			testCaseAssociate{
				Machine: newMachine("", nil),
				M3Machine: newMetal3Machine(metal3machineName, m3mSpecAll(), nil,
					m3mObjectMetaWithValidAnnotations(),
				),
				Host: newBareMetalHost(baremetalhostName, bmhSpecBMC(), bmov1alpha1.StateNone, nil,
					false, "metadata", false, "",
				),
				BMCSecret:      newBMCSecret("mycredentials", false),
				ExpectRequeue:  false,
				ExpectOwnerRef: true,
			},
		),
		Entry("Associate empty machine, host empty, Metal3 machine spec set",
			testCaseAssociate{
				Machine: newMachine("", nil),
				M3Machine: newMetal3Machine(metal3machineName, m3mSpecAll(), nil,
					m3mObjectMetaWithValidAnnotations(),
				),
				Host:           newBareMetalHost("", nil, bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
				ExpectRequeue:  true,
				ExpectOwnerRef: false,
			},
		),
		Entry("Associate machine, host nil, Metal3 machine spec set, requeue",
			testCaseAssociate{
				Machine: newMachine("myUniqueMachine", nil),
				M3Machine: newMetal3Machine(metal3machineName, m3mSpecAll(), nil,
					m3mObjectMetaWithValidAnnotations(),
				),
				Host:          nil,
				ExpectRequeue: true,
			},
		),
		Entry("Associate machine, host set, Metal3 machine spec set, set clusterLabel",
			testCaseAssociate{
				Machine:            newMachine(machineName, nil),
				M3Machine:          newMetal3Machine(metal3machineName, m3mSpecAll(), nil, nil),
				Host:               newBareMetalHost(baremetalhostName, bmhSpecBMC(), bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
				BMCSecret:          newBMCSecret("mycredentials", false),
				ExpectClusterLabel: true,
				ExpectRequeue:      false,
				ExpectOwnerRef:     true,
			},
		),
		Entry("Associate machine with DataTemplate missing",
			testCaseAssociate{
				Machine: newMachine(machineName, nil),
				M3Machine: newMetal3Machine(metal3machineName,
					&infrav1.Metal3MachineSpec{
						DataTemplate: &corev1.ObjectReference{
							Name:      "abcd",
							Namespace: namespaceName,
						},
						UserData: &corev1.SecretReference{
							Name:      metal3machineName + "-user-data",
							Namespace: namespaceName,
						},
						Image: infrav1.Image{
							URL:        testImageURL,
							Checksum:   testImageChecksumURL,
							DiskFormat: testImageDiskFormat,
						},
					}, nil, nil,
				),
				Host:               newBareMetalHost(baremetalhostName, bmhSpecBMC(), bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
				BMCSecret:          newBMCSecret("mycredentials", false),
				ExpectClusterLabel: true,
				ExpectRequeue:      false,
				ExpectOwnerRef:     true,
			},
		),
		Entry("Associate machine with DataTemplate and Data ready",
			testCaseAssociate{
				Machine: newMachine(machineName, nil),
				M3Machine: newMetal3Machine(metal3machineName,
					&infrav1.Metal3MachineSpec{
						DataTemplate: &corev1.ObjectReference{
							Name:      "abcd",
							Namespace: namespaceName,
						},
						UserData: &corev1.SecretReference{
							Name:      metal3machineName + "-user-data",
							Namespace: namespaceName,
						},
						Image: infrav1.Image{
							URL:        testImageURL,
							Checksum:   testImageChecksumURL,
							DiskFormat: testImageDiskFormat,
						},
					}, &infrav1.Metal3MachineStatus{
						RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: namespaceName},
					}, nil,
				),
				Host:      newBareMetalHost(baremetalhostName, bmhSpecBMC(), bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
				BMCSecret: newBMCSecret("mycredentials", false),
				Data: &infrav1.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcd-0",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataSpec{
						MetaData: &corev1.SecretReference{
							Name: "metadata",
						},
						NetworkData: &corev1.SecretReference{
							Name: "networkdata",
						},
					},
					Status: infrav1.Metal3DataStatus{
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
		Host        *bmov1alpha1.BareMetalHost
		M3Machine   *infrav1.Metal3Machine
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
				Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Update machine", testCaseUpdate{
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, nil, nil,
				m3mObjectMetaWithValidAnnotations(),
			),
			Host: newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
		}),
		Entry("Update machine, DataTemplate missing", testCaseUpdate{
			Machine: newMachine(machineName, nil),
			M3Machine: newMetal3Machine(metal3machineName, &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name: "abcd",
				},
			}, nil,
				m3mObjectMetaWithValidAnnotations(),
			),
			Host:        newBareMetalHost(baremetalhostName, nil, bmov1alpha1.StateNone, nil, false, "metadata", false, ""),
			ExpectError: true,
		}),
	)

	type testCaseFindOwnerRef struct {
		M3Machine     infrav1.Metal3Machine
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
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(&NotFoundError{}))
			} else {
				Expect(err).ToNot(HaveOccurred())
				Expect(index).To(BeEquivalentTo(tc.ExpectedIndex))
			}
		},
		Entry("Empty list", testCaseFindOwnerRef{
			M3Machine:   *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs:   []metav1.OwnerReference{},
			ExpectError: true,
		}),
		Entry("Absent", testCaseFindOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
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
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					Kind:       metal3MachineKind,
					APIVersion: infrav1.GroupVersion.String(),
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
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				{
					Kind:       metal3MachineKind,
					APIVersion: infrav1.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
			ExpectError:   false,
			ExpectedIndex: 1,
		}),
		Entry("Present but different versions", testCaseFindOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				{
					Kind:       metal3MachineKind,
					APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
			ExpectError:   false,
			ExpectedIndex: 1,
		}),
		Entry("Wrong group", testCaseFindOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				{
					Kind:       metal3MachineKind,
					APIVersion: "nfrastructure.cluster.x-k8s.io/v1alpha1",
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
			ExpectError: true,
		}),
	)

	type testCaseOwnerRef struct {
		M3Machine  infrav1.Metal3Machine
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
			Expect(err).ToNot(HaveOccurred())
			_, err = machineMgr.FindOwnerRef(refList)
			Expect(err).To(HaveOccurred())
		},
		Entry("Empty list", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{},
		}),
		Entry("Absent", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
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
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					Kind:       metal3MachineKind,
					APIVersion: infrav1.GroupVersion.String(),
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
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				{
					Kind:       metal3MachineKind,
					APIVersion: infrav1.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					Kind:       metal3MachineKind,
					APIVersion: infrav1.GroupVersion.String(),
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
			Expect(err).ToNot(HaveOccurred())
			index, err := machineMgr.FindOwnerRef(refList)
			Expect(err).ToNot(HaveOccurred())
			Expect(*refList[index].Controller).To(BeEquivalentTo(tc.Controller))
		},
		Entry("Empty list", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{},
		}),
		Entry("Absent", testCaseOwnerRef{
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
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
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					Kind:       metal3MachineKind,
					APIVersion: infrav1.GroupVersion.String(),
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
			M3Machine: *newMetal3Machine("myName", nil, nil, nil),
			OwnerRefs: []metav1.OwnerReference{
				{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
				{
					Kind:       metal3MachineKind,
					APIVersion: infrav1.GroupVersion.String(),
					Name:       "myName",
					UID:        "adfasdf",
				},
			},
		}),
	)

	type testCaseM3MetaData struct {
		M3Machine                            *infrav1.Metal3Machine
		Machine                              *clusterv1.Machine
		DataClaim                            *infrav1.Metal3DataClaim
		Data                                 *infrav1.Metal3Data
		ExpectError                          bool
		ExpectRequeue                        bool
		ExpectDataStatus                     bool
		ExpectMetal3DataReadyCondition       bool
		ExpectMetal3DataReadyConditionStatus bool
		ExpectSecretStatus                   bool
		expectClaim                          bool
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
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
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
				dataTemplate := infrav1.Metal3DataClaim{}
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
			M3Machine: newMetal3Machine("myName", nil, nil, nil),
		}),
		Entry("MetaData and NetworkData should be set in spec", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				MetaData:    &corev1.SecretReference{Name: "abcd"},
				NetworkData: &corev1.SecretReference{Name: "defg"},
			}, nil, nil),
		}),
		Entry("RenderedData should be set in status", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &infrav1.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd"},
			}, nil),
		}),
		Entry("Should expect DataClaim if it does not exist yet", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine:     newMachine(machineName, nil),
			expectClaim: true,
		}),
		Entry("Should not be an error if DataClaim exists", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine(machineName, nil),
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "myName",
							Kind:       metal3MachineKind,
							APIVersion: infrav1.GroupVersion.String(),
						},
					},
				},
			},
			expectClaim: true,
		}),
		Entry("Should not be an error if DataClaim is empty", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine:     newMachine(machineName, nil),
			DataClaim:   &infrav1.Metal3DataClaim{},
			expectClaim: true,
		}),
		Entry("Should expect claim if DataClaim ready", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine(machineName, nil),
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "myName",
							Kind:       metal3MachineKind,
							APIVersion: infrav1.GroupVersion.String(),
						},
					},
				},
				Status: infrav1.Metal3DataClaimStatus{
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
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
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
			metal3DataReadyCondition := filterCondition(tc.M3Machine.Status.Conditions, infrav1.Metal3DataReadyCondition)
			if tc.ExpectMetal3DataReadyCondition {
				Expect(metal3DataReadyCondition).To(HaveLen(1))
				if tc.ExpectMetal3DataReadyConditionStatus {
					Expect(metal3DataReadyCondition[0].Status).To(Equal(corev1.ConditionTrue))
				} else {
					Expect(metal3DataReadyCondition[0].Status).To(Equal(corev1.ConditionFalse))
				}
			} else {
				Expect(metal3DataReadyCondition).To(BeEmpty())
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
			M3Machine:                      newMetal3Machine("myName", nil, nil, nil),
			Machine:                        nil,
			ExpectMetal3DataReadyCondition: false,
		}),
		Entry("Should requeue if there is no Data template", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine:                        newMachine(machineName, nil),
			ExpectRequeue:                  true,
			ExpectMetal3DataReadyCondition: false,
		}),
		Entry("Should requeue if Data claim without status", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine(machineName, nil),
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
				},
			},
			ExpectRequeue: true,
		}),
		Entry("Should requeue if Data claim with empty status", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine(machineName, nil),
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
				},
				Status: infrav1.Metal3DataClaimStatus{
					RenderedData: &corev1.ObjectReference{},
				},
			},
			ExpectRequeue: true,
		}),
		Entry("Should requeue if Data does not exist", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{Name: "abcd"},
			}, nil, nil),
			Machine: newMachine(machineName, nil),
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
				},
				Spec: infrav1.Metal3DataClaimSpec{
					Template: corev1.ObjectReference{
						Name:      "abcd",
						Namespace: namespaceName,
					},
				},
				Status: infrav1.Metal3DataClaimStatus{
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
			M3Machine: newMetal3Machine("myName", nil, &infrav1.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: namespaceName},
			}, nil),
			Machine:          newMachine(machineName, nil),
			ExpectRequeue:    true,
			ExpectDataStatus: true,
		}),
		Entry("Should requeue if Data is not ready", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &infrav1.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: namespaceName},
			}, nil),
			Machine: newMachine(machineName, nil),
			Data: &infrav1.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abcd-0",
					Namespace: namespaceName,
				},
			},
			ExpectRequeue:                        true,
			ExpectDataStatus:                     true,
			ExpectMetal3DataReadyCondition:       true,
			ExpectMetal3DataReadyConditionStatus: false,
		}),
		Entry("Should not error if Data is ready but no secrets", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &infrav1.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: namespaceName},
			}, nil),
			Machine: newMachine(machineName, nil),
			Data: &infrav1.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abcd-0",
					Namespace: namespaceName,
				},
				Status: infrav1.Metal3DataStatus{
					Ready: true,
				},
			},
			ExpectDataStatus:                     true,
			ExpectSecretStatus:                   true,
			ExpectMetal3DataReadyCondition:       true,
			ExpectMetal3DataReadyConditionStatus: true,
		}),
		Entry("Should not error if Data is ready with secrets", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", nil, &infrav1.Metal3MachineStatus{
				RenderedData: &corev1.ObjectReference{Name: "abcd-0", Namespace: namespaceName},
			}, nil),
			Machine: newMachine(machineName, nil),
			Data: &infrav1.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abcd-0",
					Namespace: namespaceName,
				},
				Spec: infrav1.Metal3DataSpec{
					MetaData: &corev1.SecretReference{
						Name: "metadata",
					},
					NetworkData: &corev1.SecretReference{
						Name: "networkdata",
					},
				},
				Status: infrav1.Metal3DataStatus{
					Ready: true,
				},
			},
			ExpectDataStatus:                     true,
			ExpectSecretStatus:                   true,
			ExpectMetal3DataReadyCondition:       true,
			ExpectMetal3DataReadyConditionStatus: true,
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
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
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
			dataTemplate := infrav1.Metal3DataClaim{}
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
			M3Machine: newMetal3Machine("myName", nil, &infrav1.Metal3MachineStatus{
				MetaData:    &corev1.SecretReference{Name: "abcd"},
				NetworkData: &corev1.SecretReference{Name: "defg"},
			}, nil),
			Machine: newMachine(machineName, nil),
		}),
		Entry("MetaData and NetworkData should be set in spec and status", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				MetaData:    &corev1.SecretReference{Name: "abcd"},
				NetworkData: &corev1.SecretReference{Name: "defg"},
			}, &infrav1.Metal3MachineStatus{
				MetaData:    &corev1.SecretReference{Name: "abcd"},
				NetworkData: &corev1.SecretReference{Name: "defg"},
			}, nil),
			Machine:            newMachine(machineName, nil),
			ExpectSecretStatus: true,
		}),
		Entry("Should return nil if DataClaim not found", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name:      "abc",
					Namespace: namespaceName,
				},
			}, nil, nil),
			Machine: newMachine(machineName, nil),
		}),
		Entry("Should not error if DataClaim found", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name:      "abcd",
					Namespace: namespaceName,
				},
			}, nil, nil),
			Machine: newMachine(machineName, nil),
			DataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myName",
					Namespace: namespaceName,
				},
			},
		}),
		Entry("Should return nil if DataClaim is empty", testCaseM3MetaData{
			M3Machine: newMetal3Machine("myName", &infrav1.Metal3MachineSpec{
				DataTemplate: &corev1.ObjectReference{
					Name:      "abc",
					Namespace: namespaceName,
				},
			}, nil, nil),
			Machine:   newMachine(machineName, nil),
			DataClaim: &infrav1.Metal3DataClaim{},
		}),
	)

	type testCaseNodeReuseLabelMatches struct {
		Machine                  *clusterv1.Machine
		Host                     *bmov1alpha1.BareMetalHost
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
			Host: &bmov1alpha1.BareMetalHost{
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
						clusterv1beta1.MachineControlPlaneLabel: "cluster.x-k8s.io/control-plane",
					},
				},
			},
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cp-test1",
					Labels: map[string]string{
						nodeReuseLabelName: "cp-test1",
						"foo":              "bar",
					},
				},
			},
			expectNodeReuseLabel:     true,
			expectNodeReuseLabelName: "cp-test1",
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
						clusterv1beta1.MachineControlPlaneLabel: "cluster.x-k8s.io/control-plane",
					},
				},
			},
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cp-test1",
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
						clusterv1beta1.MachineControlPlaneLabel: "cluster.x-k8s.io/control-plane",
					},
				},
			},
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cp-test1",
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
						clusterv1beta1.MachineControlPlaneLabel: "",
					},
				},
			},
			Host: &bmov1alpha1.BareMetalHost{
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
						clusterv1beta1.MachineDeploymentNameLabel: "cluster.x-k8s.io/deployment-name",
					},
				},
			},
			Host: &bmov1alpha1.BareMetalHost{
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
		Host                 *bmov1alpha1.BareMetalHost
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
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						nodeReuseLabelName: cpName,
						"foo":              "bar",
					},
				},
			},
			expectNodeReuseLabel: true,
		}),
		Entry("Node reuse label does not exist on the host", testCaseNodeReuseLabelExists{
			Host: &bmov1alpha1.BareMetalHost{
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
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
			},
			expectNodeReuseLabel: false,
		}),
	)

	type testCaseGetControlPlaneName struct {
		Machine        *clusterv1.Machine
		expectedCp     bool
		expectedCpName string
		expectError    bool
	}

	DescribeTable("Test getControlPlaneName",
		func(tc testCaseGetControlPlaneName) {
			objects := []client.Object{}
			if tc.Machine != nil {
				objects = append(objects, tc.Machine)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			machineMgr, err := NewMachineManager(fakeClient, nil, nil, tc.Machine,
				nil, logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			result, err := machineMgr.getControlPlaneName(context.TODO())
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			if tc.expectedCp {
				Expect(result).To(Equal(tc.expectedCpName))
			}

		},
		Entry("Should find the expected cp", testCaseGetControlPlaneName{
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
			expectError:    false,
			expectedCp:     true,
			expectedCpName: "cp-test1",
		}),
		Entry("Should find the expected ControlPlane for any Kind/Provider", testCaseGetControlPlaneName{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: controlplanev1.GroupVersion.String(),
							Kind:       "AnyControlPlane",
							Name:       "test1",
						},
					},
				},
			},
			expectError:    false,
			expectedCp:     true,
			expectedCpName: "cp-test1",
		}),
		Entry("Should not find the expected cp, API version is not correct", testCaseGetControlPlaneName{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: infrav1.GroupVersion.String(),
							Kind:       "KubeadmControlPlane",
							Name:       "test1",
						},
					},
				},
			},
			expectError: true,
		}),
		Entry("Should not find the expected cp and error when machine is nil", testCaseGetControlPlaneName{
			Machine:     nil,
			expectError: true,
		}),
		Entry("Should give an error if Machine.ObjectMeta.OwnerReferences is nil", testCaseGetControlPlaneName{
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
									APIVersion: infrav1.GroupVersion.String(),
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
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: testObjectMeta("ms-test1", "", ""),
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
						clusterv1beta1.MachineControlPlaneLabel: "cluster.x-k8s.io/control-plane",
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
							APIVersion: infrav1.GroupVersion.String(),
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

	type testCaseDuplicateProviderIDsExist struct {
		Machine          *clusterv1.Machine
		validNodes       map[string][]corev1.Node
		providerIDLegacy string
		providerIDNew    string
		expectError      bool
	}

	DescribeTable("Test duplicateProviderIDsExist",
		func(tc testCaseDuplicateProviderIDsExist) {
			machineMgr, err := NewMachineManager(nil, nil, nil, tc.Machine,
				nil, logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = machineMgr.duplicateProviderIDsExist(tc.validNodes, tc.providerIDLegacy, tc.providerIDNew)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Should find duplicates if both providerIDNew and providerIDLegacy are in use by multiple nodes", testCaseDuplicateProviderIDsExist{
			validNodes: map[string][]corev1.Node{
				"metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}},
				"metal3://metal3/node-0/test-controlplane-xyz":  {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}},
			},
			providerIDLegacy: "metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6",
			providerIDNew:    "metal3://metal3/node-0/test-controlplane-xyz",
			expectError:      true,
		}),
		Entry("Should find duplicates if validNodes node count is more than 1 for a providerID", testCaseDuplicateProviderIDsExist{
			validNodes: map[string][]corev1.Node{
				"metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}, corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}},
			},
			providerIDLegacy: "",
			providerIDNew:    "",
			expectError:      true,
		}),
		Entry("Should not find duplicates if ether providerIDNew or providerIDLegacy are in use by single node", testCaseDuplicateProviderIDsExist{
			validNodes: map[string][]corev1.Node{
				"metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}},
			},
			providerIDLegacy: "metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6",
			providerIDNew:    "metal3://metal3/node-0/test-controlplane-xyz",
			expectError:      false,
		}),
		Entry("Should not find duplicates if both providerIDNew and providerIDLegacy are in use by single node", testCaseDuplicateProviderIDsExist{
			validNodes: map[string][]corev1.Node{
				"metal3://metal3/node-0/test-controlplane-xyz": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}},
			},
			providerIDLegacy: "metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6",
			providerIDNew:    "metal3://metal3/node-0/test-controlplane-xyz",
			expectError:      false,
		}),
		Entry("Should not find duplicates if validNodes is nil", testCaseDuplicateProviderIDsExist{
			validNodes:       nil,
			providerIDLegacy: "metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6",
			providerIDNew:    "metal3://metal3/node-0/test-controlplane-xyz",
			expectError:      false,
		}),
		Entry("Should find duplicates if providerIDNew's common prefix metal3://<namespace>/<bmh>/  is a substring of multible validNodes providerIDs", testCaseDuplicateProviderIDsExist{
			validNodes: map[string][]corev1.Node{
				"metal3://metal3/node-0/test-controlplane-xyz": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}},
				"metal3://metal3/node-0/test-controlplane-123": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}},
			},
			providerIDLegacy: "",
			providerIDNew:    "metal3://metal3/node-0/test-controlplane-000",
			expectError:      true,
		}),
		Entry("Should find duplicates if providerIDNew's common prefix metal3://<namespace>/<bmh>/  is a substring of validNodes providerID and providerIDLegacy is used", testCaseDuplicateProviderIDsExist{
			validNodes: map[string][]corev1.Node{
				"metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}},
				"metal3://metal3/node-0/test-controlplane-123":  {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}},
			},
			providerIDLegacy: "metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6",
			providerIDNew:    "metal3://metal3/node-0/test-controlplane-000",
			expectError:      true,
		}),
		Entry("Should not find duplicates if there are overlapping names of validNodes providerID in the legacy format", testCaseDuplicateProviderIDsExist{
			validNodes: map[string][]corev1.Node{
				"metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}},
				"metal3://d668eb95-5df6-4c10-a01a":              {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}},
			},
			providerIDLegacy: "metal3://d668eb95-5df6-4c10-a01a",
			providerIDNew:    "metal3://metal3/node-0/test-controlplane-000",
			expectError:      false,
		}),
		Entry("Should find duplicates if there are overlapping names of validNodes providerID in the 'new' format without common prefix", testCaseDuplicateProviderIDsExist{
			validNodes: map[string][]corev1.Node{
				"metal3://worker-1":  {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}},
				"metal3://worker-10": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}},
			},
			providerIDLegacy: "",
			providerIDNew:    "metal3://worker-1",
			expectError:      true,
		}),
		Entry("Should find duplicates when providerIDNew is empty, validNodes length is more than 1 and legacy format is used", testCaseDuplicateProviderIDsExist{
			validNodes: map[string][]corev1.Node{
				"metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}},
				"metal3://d668eb95-5df6-4c10-a01a":              {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}},
			},
			providerIDLegacy: "metal3://d668eb95-5df6-4c10-a01a",
			providerIDNew:    "",
			expectError:      true,
		}),
		Entry("Should not find duplicates when providerIDNew is empty and validNodes length is 1", testCaseDuplicateProviderIDsExist{
			validNodes: map[string][]corev1.Node{
				"metal3://d668eb95-5df6-4c10-a01a": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}},
			},
			providerIDLegacy: "metal3://d668eb95-5df6-4c10-a01a",
			providerIDNew:    "",
			expectError:      false,
		}),
		Entry("Should not find duplicates when providerIDNew and providerIDLegacy are empty and validNodes length is 1", testCaseDuplicateProviderIDsExist{
			validNodes: map[string][]corev1.Node{
				"metal3://d668eb95-5df6-4c10-a01a": {corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}},
			},
			providerIDLegacy: "",
			providerIDNew:    "",
			expectError:      false,
		}),
	)
})

/*-----------------------------------
---------Helper functions------------
------------------------------------*/

func setupSchemeMm() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := infrav1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := bmov1alpha1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := clusterv1beta1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := clusterv1.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
}

func newConfig(userDataNamespace string,
	labels map[string]string, reqs []infrav1.HostSelectorRequirement, failureDomain string,
) (*infrav1.Metal3Machine, *clusterv1.ContractVersionedObjectReference) {
	config := infrav1.Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
		},
		Spec: infrav1.Metal3MachineSpec{
			Image: infrav1.Image{
				URL:        testImageURL,
				Checksum:   testImageChecksumURL,
				DiskFormat: testImageDiskFormat,
			},
			UserData: &corev1.SecretReference{
				Name:      testUserDataSecretName,
				Namespace: userDataNamespace,
			},
			HostSelector: infrav1.HostSelector{
				MatchLabels:      labels,
				MatchExpressions: reqs,
			},
			FailureDomain: failureDomain,
		},
		Status: infrav1.Metal3MachineStatus{
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

	infrastructureRef := &clusterv1.ContractVersionedObjectReference{
		Name:     "someothermachine",
		Kind:     metal3MachineKind,
		APIGroup: infrav1.GroupVersion.Group,
	}
	return &config, infrastructureRef
}

func newMachine(machineName string, infraRef *clusterv1.ContractVersionedObjectReference,
) *clusterv1.Machine {
	if machineName == "" {
		return &clusterv1.Machine{}
	}

	if infraRef == nil {
		infraRef = &clusterv1.ContractVersionedObjectReference{}
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
				ConfigRef:      clusterv1.ContractVersionedObjectReference{},
				DataSecretName: nil,
			},
		},
	}
	return machine
}

func newMetal3Machine(name string,
	spec *infrav1.Metal3MachineSpec,
	status *infrav1.Metal3MachineStatus,
	objMeta *metav1.ObjectMeta) *infrav1.Metal3Machine {
	if name == "" {
		return &infrav1.Metal3Machine{}
	}
	if objMeta == nil {
		objMeta = &metav1.ObjectMeta{
			Name:      name,
			Namespace: namespaceName,
		}
	}

	typeMeta := &metav1.TypeMeta{
		Kind:       metal3MachineKind,
		APIVersion: infrav1.GroupVersion.String(),
	}

	if spec == nil {
		spec = &infrav1.Metal3MachineSpec{}
	}
	if status == nil {
		status = &infrav1.Metal3MachineStatus{}
	}

	return &infrav1.Metal3Machine{
		TypeMeta:   *typeMeta,
		ObjectMeta: *objMeta,
		Spec:       *spec,
		Status:     *status,
	}
}

func newBareMetalHost(name string,
	spec *bmov1alpha1.BareMetalHostSpec,
	state bmov1alpha1.ProvisioningState,
	status *bmov1alpha1.BareMetalHostStatus,
	powerOn bool,
	autoCleanMode string,
	clusterlabel bool,
	bmhUID string) *bmov1alpha1.BareMetalHost {
	if name == "" {
		return &bmov1alpha1.BareMetalHost{
			ObjectMeta: testObjectMeta("", namespaceName, ""),
		}
	}

	uid := Bmhuid
	if bmhUID != "" {
		uid = types.UID(bmhUID)
	}

	objMeta := &metav1.ObjectMeta{
		Name:      name,
		Namespace: namespaceName,
		UID:       uid,
	}

	if clusterlabel == true {
		objMeta = &metav1.ObjectMeta{
			Name:      name,
			Namespace: namespaceName,
			UID:       uid,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
				"foo":                      "bar",
			},
		}
	}

	if spec == nil {
		return &bmov1alpha1.BareMetalHost{
			ObjectMeta: *objMeta,
		}
	}
	spec.AutomatedCleaningMode = bmov1alpha1.AutomatedCleaningMode(autoCleanMode)

	if status != nil {
		status.Provisioning.State = state
		status.PoweredOn = powerOn
	} else {
		status = &bmov1alpha1.BareMetalHostStatus{}
	}

	return &bmov1alpha1.BareMetalHost{
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
				clusterv1.ClusterNameLabel: clusterName,
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
			Name:       metal3machineName + "-user-data",
			Namespace:  namespaceName,
			Finalizers: []string{"abcd"},
		},
		Data: map[string][]byte{
			"userData": []byte("QmFyRm9vCg=="),
		},
		Type: "Opaque",
	}
}

func filterCondition(conditions clusterv1beta1.Conditions, conditionType clusterv1beta1.ConditionType) []clusterv1beta1.Condition {
	filtered := []clusterv1beta1.Condition{}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			filtered = append(filtered, conditions[i])
		}
	}
	return filtered
}
