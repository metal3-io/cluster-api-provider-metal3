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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Metal3MachineTemplate manager", func() {

	type testCaseUpdate struct {
		M3MachineTemplate *infrav1.Metal3MachineTemplate
		M3MachineList     *infrav1.Metal3MachineList
		ExpectedValue     *string
	}

	DescribeTable("Test UpdateAutomatedCleaningMode",
		func(tc testCaseUpdate) {
			objects := []runtime.Object{
				tc.M3MachineTemplate,
				tc.M3MachineList,
			}
			fakeClient := fakeclient.NewClientBuilder().WithScheme(setupSchemeMm()).WithRuntimeObjects(objects...).Build()
			templateMgr, err := NewMachineTemplateManager(fakeClient, tc.M3MachineTemplate,
				tc.M3MachineList, logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			err = templateMgr.UpdateAutomatedCleaningMode(context.TODO())

			Expect(err).NotTo(HaveOccurred())

			for i := range tc.M3MachineList.Items {
				m3m := &tc.M3MachineList.Items[i]
				key := client.ObjectKey{
					Name:      m3m.Name,
					Namespace: m3m.Namespace,
				}
				updatedM3M := infrav1.Metal3Machine{}
				err = fakeClient.Get(context.TODO(), key, &updatedM3M)
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedM3M.Spec.AutomatedCleaningMode).To(Equal(tc.ExpectedValue))
			}
		},
		Entry("Disk cleaning disabled", testCaseUpdate{
			ExpectedValue: ptr.To("disabled"),
			M3MachineTemplate: &infrav1.Metal3MachineTemplate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "Metal3MachineTemplate",
				},
				ObjectMeta: testObjectMeta("abc", "foo", ""),
				Spec: infrav1.Metal3MachineTemplateSpec{
					Template: infrav1.Metal3MachineTemplateResource{
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To(infrav1.CleaningModeDisabled),
						},
					},
				},
			},
			M3MachineList: &infrav1.Metal3MachineList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "Metal3MachineList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []infrav1.Metal3Machine{
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-1",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": infrav1.ClonedFromGroupKind,
							},
						},
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To(infrav1.CleaningModeMetadata),
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-2",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": infrav1.ClonedFromGroupKind,
							},
						},
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To(infrav1.CleaningModeMetadata),
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-3",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": infrav1.ClonedFromGroupKind,
							},
						},
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To(infrav1.CleaningModeMetadata),
						},
					},
				},
			},
		}),
		Entry("Disk cleaning enabled", testCaseUpdate{
			ExpectedValue: ptr.To("metadata"),
			M3MachineTemplate: &infrav1.Metal3MachineTemplate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "Metal3MachineTemplate",
				},
				ObjectMeta: testObjectMeta("abc", "foo", ""),
				Spec: infrav1.Metal3MachineTemplateSpec{
					Template: infrav1.Metal3MachineTemplateResource{
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To(infrav1.CleaningModeMetadata),
						},
					},
				},
			},
			M3MachineList: &infrav1.Metal3MachineList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "Metal3MachineList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []infrav1.Metal3Machine{
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-1",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": infrav1.ClonedFromGroupKind,
							},
						},
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To(infrav1.CleaningModeDisabled),
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-2",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": infrav1.ClonedFromGroupKind,
							},
						},
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To(infrav1.CleaningModeDisabled),
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-3",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": infrav1.ClonedFromGroupKind,
							},
						},
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To(infrav1.CleaningModeDisabled),
						},
					},
				},
			},
		}),
		Entry("Don't synchronize M3Machines which are not part of the M3MTemplate ", testCaseUpdate{
			ExpectedValue: ptr.To("metadata"),
			M3MachineTemplate: &infrav1.Metal3MachineTemplate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "Metal3MachineTemplate",
				},
				ObjectMeta: testObjectMeta("abc", "foo", ""),
				Spec: infrav1.Metal3MachineTemplateSpec{
					Template: infrav1.Metal3MachineTemplateResource{
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To("disabled"),
						},
					},
				},
			},
			M3MachineList: &infrav1.Metal3MachineList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "Metal3MachineList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []infrav1.Metal3Machine{
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-3",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "xyz",
								"cluster.x-k8s.io/cloned-from-groupkind": infrav1.ClonedFromGroupKind,
							},
						},
						Spec: infrav1.Metal3MachineSpec{
							AutomatedCleaningMode: ptr.To(infrav1.CleaningModeMetadata),
						},
					},
				},
			},
		}),
	)
})
