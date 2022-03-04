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

package controllers

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utils "k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type commonTestCase struct {
	testRequest                       ctrl.Request
	expectedResult                    ctrl.Result
	expectedError                     *string
	m3mTemplate                       *capm3.Metal3MachineTemplate
	shouldUpdateAutomatedCleaningMode bool
}

type reconcileTemplateTestCase struct {
	common                               commonTestCase
	m3mTemplateCantBeFound               bool
	failedToCreateMachineTemplateManager bool
	m3mTemplateIsPaused                  bool
}

type reconcileTemplateNormalTestCase struct {
	common                            commonTestCase
	failedUpdateAutomatedCleaningMode bool
}

var _ = Describe("Metal3MachineTemplate controller", func() {
	var mockController *gomock.Controller
	var testReconciler *Metal3MachineTemplateReconciler
	var fakeClientBuilder *fake.ClientBuilder
	var fakeClient client.WithWatch
	var m *baremetal_mocks.MockTemplateManagerInterface
	var mf *baremetal_mocks.MockManagerFactoryInterface
	var objects []client.Object
	namespace := "foo"
	name := "abc"
	templateMgrErrorMsg := "failed to create helper for managing the templateMgr"
	defaultTestRequest := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      metal3machineTemplateName,
			Namespace: namespaceName,
		},
	}
	type TestCaseM3MtoM3MT struct {
		M3Machine     *capm3.Metal3Machine
		M3MTemplate   *capm3.Metal3MachineTemplate
		ExpectRequest bool
	}
	DescribeTable("Metal3Machine To Metal3MachineTemplate tests",
		func(tc TestCaseM3MtoM3MT) {
			r := Metal3MachineTemplateReconciler{}
			obj := client.Object(tc.M3Machine)
			reqs := r.Metal3MachinesToMetal3MachineTemplate(obj)

			if tc.ExpectRequest {
				Expect(len(reqs)).To(Equal(1), "Expected 1 request, found %d", len(reqs))
				Expect(tc.M3Machine.Annotations[clonedFromName]).To(Equal(tc.M3MTemplate.Name))
				Expect(tc.M3Machine.Annotations[clonedFromGroupKind]).To(Equal(capm3.ClonedFromGroupKind))
				Expect(tc.M3Machine.Namespace).To(Equal(tc.M3MTemplate.Namespace))
			} else {
				Expect(len(reqs)).To(Equal(0), "Expected 0 request, found %d", len(reqs))
			}
		},
		Entry("Reconciliation should not be requested due to missing reference to a template",
			TestCaseM3MtoM3MT{
				M3Machine: &capm3.Metal3Machine{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-1",
						Namespace: "bar",
						Annotations: map[string]string{
							baremetal.HostAnnotation: namespaceName + "/myhost",
						},
					},
					Spec: capm3.Metal3MachineSpec{
						AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeDisabled),
					},
				},
				M3MTemplate: &capm3.Metal3MachineTemplate{
					TypeMeta: metav1.TypeMeta{
						APIVersion: capm3.GroupVersion.String(),
						Kind:       "Metal3MachineTemplate",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Spec: capm3.Metal3MachineTemplateSpec{
						Template: capm3.Metal3MachineTemplateResource{
							Spec: capm3.Metal3MachineSpec{
								AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeDisabled),
							},
						},
					},
				},
				ExpectRequest: false,
			},
		),
		Entry("Reconciliation should be requested",
			TestCaseM3MtoM3MT{
				M3Machine: &capm3.Metal3Machine{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-1",
						Namespace: namespace,
						Annotations: map[string]string{
							"cluster.x-k8s.io/cloned-from-name":      name,
							"cluster.x-k8s.io/cloned-from-groupkind": capm3.ClonedFromGroupKind,
						},
					},
					Spec: capm3.Metal3MachineSpec{
						AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeDisabled),
					},
				},
				M3MTemplate: &capm3.Metal3MachineTemplate{
					TypeMeta: metav1.TypeMeta{
						APIVersion: capm3.GroupVersion.String(),
						Kind:       "Metal3MachineTemplate",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Spec: capm3.Metal3MachineTemplateSpec{
						Template: capm3.Metal3MachineTemplateResource{
							Spec: capm3.Metal3MachineSpec{
								AutomatedCleaningMode: utils.StringPtr(capm3.CleaningModeDisabled),
							},
						},
					},
				},
				ExpectRequest: true,
			},
		),
	)
	DescribeTable("Metal3MachineTemplate Reconcile test",
		func(tc reconcileTemplateTestCase) {
			mockController = gomock.NewController(GinkgoT())
			m = baremetal_mocks.NewMockTemplateManagerInterface(mockController)
			mf = baremetal_mocks.NewMockManagerFactoryInterface(mockController)
			fakeClientBuilder = fake.NewClientBuilder()
			objects = []client.Object{}

			if tc.m3mTemplateCantBeFound {
				fakeClient = fakeClientBuilder.WithScheme(setupScheme()).Build()
			} else {
				if tc.common.m3mTemplate != nil {
					objects = append(objects, tc.common.m3mTemplate)
				}
				fakeClient = fakeClientBuilder.WithScheme(setupScheme()).WithObjects(objects...).Build()
			}

			testReconciler = &Metal3MachineTemplateReconciler{
				Client:           fakeClient,
				ManagerFactory:   mf,
				Log:              logr.Discard(),
				WatchFilterValue: "",
			}

			if tc.failedToCreateMachineTemplateManager {
				mf.EXPECT().NewMachineTemplateManager(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(m, errors.New(""))
			} else if tc.m3mTemplateIsPaused {
				mf.EXPECT().NewMachineTemplateManager(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(m, nil)
			} else if tc.common.shouldUpdateAutomatedCleaningMode {
				mf.EXPECT().NewMachineTemplateManager(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(m, nil)
				m.EXPECT().UpdateAutomatedCleaningMode(context.TODO()).Return(
					nil)
			}

			result, err := testReconciler.Reconcile(context.TODO(), tc.common.testRequest)
			Expect(result).To(Equal(tc.common.expectedResult))
			evaluateM3MTemplateTestError(tc.common.expectedError, err)
			mockController.Finish()
		},
		Entry("M3MTemplate haven't been found",
			reconcileTemplateTestCase{
				common: commonTestCase{
					testRequest:    defaultTestRequest,
					expectedResult: ctrl.Result{},
					expectedError:  nil,
				},
				m3mTemplateCantBeFound: true,
			}),
		Entry("Failed to create helper for managing the template manager",
			reconcileTemplateTestCase{
				common: commonTestCase{
					testRequest:    defaultTestRequest,
					expectedResult: ctrl.Result{},
					expectedError:  &templateMgrErrorMsg,
					m3mTemplate: newMetal3MachineTemplate(
						metal3machineTemplateName,
						namespaceName,
						map[string]string{}),
				},
				failedToCreateMachineTemplateManager: true,
			}),
		Entry("Metal3MachineTemplate is currently paused",
			reconcileTemplateTestCase{
				common: commonTestCase{
					testRequest:    defaultTestRequest,
					expectedResult: ctrl.Result{Requeue: true, RequeueAfter: requeueAfter},
					expectedError:  nil,
					m3mTemplate: newMetal3MachineTemplate(
						metal3machineTemplateName,
						namespaceName,
						map[string]string{
							clusterv1.PausedAnnotation: "true",
						}),
				},
				m3mTemplateIsPaused: true,
			}),
		Entry("updateAutomatedCleaningMode should Succeed through normalReconcile call",
			reconcileTemplateTestCase{
				common: commonTestCase{
					testRequest:    defaultTestRequest,
					expectedResult: ctrl.Result{},
					expectedError:  nil,
					m3mTemplate: newMetal3MachineTemplate(
						metal3machineTemplateName,
						namespaceName,
						map[string]string{}),
					shouldUpdateAutomatedCleaningMode: true,
				},
			}),
	)

	DescribeTable("Metal3MachineTemplate reconcileNormal test",
		func(tc reconcileTemplateNormalTestCase) {
			mockController = gomock.NewController(GinkgoT())
			m = baremetal_mocks.NewMockTemplateManagerInterface(mockController)
			mf = baremetal_mocks.NewMockManagerFactoryInterface(mockController)
			fakeClientBuilder = fake.NewClientBuilder()
			objects = []client.Object{}
			objects = append(objects, tc.common.m3mTemplate)
			fakeClient = fakeClientBuilder.WithScheme(setupScheme()).WithObjects(objects...).Build()

			if tc.failedUpdateAutomatedCleaningMode {
				m.EXPECT().UpdateAutomatedCleaningMode(context.TODO()).Return(
					errors.New(""))
			} else if tc.common.shouldUpdateAutomatedCleaningMode {
				m.EXPECT().UpdateAutomatedCleaningMode(context.TODO()).Return(
					nil)
			}

			testReconciler = &Metal3MachineTemplateReconciler{
				Client:           fakeClient,
				ManagerFactory:   mf,
				Log:              logr.Discard(),
				WatchFilterValue: "",
			}

			result, err := testReconciler.reconcileNormal(context.TODO(), m)
			Expect(result).To(Equal(tc.common.expectedResult))
			evaluateM3MTemplateTestError(tc.common.expectedError, err)
			mockController.Finish()
		},
		Entry("updateAutomatedCleaningMode should Fail",
			reconcileTemplateNormalTestCase{
				common: commonTestCase{
					testRequest:    defaultTestRequest,
					expectedResult: ctrl.Result{},
					expectedError:  new(string),
					m3mTemplate: newMetal3MachineTemplate(metal3machineTemplateName,
						namespaceName,
						map[string]string{}),
				},
				failedUpdateAutomatedCleaningMode: true,
			}),
		Entry("updateAutomatedCleaningMode should Succeed",
			reconcileTemplateNormalTestCase{
				common: commonTestCase{
					testRequest:    defaultTestRequest,
					expectedResult: ctrl.Result{},
					expectedError:  nil,
					m3mTemplate: newMetal3MachineTemplate(
						metal3machineTemplateName,
						namespaceName,
						map[string]string{}),
					shouldUpdateAutomatedCleaningMode: true,
				},
			}),
	)
})

func evaluateM3MTemplateTestError(expected *string, actual error) {
	if expected == nil {
		Expect(actual).To(BeNil())
	} else {
		Expect(expected).ToNot(BeNil())
		Ω(actual.Error()).Should(ContainSubstring(*expected))
	}
}
