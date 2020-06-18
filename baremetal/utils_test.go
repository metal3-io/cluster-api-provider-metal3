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

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	//fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/cluster-api/util/patch"
)

var _ = Describe("Metal3 manager utils", func() {

	type testCaseFilter struct {
		TestList     []string
		TestString   string
		ExpectedList []string
	}

	DescribeTable("Test Filter",
		func(tc testCaseFilter) {
			resultList := Filter(tc.TestList, tc.TestString)
			Expect(resultList).To(Equal(tc.ExpectedList))
		},
		Entry("Absent", testCaseFilter{
			TestList:     []string{"abc", "bcd", "def"},
			TestString:   "efg",
			ExpectedList: []string{"abc", "bcd", "def"},
		}),
		Entry("Present in 1", testCaseFilter{
			TestList:     []string{"abc", "bcd", "def"},
			TestString:   "abc",
			ExpectedList: []string{"bcd", "def"},
		}),
		Entry("Present in 2", testCaseFilter{
			TestList:     []string{"abc", "bcd", "def"},
			TestString:   "bcd",
			ExpectedList: []string{"abc", "def"},
		}),
		Entry("Present in 3", testCaseFilter{
			TestList:     []string{"abc", "bcd", "def"},
			TestString:   "def",
			ExpectedList: []string{"abc", "bcd"},
		}),
	)

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

	type testCasePatch struct {
		TestObject     *capm3.Metal3Machine
		ExistingObject *capm3.Metal3Machine
		ExpectedError  bool
		CreateObject   bool
	}

	var testObject = &capm3.Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "abc",
			Namespace: "myns",
		},
		Spec: capm3.Metal3MachineSpec{
			ProviderID: pointer.StringPtr("abcdef"),
		},
		Status: capm3.Metal3MachineStatus{
			Ready: true,
		},
	}

	var existingObject = &capm3.Metal3Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "abc",
			Namespace: "myns",
		},
		Spec: capm3.Metal3MachineSpec{
			ProviderID: pointer.StringPtr("abcdefg"),
		},
		Status: capm3.Metal3MachineStatus{
			Ready: true,
		},
	}

	DescribeTable("Test patchIfFound",
		func(tc testCasePatch) {
			var err error
			c := k8sClient

			// Create the object in the API
			if tc.CreateObject {
				err = c.Create(context.TODO(), tc.ExistingObject)
				Expect(err).NotTo(HaveOccurred())
				m3m := capm3.Metal3Machine{}
				err = c.Get(context.TODO(),
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
			helper, err := patch.NewHelper(tc.ExistingObject, c)
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
					savedObject := capm3.Metal3Machine{}
					err = c.Get(context.TODO(),
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
			err = c.Delete(context.TODO(), tc.ExistingObject)
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

	It("Parses the providerID properly", func() {
		Expect(parseProviderID("metal3://abcd")).To(Equal("abcd"))
		Expect(parseProviderID("foo://abcd")).To(Equal("foo://abcd"))
	})
})
