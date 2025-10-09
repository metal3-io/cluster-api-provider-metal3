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
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var timeNow = metav1.Now()

var _ = Describe("Metal3DataTemplate manager", func() {
	DescribeTable("Test Finalizers",
		func(template *infrav1.Metal3DataTemplate) {
			templateMgr, err := NewDataTemplateManager(nil, template,
				logr.Discard(),
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
		cluster     *clusterv1.Cluster
		template    *infrav1.Metal3DataTemplate
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
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, "", ""),
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: testObjectMeta("abc-cluster", "", ""),
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
			cluster: &clusterv1.Cluster{
				ObjectMeta: testObjectMeta("abc-cluster", "", ""),
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
			cluster: &clusterv1.Cluster{
				ObjectMeta: testObjectMeta("abc-cluster", "", ""),
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
			template:        &infrav1.Metal3DataTemplate{},
			expectedMap:     map[int]string{},
			expectedIndexes: map[string]int{},
		}),
		Entry("indexes", testGetIndexes{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta(metal3DataTemplateName, namespaceName, m3dtuid),
				Spec:       infrav1.Metal3DataTemplateSpec{},
			},
			indexes: []*infrav1.Metal3Data{
				{
					ObjectMeta: testObjectMeta("abc-0", namespaceName, ""),
					Spec: infrav1.Metal3DataSpec{
						Index:    0,
						Template: *testObjectReference(metal3DataTemplateName),
						Claim:    *testObjectReference(metal3DataClaimName),
					},
				},
				{
					ObjectMeta: testObjectMeta("bbc-1", namespaceName, ""),
					Spec: infrav1.Metal3DataSpec{
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
					Spec: infrav1.Metal3DataSpec{
						Index:    2,
						Template: corev1.ObjectReference{},
						Claim:    *testObjectReference(metal3DataClaimName),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataSpec{
						Index: 3,
						Template: corev1.ObjectReference{
							Namespace: namespaceName,
						},
						Claim: corev1.ObjectReference{},
					},
				},
			},
			expectedMap: map[int]string{
				0: metal3DataClaimName,
			},
			expectedIndexes: map[string]int{
				metal3DataClaimName: 0,
			},
		}),
	)

	var templateMeta = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: namespaceName,
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
			objects := []client.Object{}
			for _, address := range tc.datas {
				objects = append(objects, address)
			}
			for _, claim := range tc.dataClaims {
				objects = append(objects, claim)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).WithStatusSubresource(objects...).Build()
			templateMgr, err := NewDataTemplateManager(fakeClient, tc.template, logr.Discard())
			Expect(err).NotTo(HaveOccurred())

			hasData, _, err := templateMgr.UpdateDatas(context.TODO())
			if tc.expectRequeue || tc.expectError {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(hasData).To(Equal(tc.expectedNbIndexes > 0))
			Expect(tc.template.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(tc.template.Status.Indexes).To(Equal(tc.expectedIndexes))

			// get list of Metal3DataClaim objects
			dataClaimObjects := infrav1.Metal3DataClaimList{}
			opts := &client.ListOptions{}
			err = fakeClient.List(context.TODO(), &dataClaimObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			// Iterate over the Metal3DataClaim objects to check
			// if the render data points to somewhere on claims that are not
			// being deleted
			for _, claim := range dataClaimObjects.Items {
				if claim.DeletionTimestamp == nil || claim.DeletionTimestamp.IsZero() {
					Expect(claim.Status.RenderedData).NotTo(BeNil())
				}
			}

			// get list of Metal3DataClaim objects
			dataObjects := infrav1.Metal3DataList{}
			opts = &client.ListOptions{}
			err = fakeClient.List(context.TODO(), &dataObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			for _, data := range dataObjects.Items {
				if data.DeletionTimestamp == nil || data.DeletionTimestamp.IsZero() {
					correctDeletion := false
					parentClaim := data.Spec.Claim.Name
					Expect(parentClaim).NotTo(BeEmpty())
					for _, claim := range dataClaimObjects.Items {
						if parentClaim == claim.ObjectMeta.Name {
							Expect(claim.DeletionTimestamp.IsZero()).To(BeTrue())
							Expect(claim.Status.RenderedData).NotTo(BeNil())
							correctDeletion = true
						}
					}
					Expect(correctDeletion).To(BeTrue())
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
						Name:      "claim-without-status",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "orphaned-claim",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      "other-template",
							Namespace: namespaceName,
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &corev1.ObjectReference{
							Name:      "abc-2", // Doesn't matter because we are not reconciling the "other template"
							Namespace: namespaceName,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "claim-with-status",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &corev1.ObjectReference{
							Name:      "abc-1",
							Namespace: namespaceName,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "deleting-claim",
						Namespace:         namespaceName,
						DeletionTimestamp: &timeNow,
						Finalizers:        []string{"ipclaim.ipam.metal3.io"},
					},
					Spec: infrav1.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &corev1.ObjectReference{
							Name:      "abc-3",
							Namespace: namespaceName,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "deleting-claim-missmatch-name",
						Namespace:         namespaceName,
						DeletionTimestamp: &timeNow,
						Finalizers:        []string{"ipclaim.ipam.metal3.io"},
					},
					Spec: infrav1.Metal3DataClaimSpec{
						Template: corev1.ObjectReference{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &corev1.ObjectReference{
							Name:      "cdc-3",
							Namespace: namespaceName,
						},
					},
				},
			},
			datas: []*infrav1.Metal3Data{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataSpec{
						Template: corev1.ObjectReference{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
						Claim: corev1.ObjectReference{
							Name:      "claim-without-status",
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
					Spec: infrav1.Metal3DataSpec{
						Template: corev1.ObjectReference{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
						Claim: corev1.ObjectReference{
							Name:      "claim-with-status",
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
					Spec: infrav1.Metal3DataSpec{
						Template: corev1.ObjectReference{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
						Claim: corev1.ObjectReference{
							Name:      "deleting-claim",
							Namespace: namespaceName,
						},
						Index: 3,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cdc-3",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataSpec{
						Template: corev1.ObjectReference{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
						Claim: corev1.ObjectReference{
							Name:      "deleting-claim-missmatch-name",
							Namespace: namespaceName,
						},
						Index: 40,
					},
				},
			},
			expectedIndexes: map[string]int{
				"claim-without-status": 0,
				"claim-with-status":    1,
			},
			expectedNbIndexes: 2,
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
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			// get list of Metal3Data objects
			dataObjects := infrav1.Metal3DataList{}
			opts := &client.ListOptions{}
			err = fakeClient.List(context.TODO(), &dataObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(tc.expectedDatas).To(HaveLen(len(dataObjects.Items)))
			// Iterate over the Metal3Data objects to find all indexes and objects
			for _, address := range dataObjects.Items {
				Expect(tc.expectedDatas).To(ContainElement(address.Name))
				// TODO add further testing later
			}
			Expect(tc.dataClaim.Finalizers).To(HaveLen(1))

			Expect(allocatedMap).To(Equal(tc.expectedMap))
			Expect(tc.template.Status.Indexes).To(Equal(tc.expectedIndexes))
		},
		Entry("Already exists", testCaseCreateAddresses{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: map[string]int{
						metal3DataClaimName: 0,
					},
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
			},
			expectedIndexes: map[string]int{
				metal3DataClaimName: 0,
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
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
			},
			expectedIndexes: map[string]int{
				metal3DataClaimName: 0,
			},
			expectedMap: map[int]string{
				0: metal3DataClaimName,
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
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
			},
			expectedIndexes: map[string]int{
				metal3DataClaimName: 1,
				"bcd":               0,
			},
			expectedMap: map[int]string{
				0: "bcd",
				1: metal3DataClaimName,
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
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
			},
			datas: []*infrav1.Metal3Data{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: namespaceName,
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
			objects := []client.Object{}
			for _, address := range tc.datas {
				objects = append(objects, address)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupSchemeMm()).WithObjects(objects...).Build()
			templateMgr, err := NewDataTemplateManager(fakeClient, tc.template,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			allocatedMap, err := templateMgr.deleteMetal3DataAndClaim(context.TODO(), tc.dataClaim, tc.indexes)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			// get list of Metal3Data objects
			dataObjects := infrav1.Metal3DataList{}
			opts := &client.ListOptions{}
			err = fakeClient.List(context.TODO(), &dataObjects, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataObjects.Items).To(BeEmpty())

			Expect(tc.template.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(allocatedMap).To(Equal(tc.expectedMap))
			Expect(tc.template.Status.Indexes).To(Equal(tc.expectedIndexes))
			Expect(tc.dataClaim.Finalizers).To(BeEmpty())
		},
		Entry("Empty Template", testCaseDeleteDatas{
			template: &infrav1.Metal3DataTemplate{},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMeta("TestRef", "", ""),
			},
		}),
		Entry("No Deletion needed", testCaseDeleteDatas{
			template: &infrav1.Metal3DataTemplate{},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMeta("TestRef", "", ""),
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
				ObjectMeta: testObjectMeta("TestRef", "", ""),
			},
			indexes: map[int]string{
				0: "TestRef",
			},
			expectedIndexes: map[string]int{},
			expectedMap:     map[int]string{},
		}),
		Entry("Deletion needed", testCaseDeleteDatas{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta("abc", "", ""),
				Spec:       infrav1.Metal3DataTemplateSpec{},
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
		Entry("Deletion needed but data name doesn't match template", testCaseDeleteDatas{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta("abc", "", ""),
				Spec:       infrav1.Metal3DataTemplateSpec{},
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
				Status: infrav1.Metal3DataClaimStatus{
					RenderedData: &corev1.ObjectReference{
						Name: "error-42",
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
						Name: "error-42",
					},
				},
			},
		}),
	)

})
