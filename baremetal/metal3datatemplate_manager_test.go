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
	"strconv"

	"github.com/go-logr/logr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var timeNow = metav1.Now()

var _ = Describe("Metal3DataTemplate manager", func() {
	DescribeTable("Test Finalizers",
		func(template *capm3.Metal3DataTemplate) {
			templateMgr, err := NewDataTemplateManager(nil, template,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			templateMgr.SetFinalizer()

			Expect(template.ObjectMeta.Finalizers).To(ContainElement(
				capm3.DataTemplateFinalizer,
			))

			templateMgr.UnsetFinalizer()

			Expect(template.ObjectMeta.Finalizers).NotTo(ContainElement(
				capm3.DataTemplateFinalizer,
			))
		},
		Entry("No finalizers", &capm3.Metal3DataTemplate{}),
		Entry("Additional Finalizers", &capm3.Metal3DataTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"foo"},
			},
		}),
	)

	type testCaseSetClusterOwnerRef struct {
		cluster     *clusterv1.Cluster
		template    *capm3.Metal3DataTemplate
		expectError bool
	}

	DescribeTable("Test SetClusterOwnerRef",
		func(tc testCaseSetClusterOwnerRef) {
			templateMgr, err := NewDataTemplateManager(nil, tc.template,
				logr.Discard(),
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
			template: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc-cluster",
				},
			},
		}),
		Entry("previous ownerref", testCaseSetClusterOwnerRef{
			template: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "def",
						},
					},
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc-cluster",
				},
			},
		}),
		Entry("ownerref present", testCaseSetClusterOwnerRef{
			template: &capm3.Metal3DataTemplate{
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
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc-cluster",
				},
			},
		}),
	)

	type testGetIndexes struct {
		template        *capm3.Metal3DataTemplate
		indexes         []*capm3.Metal3Data
		expectError     bool
		expectedMap     map[int]string
		expectedIndexes map[string]int
	}

	DescribeTable("Test getIndexes",
		func(tc testGetIndexes) {
			objects := []client.Object{}
			for _, address := range tc.indexes {
				objects = append(objects, address)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			templateMgr, err := NewDataTemplateManager(fakeClient, tc.template,
				logr.Discard(),
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
			template:        &capm3.Metal3DataTemplate{},
			expectedMap:     map[int]string{},
			expectedIndexes: map[string]int{},
		}),
		Entry("indexes", testGetIndexes{
			template: &capm3.Metal3DataTemplate{
				ObjectMeta: testObjectMeta,
				Spec:       capm3.Metal3DataTemplateSpec{},
			},
			indexes: []*capm3.Metal3Data{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: namespaceName,
					},
					Spec: capm3.Metal3DataSpec{
						Index:    0,
						Template: *testObjectReference,
						Claim:    *testObjectReference,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bbc-1",
						Namespace: namespaceName,
					},
					Spec: capm3.Metal3DataSpec{
						Index: 1,
						Template: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: namespaceName,
						},
						Claim: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: namespaceName,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-2",
						Namespace: namespaceName,
					},
					Spec: capm3.Metal3DataSpec{
						Index:    2,
						Template: corev1.ObjectReference{},
						Claim:    *testObjectReference,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: namespaceName,
					},
					Spec: capm3.Metal3DataSpec{
						Index: 3,
						Template: corev1.ObjectReference{
							Namespace: namespaceName,
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
		Namespace: namespaceName,
	}

	type testCaseUpdateDatas struct {
		template          *capm3.Metal3DataTemplate
		dataClaims        []*capm3.Metal3DataClaim
		datas             []*capm3.Metal3Data
		expectRequeue     bool
		expectError       bool
		expectedNbIndexes int
		expectedIndexes   map[string]int
	}

	DescribeTable("Test UpdateDatas",
		func(tc testCaseUpdateDatas) {
			objects := []client.Object{}
			for _, address := range tc.datas {
				objects = append(objects, address)
			}
			for _, claim := range tc.dataClaims {
				objects = append(objects, claim)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			templateMgr, err := NewDataTemplateManager(fakeClient, tc.template,
				logr.Discard(),
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
			dataObjects := capm3.Metal3DataClaimList{}
			opts := &client.ListOptions{}
			err = fakeClient.List(context.TODO(), &dataObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			// Iterate over the Metal3Data objects to find all indexes and objects
			for _, claim := range dataObjects.Items {
				if claim.DeletionTimestamp.IsZero() {
					Expect(claim.Status.RenderedData).NotTo(BeNil())
				}
			}

		},
		Entry("No Claims", testCaseUpdateDatas{
			template: &capm3.Metal3DataTemplate{
				ObjectMeta: templateMeta,
			},
			expectedIndexes: map[string]int{},
		}),
		Entry("Claim and IP exist", testCaseUpdateDatas{
			template: &capm3.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       capm3.Metal3DataTemplateSpec{},
			},
			dataClaims: []*capm3.Metal3DataClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: namespaceName,
					},
					Spec: capm3.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: namespaceName,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcd",
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
							Name:      "abc-2",
							Namespace: namespaceName,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abce",
						Namespace: namespaceName,
					},
					Spec: capm3.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: namespaceName,
						},
					},
					Status: capm3.Metal3DataClaimStatus{
						RenderedData: &corev1.ObjectReference{
							Name:      "abc-2",
							Namespace: namespaceName,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "abcf",
						Namespace:         namespaceName,
						DeletionTimestamp: &timeNow,
					},
					Spec: capm3.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: namespaceName,
						},
					},
					Status: capm3.Metal3DataClaimStatus{
						RenderedData: &corev1.ObjectReference{
							Name:      "abc-3",
							Namespace: namespaceName,
						},
					},
				},
			},
			datas: []*capm3.Metal3Data{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: namespaceName,
					},
					Spec: capm3.Metal3DataSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: namespaceName,
						},
						Claim: corev1.ObjectReference{
							Name:      "abc",
							Namespace: namespaceName,
						},
						Index: 0,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-1",
						Namespace: namespaceName,
					},
					Spec: capm3.Metal3DataSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: namespaceName,
						},
						Claim: corev1.ObjectReference{
							Name:      "abce",
							Namespace: namespaceName,
						},
						Index: 1,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: namespaceName,
					},
					Spec: capm3.Metal3DataSpec{
						Template: corev1.ObjectReference{
							Name:      "abc",
							Namespace: namespaceName,
						},
						Claim: corev1.ObjectReference{
							Name:      "abcf",
							Namespace: namespaceName,
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
		template1                  *capm3.Metal3DataTemplate
		template2                  *capm3.Metal3DataTemplate
		dataObject                 *capm3.Metal3Data
		dataClaim                  *capm3.Metal3DataClaim
		indexes                    map[int]string
		expectError                bool
		expectTemplateReference    bool
		expectDataObjectAssociated bool
	}

	DescribeTable("Test Template Reference",
		func(tc testCaseTemplateReference) {
			objects := []client.Object{}
			objects = append(objects, tc.dataClaim)
			if tc.dataObject != nil {
				objects = append(objects, tc.dataObject)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			templateMgr, err := NewDataTemplateManager(fakeClient, tc.template2,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = templateMgr.createData(context.TODO(), tc.dataClaim, tc.indexes)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			dataObjects := capm3.Metal3DataList{}
			opts := &client.ListOptions{}
			err = fakeClient.List(context.TODO(), &dataObjects, opts)
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
					Expect(tc.dataObject.ObjectMeta.Name).To(Equal(
						tc.template1.ObjectMeta.Name + "-" + strconv.Itoa(dataClaimIndex)))
				} else {
					result := templateMgr.dataObjectBelongsToTemplate(*tc.dataObject)
					Expect(result).To(BeFalse())
					dataClaimIndex := tc.template1.Status.Indexes[tc.dataClaim.ObjectMeta.Name]
					Expect(tc.dataObject.ObjectMeta.Name).ToNot(Equal(tc.template1.ObjectMeta.Name + "-" + strconv.Itoa(dataClaimIndex)))
				}
			}
		},
		Entry("TemplateReferenceExist", testCaseTemplateReference{
			template1: &capm3.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       capm3.Metal3DataTemplateSpec{},
			},
			indexes: map[int]string{},
			template2: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc1",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataTemplateSpec{
					TemplateReference: "abc",
				},
				Status: capm3.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			expectTemplateReference: true,
		}),
		Entry("TemplateReferenceDoNotExist", testCaseTemplateReference{
			template1: &capm3.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       capm3.Metal3DataTemplateSpec{},
			},
			indexes: map[int]string{},
			template2: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc1",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataTemplateSpec{},
				Status: capm3.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			expectTemplateReference: false,
		}),
		Entry("TemplateReferenceRefersToOldTemplate", testCaseTemplateReference{
			template1: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "template1",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataTemplateSpec{},
			},
			indexes: map[int]string{},
			template2: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "template2",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataTemplateSpec{
					TemplateReference: "template1",
				},
				Status: capm3.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			expectTemplateReference:    true,
			expectDataObjectAssociated: true,
		}),
		Entry("TemplateReferenceRefersToZombieTemplate", testCaseTemplateReference{
			template1: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "template1",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataTemplateSpec{},
			},
			indexes: map[int]string{},
			template2: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "template2",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataTemplateSpec{
					TemplateReference: "template1",
				},
				Status: capm3.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			dataObject: &capm3.Metal3Data{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: namespaceName,
				},
				Spec: capm3.Metal3DataSpec{
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
		template        *capm3.Metal3DataTemplate
		dataClaim       *capm3.Metal3DataClaim
		datas           []*capm3.Metal3Data
		indexes         map[int]string
		expectRequeue   bool
		expectError     bool
		expectedDatas   []string
		expectedMap     map[int]string
		expectedIndexes map[string]int
	}

	DescribeTable("Test CreateAddresses",
		func(tc testCaseCreateAddresses) {
			objects := []client.Object{}
			for _, address := range tc.datas {
				objects = append(objects, address)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			templateMgr, err := NewDataTemplateManager(fakeClient, tc.template,
				logr.Discard(),
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
			dataObjects := capm3.Metal3DataList{}
			opts := &client.ListOptions{}
			err = fakeClient.List(context.TODO(), &dataObjects, opts)
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
			template: &capm3.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Status: capm3.Metal3DataTemplateStatus{
					Indexes: map[string]int{
						"abc": 0,
					},
				},
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			expectedIndexes: map[string]int{
				"abc": 0,
			},
		}),
		Entry("Not allocated yet, first", testCaseCreateAddresses{
			template: &capm3.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       capm3.Metal3DataTemplateSpec{},
				Status: capm3.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			indexes: map[int]string{},
			dataClaim: &capm3.Metal3DataClaim{
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
			template: &capm3.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       capm3.Metal3DataTemplateSpec{},
				Status: capm3.Metal3DataTemplateStatus{
					Indexes: map[string]int{
						"bcd": 0,
					},
				},
			},
			indexes: map[int]string{0: "bcd"},
			dataClaim: &capm3.Metal3DataClaim{
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
			template: &capm3.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       capm3.Metal3DataTemplateSpec{},
				Status: capm3.Metal3DataTemplateStatus{
					Indexes: map[string]int{},
				},
			},
			indexes: map[int]string{},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR,
			},
			datas: []*capm3.Metal3Data{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: namespaceName,
					},
					Spec: capm3.Metal3DataSpec{
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
		template        *capm3.Metal3DataTemplate
		dataClaim       *capm3.Metal3DataClaim
		datas           []*capm3.Metal3Data
		indexes         map[int]string
		expectedMap     map[int]string
		expectedIndexes map[string]int
		expectError     bool
	}

	DescribeTable("Test DeleteAddresses",
		func(tc testCaseDeleteDatas) {
			objects := []client.Object{}
			for _, address := range tc.datas {
				objects = append(objects, address)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			templateMgr, err := NewDataTemplateManager(fakeClient, tc.template,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			allocatedMap, err := templateMgr.deleteData(context.TODO(), tc.dataClaim, tc.indexes)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			// get list of Metal3Data objects
			dataObjects := capm3.Metal3DataList{}
			opts := &client.ListOptions{}
			err = fakeClient.List(context.TODO(), &dataObjects, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(dataObjects.Items)).To(Equal(0))

			Expect(tc.template.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(allocatedMap).To(Equal(tc.expectedMap))
			Expect(tc.template.Status.Indexes).To(Equal(tc.expectedIndexes))
			Expect(len(tc.dataClaim.Finalizers)).To(Equal(0))
		},
		Entry("Empty Template", testCaseDeleteDatas{
			template: &capm3.Metal3DataTemplate{},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
		}),
		Entry("No Deletion needed", testCaseDeleteDatas{
			template: &capm3.Metal3DataTemplate{},
			dataClaim: &capm3.Metal3DataClaim{
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
			template: &capm3.Metal3DataTemplate{
				Status: capm3.Metal3DataTemplateStatus{
					Indexes: map[string]int{
						"TestRef": 0,
					},
				},
			},
			dataClaim: &capm3.Metal3DataClaim{
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
			template: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				Spec: capm3.Metal3DataTemplateSpec{},
				Status: capm3.Metal3DataTemplateStatus{
					Indexes: map[string]int{
						"TestRef": 0,
					},
				},
			},
			dataClaim: &capm3.Metal3DataClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
					Finalizers: []string{
						capm3.DataClaimFinalizer,
					},
				},
			},
			indexes: map[int]string{
				0: "TestRef",
			},
			expectedMap:     map[int]string{},
			expectedIndexes: map[string]int{},
			datas: []*capm3.Metal3Data{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-0",
					},
				},
			},
		}),
	)

})
