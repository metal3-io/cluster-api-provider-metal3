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
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
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
	templateMgrErrorMsg := "failed to create helper for managing the templateMgr"
	defaultTestRequest := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      metal3machineTemplateName,
			Namespace: namespaceName,
		},
	}

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
							capi.PausedAnnotation: "true",
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
