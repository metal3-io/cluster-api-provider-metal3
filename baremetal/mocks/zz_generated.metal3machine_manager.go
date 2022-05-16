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
// Source: ./baremetal/metal3machine_manager.go

// Package baremetal_mocks is a generated GoMock package.
package baremetal_mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	baremetal "github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	errors "sigs.k8s.io/cluster-api/errors"
)

// MockMachineManagerInterface is a mock of MachineManagerInterface interface.
type MockMachineManagerInterface struct {
	ctrl     *gomock.Controller
	recorder *MockMachineManagerInterfaceMockRecorder
}

// MockMachineManagerInterfaceMockRecorder is the mock recorder for MockMachineManagerInterface.
type MockMachineManagerInterfaceMockRecorder struct {
	mock *MockMachineManagerInterface
}

// NewMockMachineManagerInterface creates a new mock instance.
func NewMockMachineManagerInterface(ctrl *gomock.Controller) *MockMachineManagerInterface {
	mock := &MockMachineManagerInterface{ctrl: ctrl}
	mock.recorder = &MockMachineManagerInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMachineManagerInterface) EXPECT() *MockMachineManagerInterfaceMockRecorder {
	return m.recorder
}

// Associate mocks base method.
func (m *MockMachineManagerInterface) Associate(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Associate", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Associate indicates an expected call of Associate.
func (mr *MockMachineManagerInterfaceMockRecorder) Associate(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Associate", reflect.TypeOf((*MockMachineManagerInterface)(nil).Associate), arg0)
}

// AssociateM3Metadata mocks base method.
func (m *MockMachineManagerInterface) AssociateM3Metadata(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AssociateM3Metadata", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// AssociateM3Metadata indicates an expected call of AssociateM3Metadata.
func (mr *MockMachineManagerInterfaceMockRecorder) AssociateM3Metadata(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AssociateM3Metadata", reflect.TypeOf((*MockMachineManagerInterface)(nil).AssociateM3Metadata), arg0)
}

// Delete mocks base method.
func (m *MockMachineManagerInterface) Delete(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockMachineManagerInterfaceMockRecorder) Delete(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockMachineManagerInterface)(nil).Delete), arg0)
}

// DissociateM3Metadata mocks base method.
func (m *MockMachineManagerInterface) DissociateM3Metadata(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DissociateM3Metadata", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// DissociateM3Metadata indicates an expected call of DissociateM3Metadata.
func (mr *MockMachineManagerInterfaceMockRecorder) DissociateM3Metadata(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DissociateM3Metadata", reflect.TypeOf((*MockMachineManagerInterface)(nil).DissociateM3Metadata), arg0)
}

// GetBaremetalHostID mocks base method.
func (m *MockMachineManagerInterface) GetBaremetalHostID(arg0 context.Context) (*string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBaremetalHostID", arg0)
	ret0, _ := ret[0].(*string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBaremetalHostID indicates an expected call of GetBaremetalHostID.
func (mr *MockMachineManagerInterfaceMockRecorder) GetBaremetalHostID(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBaremetalHostID", reflect.TypeOf((*MockMachineManagerInterface)(nil).GetBaremetalHostID), arg0)
}

// GetProviderIDAndBMHID mocks base method.
func (m *MockMachineManagerInterface) GetProviderIDAndBMHID() (string, *string) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProviderIDAndBMHID")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(*string)
	return ret0, ret1
}

// GetProviderIDAndBMHID indicates an expected call of GetProviderIDAndBMHID.
func (mr *MockMachineManagerInterfaceMockRecorder) GetProviderIDAndBMHID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProviderIDAndBMHID", reflect.TypeOf((*MockMachineManagerInterface)(nil).GetProviderIDAndBMHID))
}

// HasAnnotation mocks base method.
func (m *MockMachineManagerInterface) HasAnnotation() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasAnnotation")
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasAnnotation indicates an expected call of HasAnnotation.
func (mr *MockMachineManagerInterfaceMockRecorder) HasAnnotation() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasAnnotation", reflect.TypeOf((*MockMachineManagerInterface)(nil).HasAnnotation))
}

// IsBootstrapReady mocks base method.
func (m *MockMachineManagerInterface) IsBootstrapReady() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsBootstrapReady")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsBootstrapReady indicates an expected call of IsBootstrapReady.
func (mr *MockMachineManagerInterfaceMockRecorder) IsBootstrapReady() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsBootstrapReady", reflect.TypeOf((*MockMachineManagerInterface)(nil).IsBootstrapReady))
}

// IsProvisioned mocks base method.
func (m *MockMachineManagerInterface) IsProvisioned() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsProvisioned")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsProvisioned indicates an expected call of IsProvisioned.
func (mr *MockMachineManagerInterfaceMockRecorder) IsProvisioned() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsProvisioned", reflect.TypeOf((*MockMachineManagerInterface)(nil).IsProvisioned))
}

// RemovePauseAnnotation mocks base method.
func (m *MockMachineManagerInterface) RemovePauseAnnotation(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemovePauseAnnotation", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemovePauseAnnotation indicates an expected call of RemovePauseAnnotation.
func (mr *MockMachineManagerInterfaceMockRecorder) RemovePauseAnnotation(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemovePauseAnnotation", reflect.TypeOf((*MockMachineManagerInterface)(nil).RemovePauseAnnotation), arg0)
}

// SetError mocks base method.
func (m *MockMachineManagerInterface) SetError(arg0 string, arg1 errors.MachineStatusError) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetError", arg0, arg1)
}

// SetError indicates an expected call of SetError.
func (mr *MockMachineManagerInterfaceMockRecorder) SetError(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetError", reflect.TypeOf((*MockMachineManagerInterface)(nil).SetError), arg0, arg1)
}

// SetFinalizer mocks base method.
func (m *MockMachineManagerInterface) SetFinalizer() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetFinalizer")
}

// SetFinalizer indicates an expected call of SetFinalizer.
func (mr *MockMachineManagerInterfaceMockRecorder) SetFinalizer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetFinalizer", reflect.TypeOf((*MockMachineManagerInterface)(nil).SetFinalizer))
}

// SetNodeProviderID mocks base method.
func (m *MockMachineManagerInterface) SetNodeProviderID(arg0 context.Context, arg1, arg2 *string, arg3 baremetal.ClientGetter) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetNodeProviderID", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetNodeProviderID indicates an expected call of SetNodeProviderID.
func (mr *MockMachineManagerInterfaceMockRecorder) SetNodeProviderID(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetNodeProviderID", reflect.TypeOf((*MockMachineManagerInterface)(nil).SetNodeProviderID), arg0, arg1, arg2, arg3)
}

// SetPauseAnnotation mocks base method.
func (m *MockMachineManagerInterface) SetPauseAnnotation(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetPauseAnnotation", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetPauseAnnotation indicates an expected call of SetPauseAnnotation.
func (mr *MockMachineManagerInterfaceMockRecorder) SetPauseAnnotation(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetPauseAnnotation", reflect.TypeOf((*MockMachineManagerInterface)(nil).SetPauseAnnotation), arg0)
}

// SetProviderID mocks base method.
func (m *MockMachineManagerInterface) SetProviderID(arg0 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetProviderID", arg0)
}

// SetProviderID indicates an expected call of SetProviderID.
func (mr *MockMachineManagerInterfaceMockRecorder) SetProviderID(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetProviderID", reflect.TypeOf((*MockMachineManagerInterface)(nil).SetProviderID), arg0)
}

// UnsetFinalizer mocks base method.
func (m *MockMachineManagerInterface) UnsetFinalizer() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UnsetFinalizer")
}

// UnsetFinalizer indicates an expected call of UnsetFinalizer.
func (mr *MockMachineManagerInterfaceMockRecorder) UnsetFinalizer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnsetFinalizer", reflect.TypeOf((*MockMachineManagerInterface)(nil).UnsetFinalizer))
}

// Update mocks base method.
func (m *MockMachineManagerInterface) Update(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *MockMachineManagerInterfaceMockRecorder) Update(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockMachineManagerInterface)(nil).Update), arg0)
}
