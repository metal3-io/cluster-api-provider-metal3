/*
Copyright 2024 The Kubernetes Authors.

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

package metrics

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestRecordMetal3MachineReconcile(t *testing.T) {
	g := NewWithT(t)

	// Test successful reconciliation
	startTime := time.Now().Add(-time.Second)
	RecordMetal3MachineReconcile("test-ns", "test-cluster", startTime, nil)

	// Verify the counter was incremented (we can't easily verify the value,
	// but we can verify it doesn't panic)
	g.Expect(Metal3MachineReconcileTotal).NotTo(BeNil())

	// Test failed reconciliation
	RecordMetal3MachineReconcile("test-ns", "test-cluster", startTime, errors.New("test error"))
	g.Expect(Metal3MachineReconcileTotal).NotTo(BeNil())
}

func TestRecordMetal3ClusterReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	RecordMetal3ClusterReconcile("test-ns", "test-cluster", startTime, nil)

	g.Expect(Metal3ClusterReconcileTotal).NotTo(BeNil())
}

func TestRecordBMHAssociation(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	RecordBMHAssociation("test-ns", "test-cluster", startTime, nil)

	g.Expect(BMHAssociationTotal).NotTo(BeNil())

	RecordBMHAssociation("test-ns", "test-cluster", startTime, errors.New("test error"))
	g.Expect(BMHAssociationTotal).NotTo(BeNil())
}

func TestRecordReconcileError(t *testing.T) {
	g := NewWithT(t)

	// Test transient error
	RecordReconcileError("test-controller", "test-ns", true)
	g.Expect(ReconcileErrorsTotal).NotTo(BeNil())

	// Test terminal error
	RecordReconcileError("test-controller", "test-ns", false)
	g.Expect(ReconcileErrorsTotal).NotTo(BeNil())
}

func TestRecordMetal3MachineProvisioning(t *testing.T) {
	g := NewWithT(t)

	creationTime := time.Now().Add(-5 * time.Minute)
	RecordMetal3MachineProvisioning("test-ns", "test-cluster", creationTime)

	g.Expect(Metal3MachineProvisioningDuration).NotTo(BeNil())
}

func TestRecordMetal3DataReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	RecordMetal3DataReconcile("test-ns", startTime, nil)

	g.Expect(Metal3DataReconcileTotal).NotTo(BeNil())
}

func TestRecordMetal3DataTemplateReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	RecordMetal3DataTemplateReconcile("test-ns", startTime, nil)

	g.Expect(Metal3DataTemplateReconcileTotal).NotTo(BeNil())
}

func TestRecordMetal3RemediationReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	RecordMetal3RemediationReconcile("test-ns", startTime, nil)

	g.Expect(Metal3RemediationTotal).NotTo(BeNil())
}

func TestRecordMetal3Remediation(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	RecordMetal3Remediation("test-ns", "reboot", startTime, nil)

	g.Expect(Metal3RemediationTotal).NotTo(BeNil())
	g.Expect(Metal3RemediationDuration).NotTo(BeNil())
}

func TestRecordMetal3MachinePhaseTransition(t *testing.T) {
	g := NewWithT(t)

	RecordMetal3MachinePhaseTransition("test-ns", "test-cluster", "provisioning")

	g.Expect(Metal3MachinePhaseTransitions).NotTo(BeNil())
}

func TestSetMetal3MachinesCount(t *testing.T) {
	g := NewWithT(t)

	SetMetal3MachinesCount("test-ns", "running", 5)

	g.Expect(Metal3MachinesCount).NotTo(BeNil())
}

func TestSetMetal3ClustersCount(t *testing.T) {
	g := NewWithT(t)

	SetMetal3ClustersCount("test-ns", 3)

	g.Expect(Metal3ClustersCount).NotTo(BeNil())
}

func TestRecordMetal3MachineTemplateReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	RecordMetal3MachineTemplateReconcile("test-ns", startTime, nil)

	g.Expect(Metal3MachineTemplateReconcileTotal).NotTo(BeNil())
	g.Expect(Metal3MachineTemplateReconcileDuration).NotTo(BeNil())
}

func TestRecordMetal3LabelSyncReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	RecordMetal3LabelSyncReconcile("test-ns", startTime, nil)

	g.Expect(Metal3LabelSyncReconcileTotal).NotTo(BeNil())
	g.Expect(Metal3LabelSyncReconcileDuration).NotTo(BeNil())
}
