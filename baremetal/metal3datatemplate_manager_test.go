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
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"

	// "k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var timeNow = metav1.Now()

var _ = Describe("Metal3DataTemplate manager", func() {
	DescribeTable("Test Finalizers",
		func(template *infrav1.Metal3DataTemplate) {
			templateMgr, err := NewDataTemplateManager(nil, template,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			templateMgr.SetFinalizer()

			Expect(template.ObjectMeta.Finalizers).To(ContainElement(
				infrav1.DataTemplateFinalizer,
			))

			templateMgr.UnsetFinalizer()

			Expect(template.ObjectMeta.Finalizers).NotTo(ContainElement(
				infrav1.DataTemplateFinalizer,
			))
		},
		Entry("No finalizers", &infrav1.Metal3DataTemplate{}),
		Entry("Additional Finalizers", &infrav1.Metal3DataTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"foo"},
			},
		}),
	)

	type testCaseSetClusterOwnerRef struct {
		cluster     *capi.Cluster
		template    *infrav1.Metal3DataTemplate
		expectError bool
	}

	DescribeTable("Test SetClusterOwnerRef",
		func(tc testCaseSetClusterOwnerRef) {
			templateMgr, err := NewDataTemplateManager(nil, tc.template,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())
			err = templateMgr.SetClusterOwnerRef(tc.cluster)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				_, err := findOwnerRefFromList(tc.template.OwnerReferences,
					tc.cluster.TypeMeta, tc.cluster.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Cluster missing", testCaseSetClusterOwnerRef{
			expectError: true,
		}),
		Entry("no previous ownerref", testCaseSetClusterOwnerRef{
			template: &infrav1.Metal3DataTemplate{
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
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						{
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
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "def",
						},
						{
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
		template        *infrav1.Metal3DataTemplate
		indexes         []*infrav1.Metal3Data
		expectError     bool
		expectedMap     map[int]string
		expectedIndexes map[string]int
	}

	DescribeTable("Test getIndexes",
		func(tc testGetIndexes) {
			objects := []runtime.Object{}
			for _, address := range tc.indexes {
				objects = append(objects, address)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			templateMgr, err := NewDataTemplateManager(c, tc.template,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			addressMap, err := templateMgr.getIndexes(context.TODO())
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(addressMap).To(Equal(tc.expectedMap))
			Expect(tc.template.Status.Indexes).To(Equal(tc.expectedIndexes))
			Expect(tc.template.Status.LastUpdated.IsZero()).To(BeFalse())
		},
		Entry("No indexes", testGetIndexes{
			template:        &infrav1.Metal3DataTemplate{},
			expectedMap:     map[int]string{},
			expectedIndexes: map[string]int{},
		}),
		Entry("indexes", testGetIndexes{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
			},
			indexes: []*infrav1.Metal3Data{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3DataSpec{
						Index:    0,
						Template: *testObjectReference,
						Claim:    *testObjectReference,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bbc-1",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3DataSpec{
						Index: 1,
						Template: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-2",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3DataSpec{
						Index:    2,
						Template: corev1.ObjectReference{},
						Claim:    *testObjectReference,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3DataSpec{
						Index: 3,
						Template: corev1.ObjectReference{
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{},
					},
				},
			},
			expectedMap: map[int]string{
				0: "abc",
			},
			expectedIndexes: map[string]int{
				"abc": 0,
			},
		}),
	)

	var templateMeta = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: "myns",
	}

	type testCaseUpdateDatas struct {
		template          *infrav1.Metal3DataTemplate
		dataClaims        []*infrav1.Metal3DataClaim
		datas             []*infrav1.Metal3Data
		expectRequeue     bool
		expectError       bool
		expectedNbIndexes int
		expectedIndexes   map[string]int
	}

	DescribeTable("Test UpdateDatas",
		func(tc testCaseUpdateDatas) {
			objects := []runtime.Object{}
			for _, address := range tc.datas {
				objects = append(objects, address)
			}
			for _, claim := range tc.dataClaims {
				objects = append(objects, claim)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			templateMgr, err := NewDataTemplateManager(c, tc.template,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			nbIndexes, err := templateMgr.UpdateDatas(context.TODO())
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
			Expect(nbIndexes).To(Equal(tc.expectedNbIndexes))
			Expect(tc.template.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(tc.template.Status.Indexes).To(Equal(tc.expectedIndexes))

			// get list of Metal3Data objects
			dataObjects := infrav1.Metal3DataClaimList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &dataObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			// Iterate over the Metal3Data objects to find all indexes and objects
			for _, claim := range dataObjects.Items {
				if claim.DeletionTimestamp.IsZero() {
					Expect(claim.Status.RenderedData).NotTo(BeNil())
				}
			}

		},
		Entry("No Claims", testCaseUpdateDatas{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
			},
			expectedIndexes: map[string]int{},
		}),
		Entry("Claim and IP exist", testCaseUpdateDatas{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
			},
			dataClaims: []*infrav1.Metal3DataClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcd",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      "abcd",
							Namespace: "myns",
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &corev1.ObjectReference{
							Name:      "abc-2",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abce",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &corev1.ObjectReference{
							Name:      "abc-2",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "abcf",
						Namespace:         "myns",
						DeletionTimestamp: &timeNow,
					},
					Spec: infrav1.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &corev1.ObjectReference{
							Name:      "abc-3",
							Namespace: "myns",
						},
					},
				},
			},
			datas: []*infrav1.Metal3Data{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3DataSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Index: 0,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-1",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3DataSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abce",
							Namespace: "myns",
						},
						Index: 1,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3DataSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abcf",
							Namespace: "myns",
						},
						Index: 3,
					},
				},
			},
			expectedIndexes: map[string]int{
				"abc":  0,
				"abce": 1,
			},
			expectedNbIndexes: 2,
		}),
	)

	type testCaseTemplateReference struct {
		template1                  *infrav1.Metal3DataTemplate
		template2                  *infrav1.Metal3DataTemplate
		dataObject                 *infrav1.Metal3Data
		dataClaim                  *infrav1.Metal3DataClaim
		indexes                    map[int]string
		expectError                bool
		expectTemplateReference    bool
		expectDataObjectAssociated bool
	}

	DescribeTable("Test Template Reference",
		func(tc testCaseTemplateReference) {
			objects := []runtime.Object{}
			objects = append(objects, tc.dataClaim)
			if tc.dataObject != nil {
				objects = append(objects, tc.dataObject)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			templateMgr, err := NewDataTemplateManager(c, tc.template2,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = templateMgr.createData(context.TODO(), tc.dataClaim, tc.indexes)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			dataObjects := infrav1.Metal3DataList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &dataObjects, opts)
			fmt.Printf("%v", dataObjects)
			Expect(err).NotTo(HaveOccurred())
			if tc.dataObject != nil {
				Expect(len(dataObjects.Items)).To(Equal(2))
			} else {
				Expect(len(dataObjects.Items)).To(Equal(1))
			}

			if tc.expectTemplateReference {
				Expect(dataObjects.Items[0].Spec.TemplateReference).To(Equal(tc.template1.Name))
			} else {
				Expect(dataObjects.Items[0].Spec.TemplateReference).ToNot(Equal(tc.template1.Name))
			}

			if tc.dataObject != nil {
				if tc.expectDataObjectAssociated {
					result := templateMgr.dataObjectBelongsToTemplate(*tc.dataObject)
					Expect(result).To(BeTrue())
					dataClaimIndex := tc.template1.Status.Indexes[tc.dataClaim.ObjectMeta.Name]
					Expect(tc.dataObject.ObjectMeta.Name).To(Equal(tc.template1.ObjectMeta.Name + "-" + strconv.Itoa(dataClaimIndex)))
				} else {
					result := templateMgr.dataObjectBelongsToTemplate(*tc.dataObject)
					Expect(result).To(BeFalse())
					dataClaimIndex := tc.template1.Status.Indexes[tc.dataClaim.ObjectMeta.Name]
					Expect(tc.dataObject.ObjectMeta.Name).ToNot(Equal(tc.template1.ObjectMeta.Name + "-" + strconv.Itoa(dataClaimIndex)))
				}
			}
		},
		Entry("TemplateReferenceExist", testCaseTemplateReference{
			template1: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
			},
			indexes: map[int]string{},
			template2: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc1",
					Namespace: "myns",
				},
				Spec: infrav1.Metal3DataTemplateSpec{
					TemplateReference: "abc",
				},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			expectTemplateReference: true,
		}),
		Entry("TemplateReferenceDoNotExist", testCaseTemplateReference{
			template1: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
			},
			indexes: map[int]string{},
			template2: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc1",
					Namespace: "myns",
				},
				Spec: infrav1.Metal3DataTemplateSpec{},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			expectTemplateReference: false,
		}),
		Entry("TemplateReferenceRefersToOldTemplate", testCaseTemplateReference{
			template1: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "template1",
					Namespace: "myns",
				},
				Spec: infrav1.Metal3DataTemplateSpec{},
			},
			indexes: map[int]string{},
			template2: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "template2",
					Namespace: "myns",
				},
				Spec: infrav1.Metal3DataTemplateSpec{
					TemplateReference: "template1",
				},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			expectTemplateReference:    true,
			expectDataObjectAssociated: true,
		}),
		Entry("TemplateReferenceRefersToZombieTemplate", testCaseTemplateReference{
			template1: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "template1",
					Namespace: "myns",
				},
				Spec: infrav1.Metal3DataTemplateSpec{},
			},
			indexes: map[int]string{},
			template2: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "template2",
					Namespace: "myns",
				},
				Spec: infrav1.Metal3DataTemplateSpec{
					TemplateReference: "template1",
				},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			dataObject: &infrav1.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
				},
				Spec: infrav1.Metal3DataSpec{
					Index: 0,
					Template: corev1.ObjectReference{
						Name: "template12",
					},
					Claim: corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			expectDataObjectAssociated: false,
		}),
	)

	type testCaseCreateAddresses struct {
		template        *infrav1.Metal3DataTemplate
		dataClaim       *infrav1.Metal3DataClaim
		datas           []*infrav1.Metal3Data
		indexes         map[int]string
		expectRequeue   bool
		expectError     bool
		expectedDatas   []string
		expectedMap     map[int]string
		expectedIndexes map[string]int
	}

	DescribeTable("Test CreateAddresses",
		func(tc testCaseCreateAddresses) {
			objects := []runtime.Object{}
			for _, address := range tc.datas {
				objects = append(objects, address)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			templateMgr, err := NewDataTemplateManager(c, tc.template,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			allocatedMap, err := templateMgr.createData(context.TODO(), tc.dataClaim,
				tc.indexes,
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
			// get list of Metal3Data objects
			dataObjects := infrav1.Metal3DataList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &dataObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(tc.expectedDatas)).To(Equal(len(dataObjects.Items)))
			// Iterate over the Metal3Data objects to find all indexes and objects
			for _, address := range dataObjects.Items {
				Expect(tc.expectedDatas).To(ContainElement(address.Name))
				// TODO add further testing later
			}
			Expect(len(tc.dataClaim.Finalizers)).To(Equal(1))

			Expect(allocatedMap).To(Equal(tc.expectedMap))
			Expect(tc.template.Status.Indexes).To(Equal(tc.expectedIndexes))
		},
		Entry("Already exists", testCaseCreateAddresses{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: map[string]int{
						"abc": 0,
					},
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			expectedIndexes: map[string]int{
				"abc": 0,
			},
		}),
		Entry("Not allocated yet, first", testCaseCreateAddresses{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			indexes: map[int]string{},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			expectedIndexes: map[string]int{
				"abc": 0,
			},
			expectedMap: map[int]string{
				0: "abc",
			},
			expectedDatas: []string{"abc-0"},
		}),
		Entry("Not allocated yet, second", testCaseCreateAddresses{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: map[string]int{
						"bcd": 0,
					},
				},
			},
			indexes: map[int]string{0: "bcd"},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			expectedIndexes: map[string]int{
				"abc": 1,
				"bcd": 0,
			},
			expectedMap: map[int]string{
				0: "bcd",
				1: "abc",
			},
			expectedDatas: []string{"abc-1"},
		}),
		Entry("Not allocated yet, conflict", testCaseCreateAddresses{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			indexes: map[int]string{},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			datas: []*infrav1.Metal3Data{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: "myns",
					},
					Spec: infrav1.Metal3DataSpec{
						Index: 0,
						Template: corev1.ObjectReference{
							Name: "abc",
						},
						Claim: corev1.ObjectReference{
							Name: "bcd",
						},
					},
				},
			},
			expectedIndexes: map[string]int{},
			expectedMap:     map[int]string{},
			expectedDatas:   []string{"abc-0"},
			expectRequeue:   true,
		}),
	)

	type testCaseDeleteDatas struct {
		template        *infrav1.Metal3DataTemplate
		dataClaim       *infrav1.Metal3DataClaim
		datas           []*infrav1.Metal3Data
		indexes         map[int]string
		expectedMap     map[int]string
		expectedIndexes map[string]int
		expectError     bool
	}

	DescribeTable("Test DeleteAddresses",
		func(tc testCaseDeleteDatas) {
			objects := []runtime.Object{}
			for _, address := range tc.datas {
				objects = append(objects, address)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			templateMgr, err := NewDataTemplateManager(c, tc.template,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			allocatedMap, err := templateMgr.deleteData(context.TODO(), tc.dataClaim, tc.indexes)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			// get list of Metal3Data objects
			dataObjects := infrav1.Metal3DataList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &dataObjects, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(dataObjects.Items)).To(Equal(0))

			Expect(tc.template.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(allocatedMap).To(Equal(tc.expectedMap))
			Expect(tc.template.Status.Indexes).To(Equal(tc.expectedIndexes))
			Expect(len(tc.dataClaim.Finalizers)).To(Equal(0))
		},
		Entry("Empty Template", testCaseDeleteDatas{
			template: &infrav1.Metal3DataTemplate{},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
		}),
		Entry("No Deletion needed", testCaseDeleteDatas{
			template: &infrav1.Metal3DataTemplate{},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			expectedMap: map[int]string{0: "abcd"},
			indexes: map[int]string{
				0: "abcd",
			},
		}),
		Entry("Deletion needed, not found", testCaseDeleteDatas{
			template: &infrav1.Metal3DataTemplate{
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: map[string]int{
						"TestRef": 0,
					},
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			indexes: map[int]string{
				0: "TestRef",
			},
			expectedIndexes: map[string]int{},
			expectedMap:     map[int]string{},
		}),
		Entry("Deletion needed", testCaseDeleteDatas{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				Spec: infrav1.Metal3DataTemplateSpec{},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: map[string]int{
						"TestRef": 0,
					},
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
					Finalizers: []string{
						infrav1.DataClaimFinalizer,
					},
				},
			},
			indexes: map[int]string{
				0: "TestRef",
			},
			expectedMap:     map[int]string{},
			expectedIndexes: map[string]int{},
			datas: []*infrav1.Metal3Data{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-0",
					},
				},
			},
		}),
	)

})
