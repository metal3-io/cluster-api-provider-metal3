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

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utils "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Metal3MachineTemplate manager", func() {

	type testCaseUpdate struct {
		M3MachineTemplate *capm3.Metal3MachineTemplate
		M3MachineList     *capm3.Metal3MachineList
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
				updatedM3M := capm3.Metal3Machine{}
				err = fakeClient.Get(context.TODO(), key, &updatedM3M)
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedM3M.Spec.AutomatedCleaningMode).To(Equal(tc.ExpectedValue))
			}
		},
		Entry("Disk cleaning disabled", testCaseUpdate{
			ExpectedValue: utils.StringPtr("disabled"),
			M3MachineTemplate: &capm3.Metal3MachineTemplate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: capm3.GroupVersion.String(),
					Kind:       "Metal3MachineTemplate",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "foo",
				},
				Spec: capm3.Metal3MachineTemplateSpec{
					Template: capm3.Metal3MachineTemplateResource{
						Spec: capm3.Metal3MachineSpec{
							AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeDisabled),
						},
					},
				},
			},
			M3MachineList: &capm3.Metal3MachineList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: capm3.GroupVersion.String(),
					Kind:       "Metal3MachineList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []capm3.Metal3Machine{
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-1",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": capm3.ClonedFromGroupKind,
							},
						},
						Spec: capm3.Metal3MachineSpec{
							AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeMetadata),
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-2",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": capm3.ClonedFromGroupKind,
							},
						},
						Spec: capm3.Metal3MachineSpec{
							AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeMetadata),
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-3",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": capm3.ClonedFromGroupKind,
							},
						},
						Spec: capm3.Metal3MachineSpec{
							AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeMetadata),
						},
					},
				},
			},
		}),
		Entry("Disk cleaning enabled", testCaseUpdate{
			ExpectedValue: utils.StringPtr("metadata"),
			M3MachineTemplate: &capm3.Metal3MachineTemplate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: capm3.GroupVersion.String(),
					Kind:       "Metal3MachineTemplate",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "foo",
				},
				Spec: capm3.Metal3MachineTemplateSpec{
					Template: capm3.Metal3MachineTemplateResource{
						Spec: capm3.Metal3MachineSpec{
							AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeMetadata),
						},
					},
				},
			},
			M3MachineList: &capm3.Metal3MachineList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: capm3.GroupVersion.String(),
					Kind:       "Metal3MachineList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []capm3.Metal3Machine{
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-1",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": capm3.ClonedFromGroupKind,
							},
						},
						Spec: capm3.Metal3MachineSpec{
							AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeDisabled),
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-2",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": capm3.ClonedFromGroupKind,
							},
						},
						Spec: capm3.Metal3MachineSpec{
							AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeDisabled),
						},
					},
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-3",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "abc",
								"cluster.x-k8s.io/cloned-from-groupkind": capm3.ClonedFromGroupKind,
							},
						},
						Spec: capm3.Metal3MachineSpec{
							AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeDisabled),
						},
					},
				},
			},
		}),
		Entry("Don't synchronize M3Machines which are not part of the M3MTemplate ", testCaseUpdate{
			ExpectedValue: utils.StringPtr("metadata"),
			M3MachineTemplate: &capm3.Metal3MachineTemplate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: capm3.GroupVersion.String(),
					Kind:       "Metal3MachineTemplate",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "foo",
				},
				Spec: capm3.Metal3MachineTemplateSpec{
					Template: capm3.Metal3MachineTemplateResource{
						Spec: capm3.Metal3MachineSpec{
							AutomatedCleaningMode: utils.StringPtr("disabled"),
						},
					},
				},
			},
			M3MachineList: &capm3.Metal3MachineList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: capm3.GroupVersion.String(),
					Kind:       "Metal3MachineList",
				},
				ListMeta: metav1.ListMeta{},
				Items: []capm3.Metal3Machine{
					{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-3",
							Namespace: "foo",
							Annotations: map[string]string{
								"cluster.x-k8s.io/cloned-from-name":      "xyz",
								"cluster.x-k8s.io/cloned-from-groupkind": capm3.ClonedFromGroupKind,
							},
						},
						Spec: capm3.Metal3MachineSpec{
							AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeMetadata),
						},
					},
				},
			},
		}),
	)
})
