// /*
// Copyright The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */
//
//

// Code generated by MockGen. DO NOT EDIT.
// Source: ./baremetal/metal3datatemplate_manager.go

// Package baremetal_mocks is a generated GoMock package.
package baremetal_mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	v1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// MockDataTemplateManagerInterface is a mock of DataTemplateManagerInterface interface.
type MockDataTemplateManagerInterface struct {
	ctrl     *gomock.Controller
	recorder *MockDataTemplateManagerInterfaceMockRecorder
}

// MockDataTemplateManagerInterfaceMockRecorder is the mock recorder for MockDataTemplateManagerInterface.
type MockDataTemplateManagerInterfaceMockRecorder struct {
	mock *MockDataTemplateManagerInterface
}

// NewMockDataTemplateManagerInterface creates a new mock instance.
func NewMockDataTemplateManagerInterface(ctrl *gomock.Controller) *MockDataTemplateManagerInterface {
	mock := &MockDataTemplateManagerInterface{ctrl: ctrl}
	mock.recorder = &MockDataTemplateManagerInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDataTemplateManagerInterface) EXPECT() *MockDataTemplateManagerInterfaceMockRecorder {
	return m.recorder
}

// SetClusterOwnerRef mocks base method.
func (m *MockDataTemplateManagerInterface) SetClusterOwnerRef(arg0 *v1beta2.Cluster) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetClusterOwnerRef", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetClusterOwnerRef indicates an expected call of SetClusterOwnerRef.
func (mr *MockDataTemplateManagerInterfaceMockRecorder) SetClusterOwnerRef(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetClusterOwnerRef", reflect.TypeOf((*MockDataTemplateManagerInterface)(nil).SetClusterOwnerRef), arg0)
}

// SetFinalizer mocks base method.
func (m *MockDataTemplateManagerInterface) SetFinalizer() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetFinalizer")
}

// SetFinalizer indicates an expected call of SetFinalizer.
func (mr *MockDataTemplateManagerInterfaceMockRecorder) SetFinalizer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetFinalizer", reflect.TypeOf((*MockDataTemplateManagerInterface)(nil).SetFinalizer))
}

// UnsetFinalizer mocks base method.
func (m *MockDataTemplateManagerInterface) UnsetFinalizer() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UnsetFinalizer")
}

// UnsetFinalizer indicates an expected call of UnsetFinalizer.
func (mr *MockDataTemplateManagerInterfaceMockRecorder) UnsetFinalizer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnsetFinalizer", reflect.TypeOf((*MockDataTemplateManagerInterface)(nil).UnsetFinalizer))
}

// UpdateDatas mocks base method.
func (m *MockDataTemplateManagerInterface) UpdateDatas(arg0 context.Context) (bool, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateDatas", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// UpdateDatas indicates an expected call of UpdateDatas.
func (mr *MockDataTemplateManagerInterfaceMockRecorder) UpdateDatas(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDatas", reflect.TypeOf((*MockDataTemplateManagerInterface)(nil).UpdateDatas), arg0)
}
