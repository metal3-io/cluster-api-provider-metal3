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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

	type testGetIndexes struct {
		template        *infrav1.Metal3DataTemplate
		indexes         []*infrav1.Metal3Data
		expectError     bool
		expectedMap     []infrav1.IndexEntry
		expectedIndexes []infrav1.IndexEntry
	}

	DescribeTable("Test getIndexes",
		func(tc testGetIndexes) {
			objects := make([]client.Object, 0, len(tc.indexes))
			for _, address := range tc.indexes {
				objects = append(objects, address)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
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
			expectedMap:     []infrav1.IndexEntry{},
			expectedIndexes: []infrav1.IndexEntry{},
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
						Index:    ptr.To(int32(0)),
						Template: testMetal3ObjectReference(metal3DataTemplateName),
						Claim:    testMetal3ObjectReference(metal3DataClaimName),
					},
				},
				{
					ObjectMeta: testObjectMeta("bbc-1", namespaceName, ""),
					Spec: infrav1.Metal3DataSpec{
						Index: ptr.To(int32(1)),
						Template: &infrav1.Metal3ObjectRef{
							Name:      "bbc",
							Namespace: namespaceName,
						},
						Claim: &infrav1.Metal3ObjectRef{
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
						Index:    ptr.To(int32(2)),
						Template: &infrav1.Metal3ObjectRef{},
						Claim:    testMetal3ObjectReference(metal3DataClaimName),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataSpec{
						Index: ptr.To(int32(3)),
						Template: &infrav1.Metal3ObjectRef{
							Namespace: namespaceName,
						},
						Claim: &infrav1.Metal3ObjectRef{},
					},
				},
			},
			expectedMap: []infrav1.IndexEntry{
				{
					Name:  metal3DataClaimName,
					Index: ptr.To(int32(0)),
				},
			},
			expectedIndexes: []infrav1.IndexEntry{
				{
					Name:  metal3DataClaimName,
					Index: ptr.To(int32(0)),
				},
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
		expectedIndexes   []infrav1.IndexEntry
	}

	DescribeTable("Test UpdateDatas",
		func(tc testCaseUpdateDatas) {
			objects := make([]client.Object, 0, len(tc.datas)+len(tc.dataClaims))
			for _, address := range tc.datas {
				objects = append(objects, address)
			}
			for _, claim := range tc.dataClaims {
				objects = append(objects, claim)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).WithStatusSubresource(objects...).Build()
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
			expectedIndexes: []infrav1.IndexEntry{},
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
						Template: &infrav1.Metal3ObjectRef{
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
						Template: &infrav1.Metal3ObjectRef{
							Name:      "other-template",
							Namespace: namespaceName,
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &infrav1.Metal3ObjectRef{
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
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &infrav1.Metal3ObjectRef{
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
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &infrav1.Metal3ObjectRef{
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
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &infrav1.Metal3ObjectRef{
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
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
						Claim: &infrav1.Metal3ObjectRef{
							Name:      "claim-without-status",
							Namespace: namespaceName,
						},
						Index: ptr.To(int32(0)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-1",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataSpec{
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
						Claim: &infrav1.Metal3ObjectRef{
							Name:      "claim-with-status",
							Namespace: namespaceName,
						},
						Index: ptr.To(int32(1)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataSpec{
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
						Claim: &infrav1.Metal3ObjectRef{
							Name:      "deleting-claim",
							Namespace: namespaceName,
						},
						Index: ptr.To(int32(3)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cdc-3",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataSpec{
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
						Claim: &infrav1.Metal3ObjectRef{
							Name:      "deleting-claim-missmatch-name",
							Namespace: namespaceName,
						},
						Index: ptr.To(int32(40)),
					},
				},
			},
			expectedIndexes: []infrav1.IndexEntry{
				{
					Name:  "claim-without-status",
					Index: ptr.To(int32(0)),
				},
				{
					Name:  "claim-with-status",
					Index: ptr.To(int32(1)),
				},
			},
			expectedNbIndexes: 2,
		}),
		// A single Metal3DataTemplate can be consumed by more than one Cluster.
		// Index allocation is scoped to the template, not to a Cluster: claims
		// coming from different Clusters share the same index space. Existing
		// indexes from every Cluster are tracked together, and a new claim from
		// any Cluster continues the shared sequence without colliding.
		Entry("Single template shared by two clusters", testCaseUpdateDatas{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
			},
			dataClaims: []*infrav1.Metal3DataClaim{
				// Already-rendered claim from cluster-a (index 0).
				{
					ObjectMeta: func() metav1.ObjectMeta {
						om := testObjectMetaWithOR("cluster-a-claim-0", metal3machineName)
						om.Labels = map[string]string{clusterv1.ClusterNameLabel: "cluster-a"}
						return om
					}(),
					Spec: infrav1.Metal3DataClaimSpec{
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &infrav1.Metal3ObjectRef{
							Name:      "abc-0",
							Namespace: namespaceName,
						},
					},
				},
				// Already-rendered claim from cluster-b (index 1) on the same template.
				{
					ObjectMeta: func() metav1.ObjectMeta {
						om := testObjectMetaWithOR("cluster-b-claim-0", metal3machineName)
						om.Labels = map[string]string{clusterv1.ClusterNameLabel: "cluster-b"}
						return om
					}(),
					Spec: infrav1.Metal3DataClaimSpec{
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
					},
					Status: infrav1.Metal3DataClaimStatus{
						RenderedData: &infrav1.Metal3ObjectRef{
							Name:      "abc-1",
							Namespace: namespaceName,
						},
					},
				},
				// New, unrendered claim from cluster-a. It must get the next free
				// index in the template-wide space (2), not reuse cluster-b's index.
				{
					ObjectMeta: func() metav1.ObjectMeta {
						om := testObjectMetaWithOR("cluster-a-claim-1", metal3machineName)
						om.Labels = map[string]string{clusterv1.ClusterNameLabel: "cluster-a"}
						return om
					}(),
					Spec: infrav1.Metal3DataClaimSpec{
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
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
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
						Claim: &infrav1.Metal3ObjectRef{
							Name:      "cluster-a-claim-0",
							Namespace: namespaceName,
						},
						Index: ptr.To(int32(0)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-1",
						Namespace: namespaceName,
					},
					Spec: infrav1.Metal3DataSpec{
						Template: &infrav1.Metal3ObjectRef{
							Name:      templateMeta.Name,
							Namespace: namespaceName,
						},
						Claim: &infrav1.Metal3ObjectRef{
							Name:      "cluster-b-claim-0",
							Namespace: namespaceName,
						},
						Index: ptr.To(int32(1)),
					},
				},
			},
			// getIndexes returns the sorted, template-wide index space across
			// both clusters (0, 1); the new cluster-a claim then extends it to 2.
			expectedIndexes: []infrav1.IndexEntry{
				{
					Name:  "cluster-a-claim-0",
					Index: ptr.To(int32(0)),
				},
				{
					Name:  "cluster-b-claim-0",
					Index: ptr.To(int32(1)),
				},
				{
					Name:  "cluster-a-claim-1",
					Index: ptr.To(int32(2)),
				},
			},
			expectedNbIndexes: 2,
		}),
	)

	type testCaseCreateAddresses struct {
		template        *infrav1.Metal3DataTemplate
		dataClaim       *infrav1.Metal3DataClaim
		datas           []*infrav1.Metal3Data
		indexes         []infrav1.IndexEntry
		expectRequeue   bool
		expectError     bool
		expectedDatas   []string
		expectedMap     []infrav1.IndexEntry
		expectedIndexes []infrav1.IndexEntry
	}

	DescribeTable("Test CreateAddresses",
		func(tc testCaseCreateAddresses) {
			objects := make([]client.Object, 0, len(tc.datas))
			for _, address := range tc.datas {
				objects = append(objects, address)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
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
				// Metal3Data created by createData must be owned by the
				// Metal3DataClaim (controlling) and the Metal3Machine, but not
				// by the Metal3DataTemplate, so that they are garbage collected
				// with the Cluster. Skip pre-existing fixtures used to trigger
				// requeue/error paths, which have no owner references.
				if tc.expectRequeue || tc.expectError {
					continue
				}
				var claimOwner, machineOwner *metav1.OwnerReference
				for i := range address.OwnerReferences {
					ref := &address.OwnerReferences[i]
					Expect(ref.Kind).NotTo(Equal("Metal3DataTemplate"))
					switch ref.Kind {
					case metal3DataClaimKind:
						claimOwner = ref
					case metal3MachineKind:
						machineOwner = ref
					}
				}
				Expect(address.OwnerReferences).To(HaveLen(2))
				Expect(claimOwner).NotTo(BeNil())
				Expect(claimOwner.Controller).To(Equal(ptr.To(true)))
				Expect(claimOwner.Name).To(Equal(tc.dataClaim.Name))
				Expect(machineOwner).NotTo(BeNil())
			}
			Expect(tc.dataClaim.Finalizers).To(HaveLen(1))

			Expect(allocatedMap).To(Equal(tc.expectedMap))
			Expect(tc.template.Status.Indexes).To(Equal(tc.expectedIndexes))
		},
		Entry("Already exists", testCaseCreateAddresses{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: []infrav1.IndexEntry{
						{
							Name:  metal3DataClaimName,
							Index: ptr.To(int32(0)),
						},
					},
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
			},
			expectedIndexes: []infrav1.IndexEntry{
				{
					Name:  metal3DataClaimName,
					Index: ptr.To(int32(0)),
				},
			},
		}),
		Entry("Not allocated yet, first", testCaseCreateAddresses{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: []infrav1.IndexEntry{},
				},
			},
			indexes: []infrav1.IndexEntry{},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
			},
			expectedIndexes: []infrav1.IndexEntry{
				{
					Name:  metal3DataClaimName,
					Index: ptr.To(int32(0)),
				},
			},
			expectedMap: []infrav1.IndexEntry{
				{
					Name:  metal3DataClaimName,
					Index: ptr.To(int32(0)),
				},
			},
			expectedDatas: []string{"abc-0"},
		}),
		Entry("Not allocated yet, second", testCaseCreateAddresses{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: []infrav1.IndexEntry{
						{
							Name:  "bcd",
							Index: ptr.To(int32(0)),
						},
					},
				},
			},
			indexes: []infrav1.IndexEntry{
				{
					Name:  "bcd",
					Index: ptr.To(int32(0)),
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
			},
			expectedIndexes: []infrav1.IndexEntry{
				{
					Name:  "bcd",
					Index: ptr.To(int32(0)),
				},
				{
					Name:  metal3DataClaimName,
					Index: ptr.To(int32(1)),
				},
			},
			expectedMap: []infrav1.IndexEntry{
				{
					Name:  "bcd",
					Index: ptr.To(int32(0)),
				},
				{
					Name:  metal3DataClaimName,
					Index: ptr.To(int32(1)),
				},
			},
			expectedDatas: []string{"abc-1"},
		}),
		Entry("Not allocated yet, conflict", testCaseCreateAddresses{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: []infrav1.IndexEntry{},
				},
			},
			indexes: []infrav1.IndexEntry{},
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
						Index: ptr.To(int32(0)),
						Template: &infrav1.Metal3ObjectRef{
							Name: "abc",
						},
						Claim: &infrav1.Metal3ObjectRef{
							Name: "bcd",
						},
					},
				},
			},
			expectedIndexes: []infrav1.IndexEntry{},
			expectedMap:     []infrav1.IndexEntry{},
			expectedDatas:   []string{"abc-0"},
			expectRequeue:   true,
		}),
		Entry("Fill index gap", testCaseCreateAddresses{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: templateMeta,
				Spec:       infrav1.Metal3DataTemplateSpec{},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: []infrav1.IndexEntry{
						{
							Name:  "bcd",
							Index: ptr.To(int32(0)),
						},
						{
							Name:  "efg",
							Index: ptr.To(int32(2)),
						},
						{
							Name:  "hij",
							Index: ptr.To(int32(3)),
						},
					},
				},
			},
			indexes: []infrav1.IndexEntry{
				{
					Name:  "bcd",
					Index: ptr.To(int32(0)),
				},
				{
					Name:  "efg",
					Index: ptr.To(int32(2)),
				},
				{
					Name:  "hij",
					Index: ptr.To(int32(3)),
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMetaWithOR(metal3DataClaimName, metal3machineName),
			},
			expectedIndexes: []infrav1.IndexEntry{
				{
					Name:  "bcd",
					Index: ptr.To(int32(0)),
				},
				{
					Name:  metal3DataClaimName,
					Index: ptr.To(int32(1)),
				},
				{
					Name:  "efg",
					Index: ptr.To(int32(2)),
				},
				{
					Name:  "hij",
					Index: ptr.To(int32(3)),
				},
			},
			expectedMap: []infrav1.IndexEntry{
				{
					Name:  "bcd",
					Index: ptr.To(int32(0)),
				},
				{
					Name:  metal3DataClaimName,
					Index: ptr.To(int32(1)),
				},
				{
					Name:  "efg",
					Index: ptr.To(int32(2)),
				},
				{
					Name:  "hij",
					Index: ptr.To(int32(3)),
				},
			},
			expectedDatas: []string{"abc-1"},
		}),
	)

	type testCaseDeleteDatas struct {
		template        *infrav1.Metal3DataTemplate
		dataClaim       *infrav1.Metal3DataClaim
		datas           []*infrav1.Metal3Data
		indexes         []infrav1.IndexEntry
		expectedMap     []infrav1.IndexEntry
		expectedIndexes []infrav1.IndexEntry
		expectError     bool
	}

	DescribeTable("Test DeleteAddresses",
		func(tc testCaseDeleteDatas) {
			objects := make([]client.Object, 0, len(tc.datas))
			for _, address := range tc.datas {
				objects = append(objects, address)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
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
			expectedMap: []infrav1.IndexEntry{
				{
					Name:  "abcd",
					Index: ptr.To(int32(0)),
				},
			},
			indexes: []infrav1.IndexEntry{
				{
					Name:  "abcd",
					Index: ptr.To(int32(0)),
				},
			},
		}),
		Entry("Deletion needed, not found", testCaseDeleteDatas{
			template: &infrav1.Metal3DataTemplate{
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: []infrav1.IndexEntry{
						{
							Name:  "TestRef",
							Index: ptr.To(int32(0)),
						},
					},
				},
			},
			dataClaim: &infrav1.Metal3DataClaim{
				ObjectMeta: testObjectMeta("TestRef", "", ""),
			},
			indexes: []infrav1.IndexEntry{
				{
					Name:  "TestRef",
					Index: ptr.To(int32(0)),
				},
			},
			expectedIndexes: []infrav1.IndexEntry{},
			expectedMap:     []infrav1.IndexEntry{},
		}),
		Entry("Deletion needed", testCaseDeleteDatas{
			template: &infrav1.Metal3DataTemplate{
				ObjectMeta: testObjectMeta("abc", "", ""),
				Spec:       infrav1.Metal3DataTemplateSpec{},
				Status: infrav1.Metal3DataTemplateStatus{
					Indexes: []infrav1.IndexEntry{
						{
							Name:  "TestRef",
							Index: ptr.To(int32(0)),
						},
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
			indexes: []infrav1.IndexEntry{
				{
					Name:  "TestRef",
					Index: ptr.To(int32(0)),
				},
			},
			expectedMap:     []infrav1.IndexEntry{},
			expectedIndexes: []infrav1.IndexEntry{},
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
					Indexes: []infrav1.IndexEntry{
						{
							Name:  "TestRef",
							Index: ptr.To(int32(0)),
						},
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
					RenderedData: &infrav1.Metal3ObjectRef{
						Name: "error-42",
					},
				},
			},
			indexes: []infrav1.IndexEntry{
				{
					Name:  "TestRef",
					Index: ptr.To(int32(0)),
				},
			},
			expectedMap:     []infrav1.IndexEntry{},
			expectedIndexes: []infrav1.IndexEntry{},
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
