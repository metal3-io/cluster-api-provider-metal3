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
// Source: ./baremetal/manager_factory.go

// Package baremetal_mocks is a generated GoMock package.
package baremetal_mocks

import (
	reflect "reflect"

	logr "github.com/go-logr/logr"
	gomock "github.com/golang/mock/gomock"
	v1alpha4 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	baremetal "github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	v1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// MockManagerFactoryInterface is a mock of ManagerFactoryInterface interface.
type MockManagerFactoryInterface struct {
	ctrl     *gomock.Controller
	recorder *MockManagerFactoryInterfaceMockRecorder
}

// MockManagerFactoryInterfaceMockRecorder is the mock recorder for MockManagerFactoryInterface.
type MockManagerFactoryInterfaceMockRecorder struct {
	mock *MockManagerFactoryInterface
}

// NewMockManagerFactoryInterface creates a new mock instance.
func NewMockManagerFactoryInterface(ctrl *gomock.Controller) *MockManagerFactoryInterface {
	mock := &MockManagerFactoryInterface{ctrl: ctrl}
	mock.recorder = &MockManagerFactoryInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockManagerFactoryInterface) EXPECT() *MockManagerFactoryInterfaceMockRecorder {
	return m.recorder
}

// NewClusterManager mocks base method.
func (m *MockManagerFactoryInterface) NewClusterManager(cluster *v1alpha3.Cluster, metal3Cluster *v1alpha4.Metal3Cluster, clusterLog logr.Logger) (baremetal.ClusterManagerInterface, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewClusterManager", cluster, metal3Cluster, clusterLog)
	ret0, _ := ret[0].(baremetal.ClusterManagerInterface)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewClusterManager indicates an expected call of NewClusterManager.
func (mr *MockManagerFactoryInterfaceMockRecorder) NewClusterManager(cluster, metal3Cluster, clusterLog interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewClusterManager", reflect.TypeOf((*MockManagerFactoryInterface)(nil).NewClusterManager), cluster, metal3Cluster, clusterLog)
}

// NewDataManager mocks base method.
func (m *MockManagerFactoryInterface) NewDataManager(arg0 *v1alpha4.Metal3Data, arg1 logr.Logger) (baremetal.DataManagerInterface, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewDataManager", arg0, arg1)
	ret0, _ := ret[0].(baremetal.DataManagerInterface)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewDataManager indicates an expected call of NewDataManager.
func (mr *MockManagerFactoryInterfaceMockRecorder) NewDataManager(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewDataManager", reflect.TypeOf((*MockManagerFactoryInterface)(nil).NewDataManager), arg0, arg1)
}

// NewDataTemplateManager mocks base method.
func (m *MockManagerFactoryInterface) NewDataTemplateManager(arg0 *v1alpha4.Metal3DataTemplate, arg1 logr.Logger) (baremetal.DataTemplateManagerInterface, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewDataTemplateManager", arg0, arg1)
	ret0, _ := ret[0].(baremetal.DataTemplateManagerInterface)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewDataTemplateManager indicates an expected call of NewDataTemplateManager.
func (mr *MockManagerFactoryInterfaceMockRecorder) NewDataTemplateManager(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewDataTemplateManager", reflect.TypeOf((*MockManagerFactoryInterface)(nil).NewDataTemplateManager), arg0, arg1)
}

// NewMachineManager mocks base method.
func (m *MockManagerFactoryInterface) NewMachineManager(arg0 *v1alpha3.Cluster, arg1 *v1alpha4.Metal3Cluster, arg2 *v1alpha3.Machine, arg3 *v1alpha4.Metal3Machine, arg4 logr.Logger) (baremetal.MachineManagerInterface, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewMachineManager", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(baremetal.MachineManagerInterface)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewMachineManager indicates an expected call of NewMachineManager.
func (mr *MockManagerFactoryInterfaceMockRecorder) NewMachineManager(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewMachineManager", reflect.TypeOf((*MockManagerFactoryInterface)(nil).NewMachineManager), arg0, arg1, arg2, arg3, arg4)
}

// NewMachineTemplateManager mocks base method.
func (m *MockManagerFactoryInterface) NewMachineTemplateManager(capm3Template *v1alpha4.Metal3MachineTemplate, capm3MachineList *v1alpha4.Metal3MachineList, metadataLog logr.Logger) (baremetal.TemplateManagerInterface, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewMachineTemplateManager", capm3Template, capm3MachineList, metadataLog)
	ret0, _ := ret[0].(baremetal.TemplateManagerInterface)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewMachineTemplateManager indicates an expected call of NewMachineTemplateManager.
func (mr *MockManagerFactoryInterfaceMockRecorder) NewMachineTemplateManager(capm3Template, capm3MachineList, metadataLog interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewMachineTemplateManager", reflect.TypeOf((*MockManagerFactoryInterface)(nil).NewMachineTemplateManager), capm3Template, capm3MachineList, metadataLog)
}
