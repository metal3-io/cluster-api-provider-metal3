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
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Metal3 manager utils", func() {

	type testCaseFilter struct {
		TestList     []string
		TestString   string
		ExpectedList []string
	}

	type testCaseContains struct {
		TestList       []string
		TestString     string
		ExpectedOutput bool
	}

	DescribeTable("Test Filter",
		func(tc testCaseContains) {
			Expect(Contains(tc.TestList, tc.TestString)).To(Equal(tc.ExpectedOutput))
		},
		Entry("Absent", testCaseContains{
			TestList:       []string{"abc", "bcd", "def"},
			TestString:     "efg",
			ExpectedOutput: false,
		}),
		Entry("Present 1", testCaseContains{
			TestList:       []string{"abc", "bcd", "def"},
			TestString:     "abc",
			ExpectedOutput: true,
		}),
		Entry("Present 2", testCaseContains{
			TestList:       []string{"abc", "bcd", "def"},
			TestString:     "bcd",
			ExpectedOutput: true,
		}),
		Entry("Present 3", testCaseContains{
			TestList:       []string{"abc", "bcd", "def"},
			TestString:     "def",
			ExpectedOutput: true,
		}),
	)

	Describe("NotFoundError", func() {
		It("should return proper message", func() {
			err := &NotFoundError{}
			Expect(err.Error()).To(Equal("Object not found"))
		})
	})

	type testCaseUpdate struct {
		TestObject     *infrav1.Metal3Machine
		ExistingObject *infrav1.Metal3Machine
		ExpectedError  bool
	}

	type testCasePatch struct {
		TestObject     *infrav1.Metal3Machine
		ExistingObject *infrav1.Metal3Machine
		ExpectedError  bool
		CreateObject   bool
	}

	var testObject = &infrav1.Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metal3machineName,
			Namespace: namespaceName,
		},
		Spec: infrav1.Metal3MachineSpec{
			ProviderID:            ptr.To("abcdef"),
			AutomatedCleaningMode: ptr.To("metadata"),
		},
		Status: infrav1.Metal3MachineStatus{
			Ready: true,
		},
	}

	var existingObject = &infrav1.Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metal3machineName,
			Namespace: namespaceName,
		},
		Spec: infrav1.Metal3MachineSpec{
			ProviderID: ptr.To("abcdefg"),
		},
		Status: infrav1.Metal3MachineStatus{
			Ready: true,
		},
	}

	DescribeTable("Test patchIfFound",
		func(tc testCasePatch) {
			var err error

			// Create the object in the API
			if tc.CreateObject {
				err = k8sClient.Create(context.TODO(), tc.ExistingObject)
				Expect(err).NotTo(HaveOccurred())
				m3m := infrav1.Metal3Machine{}
				err = k8sClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.ExistingObject.Name,
						Namespace: tc.ExistingObject.Namespace,
					},
					&m3m,
				)
				tc.ExistingObject = &m3m
				Expect(err).NotTo(HaveOccurred())
				if !tc.ExpectedError {
					tc.TestObject.ObjectMeta = m3m.ObjectMeta
				}
			}

			// Create the helper, and the object reference
			obj := tc.TestObject.DeepCopy()
			helper, err := v1beta1patch.NewHelper(tc.ExistingObject, k8sClient)
			Expect(err).NotTo(HaveOccurred())

			// Run function
			err = patchIfFound(context.TODO(), helper, tc.TestObject)
			if tc.ExpectedError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())

				// The object should not be modified
				Expect(obj.Spec).To(Equal(tc.TestObject.Spec))
				Expect(obj.Status).To(Equal(tc.TestObject.Status))

				if tc.CreateObject {
					// verify that the object was updated
					savedObject := infrav1.Metal3Machine{}
					err = k8sClient.Get(context.TODO(),
						client.ObjectKey{
							Name:      tc.TestObject.Name,
							Namespace: tc.TestObject.Namespace,
						},
						&savedObject,
					)
					Expect(err).NotTo(HaveOccurred())
					Expect(savedObject.Spec).To(Equal(tc.TestObject.Spec))
					Expect(savedObject.Status).To(Equal(tc.TestObject.Status))
					Expect(savedObject.ResourceVersion).NotTo(Equal(tc.TestObject.ResourceVersion))
				}
			}

			// Delete the object from API
			err = k8sClient.Delete(context.TODO(), tc.ExistingObject)
			if err != nil {
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		},
		Entry("Expect error", testCasePatch{
			TestObject:     testObject.DeepCopy(),
			ExistingObject: existingObject.DeepCopy(),
			CreateObject:   true,
			ExpectedError:  true,
		}),
		Entry("Object does not exist", testCasePatch{
			TestObject:     testObject.DeepCopy(),
			ExistingObject: existingObject.DeepCopy(),
			ExpectedError:  false,
			CreateObject:   false,
		}),
		Entry("Object exists", testCasePatch{
			TestObject:     testObject.DeepCopy(),
			ExistingObject: existingObject.DeepCopy(),
			ExpectedError:  false,
			CreateObject:   true,
		}),
	)

	DescribeTable("Test Update",
		func(tc testCaseUpdate) {
			if tc.ExistingObject != nil {
				err := k8sClient.Create(context.TODO(), tc.ExistingObject)
				Expect(err).NotTo(HaveOccurred())
				m3m := infrav1.Metal3Machine{}
				err = k8sClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.ExistingObject.Name,
						Namespace: tc.ExistingObject.Namespace,
					},
					&m3m,
				)
				Expect(err).NotTo(HaveOccurred())
				tc.TestObject.ObjectMeta = m3m.ObjectMeta
			}
			obj := tc.TestObject.DeepCopy()
			err := updateObject(context.TODO(), k8sClient, obj)
			if tc.ExpectedError {
				Expect(err).To(HaveOccurred())
				Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(obj.Spec).To(Equal(tc.TestObject.Spec))
				Expect(obj.Status).To(Equal(tc.TestObject.Status))
				savedObject := infrav1.Metal3Machine{}
				err = k8sClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.TestObject.Name,
						Namespace: tc.TestObject.Namespace,
					},
					&savedObject,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedObject.Spec).To(Equal(tc.TestObject.Spec))
				Expect(savedObject.ResourceVersion).NotTo(Equal(tc.TestObject.ResourceVersion))
				err = updateObject(context.TODO(), k8sClient, obj)
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
			}
			err = k8sClient.Delete(context.TODO(), tc.TestObject)
			if err != nil {
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		},
		Entry("Object does not exist", testCaseUpdate{
			TestObject:     testObject.DeepCopy(),
			ExistingObject: nil,
			ExpectedError:  true,
		}),
		Entry("Object exists", testCaseUpdate{
			TestObject:     testObject.DeepCopy(),
			ExistingObject: existingObject.DeepCopy(),
			ExpectedError:  false,
		}),
	)

	DescribeTable("Test Create",
		func(tc testCaseUpdate) {
			if tc.ExistingObject != nil {
				err := k8sClient.Create(context.TODO(), tc.ExistingObject)
				Expect(err).NotTo(HaveOccurred())
			}
			obj := tc.TestObject.DeepCopy()
			err := createObject(context.TODO(), k8sClient, obj)
			if tc.ExpectedError {
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(obj.Spec).To(Equal(tc.TestObject.Spec))
				Expect(obj.Status).To(Equal(tc.TestObject.Status))
				savedObject := infrav1.Metal3Machine{}
				err = k8sClient.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.TestObject.Name,
						Namespace: tc.TestObject.Namespace,
					},
					&savedObject,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedObject.Spec).To(Equal(tc.TestObject.Spec))
			}
			err = k8sClient.Delete(context.TODO(), tc.TestObject)
			if err != nil {
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		},
		Entry("Object does not exist", testCaseUpdate{
			TestObject:     testObject.DeepCopy(),
			ExistingObject: nil,
			ExpectedError:  false,
		}),
		Entry("Object exists", testCaseUpdate{
			TestObject:     testObject.DeepCopy(),
			ExistingObject: existingObject.DeepCopy(),
			ExpectedError:  true,
		}),
	)

	DescribeTable("Test checkSecretExists",
		func(secretExists bool) {
			if secretExists {
				err := k8sClient.Create(context.TODO(), &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: namespaceName,
					},
				})
				Expect(err).NotTo(HaveOccurred())
			}
			_, err := checkSecretExists(context.TODO(), k8sClient, "abc", namespaceName)
			if secretExists {
				Expect(err).NotTo(HaveOccurred())
				err = k8sClient.Delete(context.TODO(), &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: namespaceName,
					},
				})
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		},
		Entry("Object does not exist", false),
		Entry("Object exists", true),
	)

	DescribeTable("Test createSecret",
		func(secretExists bool) {
			if secretExists {
				err := k8sClient.Create(context.TODO(), &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: namespaceName,
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "ghij",
								Kind:       metal3MachineKind,
								APIVersion: infrav1.GroupVersion.String(),
								UID:        "7df7fe8e-9cdb-4c57-8144-0a30bf6b9496",
							},
						},
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Type: metal3SecretType,
				})
				Expect(err).NotTo(HaveOccurred())
			}
			ownerRef := []metav1.OwnerReference{{
				Name:       "abcd",
				Kind:       metal3MachineKind,
				APIVersion: infrav1.GroupVersion.String(),
				UID:        "7df7fe8e-9cdb-4c57-8144-0a30bf6b9496",
			}}
			content := map[string][]byte{
				"abc": []byte("def"),
			}
			err := createSecret(context.TODO(), k8sClient, "abc", namespaceName, "ghi",
				ownerRef, content,
			)
			Expect(err).NotTo(HaveOccurred())
			savedSecret := corev1.Secret{}
			err = k8sClient.Get(context.TODO(),
				client.ObjectKey{
					Name:      "abc",
					Namespace: namespaceName,
				},
				&savedSecret,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(savedSecret.ObjectMeta.Labels).To(Equal(map[string]string{
				clusterv1beta1.ClusterNameLabel: "ghi",
			}))
			Expect(savedSecret.ObjectMeta.OwnerReferences).To(Equal(ownerRef))
			Expect(savedSecret.Data).To(Equal(content))

			err = k8sClient.Delete(context.TODO(), &corev1.Secret{
				ObjectMeta: testObjectMeta("abc", namespaceName, ""),
			})
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("Object does not exist", false),
		Entry("Object exists", true),
	)

	DescribeTable("Test deleteSecret",
		func(secretExists bool) {
			if secretExists {
				err := k8sClient.Create(context.TODO(), &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "abc",
						Namespace:  namespaceName,
						Finalizers: []string{"foo.bar/foo"},
					},
				})
				Expect(err).NotTo(HaveOccurred())
			}

			err := deleteSecret(context.TODO(), k8sClient, "abc", namespaceName)
			Expect(err).NotTo(HaveOccurred())
			savedSecret := corev1.Secret{}
			err = k8sClient.Get(context.TODO(),
				client.ObjectKey{
					Name:      "abc",
					Namespace: namespaceName,
				},
				&savedSecret,
			)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		},
		Entry("Object does not exist", false),
		Entry("Object exists", true),
	)

	type testCaseFetchM3DataTemplate struct {
		DataTemplate  *infrav1.Metal3DataTemplate
		ClusterName   string
		TemplateRef   *corev1.ObjectReference
		ExpectError   bool
		ExpectEmpty   bool
		ExpectRequeue bool
	}

	DescribeTable("Test fetchM3DataTemplate",
		func(tc testCaseFetchM3DataTemplate) {
			if tc.DataTemplate != nil {
				err := k8sClient.Create(context.TODO(), tc.DataTemplate)
				Expect(err).NotTo(HaveOccurred())
			}

			result, err := fetchM3DataTemplate(context.TODO(), tc.TemplateRef, k8sClient,
				logr.Discard(), tc.ClusterName,
			)
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
			if tc.DataTemplate != nil {
				err = k8sClient.Delete(context.TODO(), tc.DataTemplate)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Object does not exist", testCaseFetchM3DataTemplate{
			TemplateRef: &corev1.ObjectReference{
				Name:      metal3DataTemplateName,
				Namespace: namespaceName,
			},
			ExpectRequeue: true,
		}),
		Entry("Object Ref nil", testCaseFetchM3DataTemplate{
			ExpectEmpty: true,
		}),
		Entry("Object Ref Name empty", testCaseFetchM3DataTemplate{
			TemplateRef: &corev1.ObjectReference{
				Name: "",
			},
			ExpectError: true,
		}),
		Entry("Object with wrong cluster", testCaseFetchM3DataTemplate{
			DataTemplate: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
				Spec: infrav1.Metal3DataTemplateSpec{
					ClusterName: clusterName,
				},
			},
			ClusterName: "def",
			TemplateRef: &corev1.ObjectReference{
				Name:      metal3DataTemplateName,
				Namespace: namespaceName,
			},
			ExpectError: true,
		}),
		Entry("Object with correct cluster", testCaseFetchM3DataTemplate{
			DataTemplate: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
				Spec: infrav1.Metal3DataTemplateSpec{
					ClusterName: clusterName,
				},
			},
			ClusterName: clusterName,
			TemplateRef: &corev1.ObjectReference{
				Name:      metal3DataTemplateName,
				Namespace: namespaceName,
			},
		}),
	)

	type testCaseFetchM3Data struct {
		Data          *infrav1.Metal3Data
		Name          string
		Namespace     string
		ExpectError   bool
		ExpectRequeue bool
		ExpectEmpty   bool
	}

	DescribeTable("Test fetchM3Data",
		func(tc testCaseFetchM3Data) {
			if tc.Data != nil {
				err := k8sClient.Create(context.TODO(), tc.Data)
				Expect(err).NotTo(HaveOccurred())
			}

			result, err := fetchM3Data(context.TODO(), k8sClient, logr.Discard(), tc.Name,
				tc.Namespace,
			)
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
			if tc.Data != nil {
				err = k8sClient.Delete(context.TODO(), tc.Data)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Object does not exist", testCaseFetchM3Data{
			Name:          metal3machineName,
			Namespace:     namespaceName,
			ExpectRequeue: true,
		}),
		Entry("Object exists", testCaseFetchM3Data{
			Data: &infrav1.Metal3Data{
				ObjectMeta: testObjectMeta(metal3DataName, namespaceName, ""),
			},
			Name:      metal3DataName,
			Namespace: namespaceName,
		}),
	)

	type testCaseGetM3Machine struct {
		Machine       *infrav1.Metal3Machine
		Name          string
		Namespace     string
		DataTemplate  *infrav1.Metal3DataTemplate
		ExpectError   bool
		ExpectRequeue bool
		ExpectEmpty   bool
	}

	DescribeTable("Test getM3Machine",
		func(tc testCaseGetM3Machine) {
			if tc.Machine != nil {
				err := k8sClient.Create(context.TODO(), tc.Machine)
				Expect(err).NotTo(HaveOccurred())
			}

			result, err := getM3Machine(context.TODO(), k8sClient, logr.Discard(), tc.Name,
				tc.Namespace, tc.DataTemplate, false,
			)
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
				err = k8sClient.Delete(context.TODO(), tc.Machine)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Object does not exist", testCaseGetM3Machine{
			Name:        metal3machineName,
			Namespace:   namespaceName,
			ExpectEmpty: true,
		}),
		Entry("Object exists", testCaseGetM3Machine{
			Machine: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, ""),
			},
			Name:      metal3machineName,
			Namespace: namespaceName,
		}),
		Entry("Object exists, dataTemplate nil", testCaseGetM3Machine{
			Machine: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, ""),
				Spec: infrav1.Metal3MachineSpec{
					DataTemplate: nil,
				},
			},
			DataTemplate: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
			},
			Name:        metal3machineName,
			Namespace:   namespaceName,
			ExpectEmpty: true,
		}),
		Entry("Object exists, dataTemplate name mismatch", testCaseGetM3Machine{
			Machine: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, ""),
				Spec: infrav1.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name:      "abcd",
						Namespace: namespaceName,
					},
				},
			},
			DataTemplate: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
			},
			Name:        metal3machineName,
			Namespace:   namespaceName,
			ExpectEmpty: true,
		}),
		Entry("Object exists, dataTemplate namespace mismatch", testCaseGetM3Machine{
			Machine: &infrav1.Metal3Machine{
				ObjectMeta: testObjectMeta(metal3machineName, namespaceName, ""),
				Spec: infrav1.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name:      metal3DataTemplateName,
						Namespace: "defg",
					},
				},
			},
			DataTemplate: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, ""),
			},
			Name:        metal3machineName,
			Namespace:   namespaceName,
			ExpectEmpty: true,
		}),
	)

	It("Parses the providerID properly", func() {
		Expect(parseProviderID(ProviderIDPrefix + "abcd")).To(Equal("abcd"))
		Expect(parseProviderID("foo://abcd")).To(Equal("foo://abcd"))
	})
})
