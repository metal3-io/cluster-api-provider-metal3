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

package metrics_test

import (
	"errors"
	"testing"
	"time"

	metrics "github.com/metal3-io/cluster-api-provider-metal3/internal/metrics"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordMetal3MachineReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	metric := metrics.Metal3MachineReconcileTotal.WithLabelValues("test-ns", "test-cluster", metrics.ResultSuccess)
	before := testutil.ToFloat64(metric)

	metrics.RecordMetal3MachineReconcile("test-ns", "test-cluster", startTime, false)

	g.Expect(testutil.ToFloat64(metric)).To(Equal(before + 1))
}

func TestRecordMetal3ClusterReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	metric := metrics.Metal3ClusterReconcileTotal.WithLabelValues("test-ns", "test-cluster", metrics.ResultSuccess)
	before := testutil.ToFloat64(metric)

	metrics.RecordMetal3ClusterReconcile("test-ns", "test-cluster", startTime, false)

	g.Expect(testutil.ToFloat64(metric)).To(Equal(before + 1))
}

func TestRecordBMHAssociation(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	successMetric := metrics.BMHAssociationTotal.WithLabelValues("test-ns", "test-cluster", metrics.ResultSuccess)
	errorMetric := metrics.BMHAssociationTotal.WithLabelValues("test-ns", "test-cluster", metrics.ResultError)
	successBefore := testutil.ToFloat64(successMetric)
	errorBefore := testutil.ToFloat64(errorMetric)

	metrics.RecordBMHAssociation("test-ns", "test-cluster", startTime, nil)

	metrics.RecordBMHAssociation("test-ns", "test-cluster", startTime, errors.New("test error"))

	g.Expect(testutil.ToFloat64(successMetric)).To(Equal(successBefore + 1))
	g.Expect(testutil.ToFloat64(errorMetric)).To(Equal(errorBefore + 1))
}

func TestRecordReconcileError(t *testing.T) {
	g := NewWithT(t)

	transientMetric := metrics.ReconcileErrorsTotal.WithLabelValues("test-controller", "test-ns", metrics.ErrorTypeTransient)
	terminalMetric := metrics.ReconcileErrorsTotal.WithLabelValues("test-controller", "test-ns", metrics.ErrorTypeTerminal)
	transientBefore := testutil.ToFloat64(transientMetric)
	terminalBefore := testutil.ToFloat64(terminalMetric)

	metrics.RecordReconcileError("test-controller", "test-ns", true)
	metrics.RecordReconcileError("test-controller", "test-ns", false)

	g.Expect(testutil.ToFloat64(transientMetric)).To(Equal(transientBefore + 1))
	g.Expect(testutil.ToFloat64(terminalMetric)).To(Equal(terminalBefore + 1))
}

func TestRecordMetal3MachineProvisioning(t *testing.T) {
	g := NewWithT(t)

	creationTime := time.Now().Add(-5 * time.Minute)
	metrics.RecordMetal3MachineProvisioning("test-ns", "test-cluster", creationTime)

	g.Expect(metrics.Metal3MachineProvisioningDuration).NotTo(BeNil())
}

func TestRecordMetal3DataReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	metric := metrics.Metal3DataReconcileTotal.WithLabelValues("test-ns", metrics.ResultSuccess)
	before := testutil.ToFloat64(metric)

	metrics.RecordMetal3DataReconcile("test-ns", startTime, false)

	g.Expect(testutil.ToFloat64(metric)).To(Equal(before + 1))
}

func TestRecordMetal3DataTemplateReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	metrics.RecordMetal3DataTemplateReconcile("test-ns", startTime, false)

	g.Expect(metrics.Metal3DataTemplateReconcileTotal).NotTo(BeNil())
}

func TestRecordMetal3RemediationReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	metrics.RecordMetal3RemediationReconcile("test-ns", startTime, false)

	g.Expect(metrics.Metal3RemediationReconcileTotal).NotTo(BeNil())
}

func TestRecordMetal3Remediation(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	metrics.RecordMetal3Remediation("test-ns", "reboot", startTime, nil)

	g.Expect(metrics.Metal3RemediationTotal).NotTo(BeNil())
	g.Expect(metrics.Metal3RemediationDuration).NotTo(BeNil())
}

func TestRecordMetal3MachinePhaseTransition(t *testing.T) {
	g := NewWithT(t)

	metrics.RecordMetal3MachinePhaseTransition("test-ns", "test-cluster", "provisioning")

	g.Expect(metrics.Metal3MachinePhaseTransitions).NotTo(BeNil())
}

func TestSetMetal3MachinesCount(t *testing.T) {
	g := NewWithT(t)

	metrics.SetMetal3MachinesCount("test-ns", "running", 5)

	g.Expect(metrics.Metal3MachinesCount).NotTo(BeNil())
}

func TestSetMetal3ClustersCount(t *testing.T) {
	g := NewWithT(t)

	metrics.SetMetal3ClustersCount("test-ns", 3)

	g.Expect(metrics.Metal3ClustersCount).NotTo(BeNil())
}

func TestRecordMetal3MachineTemplateReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	metrics.RecordMetal3MachineTemplateReconcile("test-ns", startTime, false)

	g.Expect(metrics.Metal3MachineTemplateReconcileTotal).NotTo(BeNil())
	g.Expect(metrics.Metal3MachineTemplateReconcileDuration).NotTo(BeNil())
}

func TestRecordMetal3LabelSyncReconcile(t *testing.T) {
	g := NewWithT(t)

	startTime := time.Now().Add(-time.Second)
	metrics.RecordMetal3LabelSyncReconcile("test-ns", startTime, false)

	g.Expect(metrics.Metal3LabelSyncReconcileTotal).NotTo(BeNil())
	g.Expect(metrics.Metal3LabelSyncReconcileDuration).NotTo(BeNil())
}
