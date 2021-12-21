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
	var fakeM3MTemplateManager *baremetal_mocks.MockTemplateManagerInterface
	var fakeManagerFactory *baremetal_mocks.MockManagerFactoryInterface
	var objects []client.Object
	templateMgrErrorMsg := "failed to create helper for managing the templateMgr"
	defaultTestRequest := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      metal3machineTemplateName,
			Namespace: namespaceName,
		},
	}

	DescribeTable("Metal3MachineTemplate Reconcile test",
		func(testCase reconcileTemplateTestCase) {
			mockController = gomock.NewController(GinkgoT())
			fakeM3MTemplateManager = baremetal_mocks.NewMockTemplateManagerInterface(mockController)
			fakeManagerFactory = baremetal_mocks.NewMockManagerFactoryInterface(mockController)
			fakeClientBuilder = fake.NewClientBuilder()
			objects = []client.Object{}

			if testCase.m3mTemplateCantBeFound {
				fakeClient = fakeClientBuilder.WithScheme(setupScheme()).Build()
			} else {
				if testCase.common.m3mTemplate != nil {
					objects = append(objects, testCase.common.m3mTemplate)
				}
				fakeClient = fakeClientBuilder.WithScheme(setupScheme()).WithObjects(objects...).Build()
			}

			testReconciler = &Metal3MachineTemplateReconciler{
				Client:           fakeClient,
				ManagerFactory:   fakeManagerFactory,
				Log:              logr.Discard(),
				WatchFilterValue: "",
			}

			if testCase.failedToCreateMachineTemplateManager {
				fakeManagerFactory.EXPECT().NewMachineTemplateManager(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(fakeM3MTemplateManager, errors.New(""))
			} else if testCase.m3mTemplateIsPaused {
				fakeManagerFactory.EXPECT().NewMachineTemplateManager(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(fakeM3MTemplateManager, nil)
			} else if testCase.common.shouldUpdateAutomatedCleaningMode {
				fakeManagerFactory.EXPECT().NewMachineTemplateManager(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(fakeM3MTemplateManager, nil)
				fakeM3MTemplateManager.EXPECT().UpdateAutomatedCleaningMode(context.TODO()).Return(
					nil)
			}

			result, err := testReconciler.Reconcile(context.TODO(), testCase.common.testRequest)
			Expect(result).To(Equal(testCase.common.expectedResult))
			evaluateM3MTemplateTestError(testCase.common.expectedError, err)
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
		func(testCase reconcileTemplateNormalTestCase) {
			mockController = gomock.NewController(GinkgoT())
			fakeM3MTemplateManager = baremetal_mocks.NewMockTemplateManagerInterface(mockController)
			fakeManagerFactory = baremetal_mocks.NewMockManagerFactoryInterface(mockController)
			fakeClientBuilder = fake.NewClientBuilder()
			objects = []client.Object{}
			objects = append(objects, testCase.common.m3mTemplate)
			fakeClient = fakeClientBuilder.WithScheme(setupScheme()).WithObjects(objects...).Build()

			if testCase.failedUpdateAutomatedCleaningMode {
				fakeM3MTemplateManager.EXPECT().UpdateAutomatedCleaningMode(context.TODO()).Return(
					errors.New(""))
			} else if testCase.common.shouldUpdateAutomatedCleaningMode {
				fakeM3MTemplateManager.EXPECT().UpdateAutomatedCleaningMode(context.TODO()).Return(
					nil)
			}

			testReconciler = &Metal3MachineTemplateReconciler{
				Client:           fakeClient,
				ManagerFactory:   fakeManagerFactory,
				Log:              logr.Discard(),
				WatchFilterValue: "",
			}

			result, err := testReconciler.reconcileNormal(context.TODO(), fakeM3MTemplateManager)
			Expect(result).To(Equal(testCase.common.expectedResult))
			evaluateM3MTemplateTestError(testCase.common.expectedError, err)
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
