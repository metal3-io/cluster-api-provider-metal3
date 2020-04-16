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

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var timeNow = metav1.Now()

var _ = Describe("Metal3DataTemplate manager", func() {
	DescribeTable("Test Finalizers",
		func(dataTemplate *capm3.Metal3DataTemplate) {
			machineMgr, err := NewDataTemplateManager(nil, dataTemplate,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			machineMgr.SetFinalizer()

			Expect(dataTemplate.ObjectMeta.Finalizers).To(ContainElement(
				capm3.DataTemplateFinalizer,
			))

			machineMgr.UnsetFinalizer()

			Expect(dataTemplate.ObjectMeta.Finalizers).NotTo(ContainElement(
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

	type testRecreateStatus struct {
		dataTemplate      *capm3.Metal3DataTemplate
		data              []*capm3.Metal3Data
		expectedIndexes   map[string]string
		expectedDataNames map[string]string
	}

	DescribeTable("Test RecreateStatus",
		func(tc testRecreateStatus) {
			objects := []runtime.Object{}
			for _, data := range tc.data {
				objects = append(objects, data)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			dataMgr, err := NewDataTemplateManager(c, tc.dataTemplate,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = dataMgr.RecreateStatusConditionally(context.TODO())
			Expect(err).NotTo(HaveOccurred())
			Expect(tc.dataTemplate.Status.Indexes).To(Equal(tc.expectedIndexes))
			Expect(tc.dataTemplate.Status.DataNames).To(Equal(tc.expectedDataNames))
			Expect(tc.dataTemplate.Status.LastUpdated.IsZero()).To(BeFalse())
		},
		Entry("No data", testRecreateStatus{
			dataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
				},
			},
			expectedIndexes:   map[string]string{},
			expectedDataNames: map[string]string{},
		}),
		Entry("data present", testRecreateStatus{
			dataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns",
					Name:      "abc",
				},
			},
			data: []*capm3.Metal3Data{
				&capm3.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: "myns",
					},
					Spec: capm3.Metal3DataSpec{
						Index: 0,
						DataTemplate: &corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Metal3Machine: &corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
				},
				&capm3.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bbc-1",
						Namespace: "myns",
					},
					Spec: capm3.Metal3DataSpec{
						Index: 1,
						DataTemplate: &corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
						Metal3Machine: &corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
					},
				},
				&capm3.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-2",
						Namespace: "myns",
					},
					Spec: capm3.Metal3DataSpec{
						Index:        2,
						DataTemplate: nil,
						Metal3Machine: &corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
				},
				&capm3.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: "myns",
					},
					Spec: capm3.Metal3DataSpec{
						Index: 3,
						DataTemplate: &corev1.ObjectReference{
							Namespace: "myns",
						},
						Metal3Machine: nil,
					},
				},
			},
			expectedIndexes: map[string]string{
				"0": "abc",
			},
			expectedDataNames: map[string]string{
				"abc": "abc-0",
			},
		}),
		Entry("No recreation of the status", testRecreateStatus{
			dataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns",
					Name:      "abc",
				},
				Status: capm3.Metal3DataTemplateStatus{
					LastUpdated: &timeNow,
					Indexes:     map[string]string{},
					DataNames:   map[string]string{},
				},
			},
			data: []*capm3.Metal3Data{
				&capm3.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: "myns",
					},
					Spec: capm3.Metal3DataSpec{
						Index: 0,
						DataTemplate: &corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Metal3Machine: &corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
				},
			},
			expectedIndexes:   map[string]string{},
			expectedDataNames: map[string]string{},
		}),
	)

	type testCaseCreateDatas struct {
		DataTemplate      *capm3.Metal3DataTemplate
		Data              []*capm3.Metal3Data
		Metal3Machine     *capm3.Metal3Machine
		ExpectRequeue     bool
		ExpectError       bool
		ExpectedData      []string
		expectedIndexes   map[string]string
		expectedDataNames map[string]string
	}

	DescribeTable("Test CreateDatas",
		func(tc testCaseCreateDatas) {
			objects := []runtime.Object{}
			for _, data := range tc.Data {
				objects = append(objects, data)
			}
			if tc.Metal3Machine != nil {
				objects = append(objects, tc.Metal3Machine)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			dataMgr, err := NewDataTemplateManager(c, tc.DataTemplate,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = dataMgr.CreateDatas(context.TODO())
			if tc.ExpectRequeue || tc.ExpectError {
				Expect(err).To(HaveOccurred())
				if tc.ExpectRequeue {
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
			err = c.List(context.TODO(), &dataObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(tc.ExpectedData)).To(Equal(len(dataObjects.Items)))
			// Iterate over the Metal3Data objects to find all indexes and objects
			for _, dataObject := range dataObjects.Items {
				Expect(tc.ExpectedData).To(ContainElement(dataObject.Name))
			}

			if !tc.ExpectError {
				Expect(tc.DataTemplate.Status.LastUpdated.IsZero()).To(BeFalse())
			}
			Expect(tc.DataTemplate.Status.Indexes).To(Equal(tc.expectedIndexes))
			Expect(tc.DataTemplate.Status.DataNames).To(Equal(tc.expectedDataNames))
		},
		Entry("No OwnerRefs", testCaseCreateDatas{
			DataTemplate: &capm3.Metal3DataTemplate{},
		}),
		Entry("wrong OwnerRef", testCaseCreateDatas{
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: "cluster.x-k8s.io/v1alpha3",
						},
					},
				},
			},
		}),
		Entry("Already exists", testCaseCreateDatas{
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Metal3Machine",
							APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
						},
					},
				},
				Status: capm3.Metal3DataTemplateStatus{
					DataNames: map[string]string{
						"abc": "foo-0",
					},
				},
			},
			expectedDataNames: map[string]string{"abc": "foo-0"},
		}),
		Entry("Metal3Machine does not exist", testCaseCreateDatas{
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Metal3Machine",
							APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
						},
					},
				},
			},
		}),
		Entry("Metal3Machine exists, missing cluster label", testCaseCreateDatas{
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Metal3Machine",
							APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
						},
					},
				},
			},
			Metal3Machine: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			ExpectError: true,
		}),
		Entry("Metal3Machine exists", testCaseCreateDatas{
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					Labels: map[string]string{
						capi.ClusterLabelName: clusterName,
					},
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Metal3Machine",
							APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
						},
					},
				},
			},
			Metal3Machine: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			ExpectedData: []string{
				"abc-0",
			},
			expectedIndexes:   map[string]string{"0": "abc"},
			expectedDataNames: map[string]string{"abc": "abc-0"},
		}),
		Entry("Metal3Machine exists, conflict", testCaseCreateDatas{
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					Labels: map[string]string{
						capi.ClusterLabelName: clusterName,
					},
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Metal3Machine",
							APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
						},
					},
				},
			},
			Metal3Machine: &capm3.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				Spec: capm3.Metal3MachineSpec{
					DataTemplate: &corev1.ObjectReference{
						Name: "abc",
					},
				},
			},
			ExpectedData: []string{
				"abc-0",
			},
			Data: []*capm3.Metal3Data{
				&capm3.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-0",
					},
					Spec: capm3.Metal3DataSpec{
						Index: 0,
						DataTemplate: &corev1.ObjectReference{
							Name: "abc",
						},
						Metal3Machine: &corev1.ObjectReference{
							Name: "abc",
						},
					},
				},
			},
			ExpectRequeue:     true,
			expectedIndexes:   map[string]string{"0": "abc"},
			expectedDataNames: map[string]string{"abc": "abc-0"},
		}),
	)

	type testCaseDeleteDatas struct {
		DataTemplate      *capm3.Metal3DataTemplate
		Data              []*capm3.Metal3Data
		expectedIndexes   map[string]string
		expectedDataNames map[string]string
	}

	DescribeTable("Test CreateDatas",
		func(tc testCaseDeleteDatas) {
			objects := []runtime.Object{}
			for _, data := range tc.Data {
				objects = append(objects, data)
			}
			c := fakeclient.NewFakeClientWithScheme(setupSchemeMm(), objects...)
			dataMgr, err := NewDataTemplateManager(c, tc.DataTemplate,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			err = dataMgr.DeleteDatas(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			// get list of Metal3Data objects
			dataObjects := capm3.Metal3DataList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &dataObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(dataObjects.Items)).To(Equal(0))

			Expect(tc.DataTemplate.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(tc.DataTemplate.Status.Indexes).To(Equal(tc.expectedIndexes))
			Expect(tc.DataTemplate.Status.DataNames).To(Equal(tc.expectedDataNames))
		},
		Entry("Empty DataTemplate", testCaseDeleteDatas{
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
		}),
		Entry("No Deletion needed", testCaseDeleteDatas{
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: "cluster.x-k8s.io/v1alpha3",
						},
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Metal3Machine",
							APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
						},
					},
				},
				Status: capm3.Metal3DataTemplateStatus{
					DataNames: map[string]string{
						"abc": "abc-0",
					},
					Indexes: map[string]string{
						"0": "abc",
					},
				},
			},
			expectedIndexes:   map[string]string{"0": "abc"},
			expectedDataNames: map[string]string{"abc": "abc-0"},
		}),
		Entry("Deletion needed, not found", testCaseDeleteDatas{
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: "cluster.x-k8s.io/v1alpha3",
						},
					},
				},
				Status: capm3.Metal3DataTemplateStatus{
					DataNames: map[string]string{
						"abc": "abc-0",
					},
					Indexes: map[string]string{
						"0": "abc",
					},
				},
			},
			expectedIndexes:   map[string]string{},
			expectedDataNames: map[string]string{},
		}),
		Entry("Deletion needed", testCaseDeleteDatas{
			DataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: "cluster.x-k8s.io/v1alpha3",
						},
					},
				},
				Status: capm3.Metal3DataTemplateStatus{
					DataNames: map[string]string{
						"abc": "abc-0",
					},
					Indexes: map[string]string{
						"0": "abc",
					},
				},
			},
			expectedIndexes:   map[string]string{},
			expectedDataNames: map[string]string{},
			Data: []*capm3.Metal3Data{
				&capm3.Metal3Data{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-0",
					},
				},
			},
		}),
	)

	type testCaseDeleteReady struct {
		dataTemplate *capm3.Metal3DataTemplate
		expectReady  bool
	}
	DescribeTable("Test CreateDatas",
		func(tc testCaseDeleteReady) {
			dataMgr, err := NewDataTemplateManager(nil, tc.dataTemplate, klogr.New())
			Expect(err).NotTo(HaveOccurred())

			ready, err := dataMgr.DeleteReady()
			Expect(err).NotTo(HaveOccurred())
			if tc.expectReady {
				Expect(ready).To(BeTrue())
			} else {
				Expect(ready).To(BeFalse())
			}
		},
		Entry("Ready", testCaseDeleteReady{
			dataTemplate: &capm3.Metal3DataTemplate{},
			expectReady:  true,
		}),
		Entry("Ready with OwnerRefs", testCaseDeleteReady{
			dataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Machine",
							APIVersion: "cluster.x-k8s.io/v1alpha3",
						},
					},
				},
			},
			expectReady: true,
		}),
		Entry("Not Ready with OwnerRefs", testCaseDeleteReady{
			dataTemplate: &capm3.Metal3DataTemplate{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Name:       "abc",
							Kind:       "Metal3Machine",
							APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
						},
					},
				},
			},
			expectReady: false,
		}),
	)
})
