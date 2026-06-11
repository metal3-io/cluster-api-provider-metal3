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
	"fmt"
	"testing"
	"time"

	metrics "github.com/metal3-io/cluster-api-provider-metal3/internal/metrics"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func uniqueLabel(t *testing.T, suffix string) string {
	t.Helper()
	return fmt.Sprintf("%s-%s", t.Name(), suffix)
}

func expectCounterIncrement(t *testing.T, counter prometheus.Counter, record func()) {
	t.Helper()

	before := testutil.ToFloat64(counter)
	record()

	NewWithT(t).Expect(testutil.ToFloat64(counter)).To(Equal(before + 1))
}

func expectHistogramObservation(t *testing.T, collector prometheus.Collector, labels prometheus.Labels, record func()) {
	t.Helper()

	before := histogramSampleCount(t, collector, labels)
	record()

	NewWithT(t).Expect(histogramSampleCount(t, collector, labels)).To(Equal(before + 1))
}

func histogramSampleCount(t *testing.T, collector prometheus.Collector, labels prometheus.Labels) uint64 {
	t.Helper()

	metricCh := make(chan prometheus.Metric)
	go func() {
		collector.Collect(metricCh)
		close(metricCh)
	}()

	for metric := range metricCh {
		dtoMetric := &dto.Metric{}
		if err := metric.Write(dtoMetric); err != nil {
			t.Fatalf("failed to read metric: %v", err)
		}
		if !hasLabels(dtoMetric, labels) || dtoMetric.GetHistogram() == nil {
			continue
		}
		return dtoMetric.GetHistogram().GetSampleCount()
	}
	return 0
}

func hasLabels(metric *dto.Metric, labels prometheus.Labels) bool {
	values := map[string]string{}
	for _, label := range metric.GetLabel() {
		values[label.GetName()] = label.GetValue()
	}
	for name, value := range labels {
		if values[name] != value {
			return false
		}
	}
	return true
}

func TestRecordMetal3MachineReconcile(t *testing.T) {
	startTime := time.Now().Add(-time.Second)
	namespace := uniqueLabel(t, "ns")
	cluster := uniqueLabel(t, "cluster")
	successMetric := metrics.Metal3MachineReconcileTotal.WithLabelValues(namespace, cluster, metrics.ResultSuccess)
	errorMetric := metrics.Metal3MachineReconcileTotal.WithLabelValues(namespace, cluster, metrics.ResultError)

	expectCounterIncrement(t, successMetric, func() {
		metrics.RecordMetal3MachineReconcile(namespace, cluster, startTime, false)
	})
	expectCounterIncrement(t, errorMetric, func() {
		metrics.RecordMetal3MachineReconcile(namespace, cluster, startTime, true)
	})
}

func TestRecordMetal3ClusterReconcile(t *testing.T) {
	startTime := time.Now().Add(-time.Second)
	namespace := uniqueLabel(t, "ns")
	cluster := uniqueLabel(t, "cluster")
	successMetric := metrics.Metal3ClusterReconcileTotal.WithLabelValues(namespace, cluster, metrics.ResultSuccess)
	errorMetric := metrics.Metal3ClusterReconcileTotal.WithLabelValues(namespace, cluster, metrics.ResultError)

	expectCounterIncrement(t, successMetric, func() {
		metrics.RecordMetal3ClusterReconcile(namespace, cluster, startTime, false)
	})
	expectCounterIncrement(t, errorMetric, func() {
		metrics.RecordMetal3ClusterReconcile(namespace, cluster, startTime, true)
	})
}

func TestRecordBMHAssociation(t *testing.T) {
	startTime := time.Now().Add(-time.Second)
	namespace := uniqueLabel(t, "ns")
	cluster := uniqueLabel(t, "cluster")
	successMetric := metrics.BMHAssociationTotal.WithLabelValues(namespace, cluster, metrics.ResultSuccess)
	errorMetric := metrics.BMHAssociationTotal.WithLabelValues(namespace, cluster, metrics.ResultError)

	expectCounterIncrement(t, successMetric, func() {
		metrics.RecordBMHAssociation(namespace, cluster, startTime, nil)
	})
	expectCounterIncrement(t, errorMetric, func() {
		metrics.RecordBMHAssociation(namespace, cluster, startTime, errors.New("test error"))
	})
}

func TestRecordReconcileError(t *testing.T) {
	namespace := uniqueLabel(t, "ns")
	controller := uniqueLabel(t, "controller")
	transientMetric := metrics.ReconcileErrorsTotal.WithLabelValues(controller, namespace, metrics.ErrorTypeTransient)
	terminalMetric := metrics.ReconcileErrorsTotal.WithLabelValues(controller, namespace, metrics.ErrorTypeTerminal)

	expectCounterIncrement(t, transientMetric, func() {
		metrics.RecordReconcileError(controller, namespace, true)
	})
	expectCounterIncrement(t, terminalMetric, func() {
		metrics.RecordReconcileError(controller, namespace, false)
	})
}

func TestRecordMetal3MachineProvisioning(t *testing.T) {
	creationTime := time.Now().Add(-5 * time.Minute)
	namespace := uniqueLabel(t, "ns")
	cluster := uniqueLabel(t, "cluster")

	expectHistogramObservation(t, metrics.Metal3MachineProvisioningDuration, prometheus.Labels{
		metrics.LabelNamespace: namespace,
		metrics.LabelCluster:   cluster,
	}, func() {
		metrics.RecordMetal3MachineProvisioning(namespace, cluster, creationTime)
	})
}

func TestRecordMetal3DataReconcile(t *testing.T) {
	startTime := time.Now().Add(-time.Second)
	namespace := uniqueLabel(t, "ns")
	successMetric := metrics.Metal3DataReconcileTotal.WithLabelValues(namespace, metrics.ResultSuccess)
	errorMetric := metrics.Metal3DataReconcileTotal.WithLabelValues(namespace, metrics.ResultError)

	expectCounterIncrement(t, successMetric, func() {
		metrics.RecordMetal3DataReconcile(namespace, startTime, false)
	})
	expectCounterIncrement(t, errorMetric, func() {
		metrics.RecordMetal3DataReconcile(namespace, startTime, true)
	})
}

func TestRecordMetal3DataTemplateReconcile(t *testing.T) {
	startTime := time.Now().Add(-time.Second)
	namespace := uniqueLabel(t, "ns")
	successMetric := metrics.Metal3DataTemplateReconcileTotal.WithLabelValues(namespace, metrics.ResultSuccess)
	errorMetric := metrics.Metal3DataTemplateReconcileTotal.WithLabelValues(namespace, metrics.ResultError)

	expectCounterIncrement(t, successMetric, func() {
		metrics.RecordMetal3DataTemplateReconcile(namespace, startTime, false)
	})
	expectCounterIncrement(t, errorMetric, func() {
		metrics.RecordMetal3DataTemplateReconcile(namespace, startTime, true)
	})
}

func TestRecordMetal3RemediationReconcile(t *testing.T) {
	startTime := time.Now().Add(-time.Second)
	namespace := uniqueLabel(t, "ns")
	successMetric := metrics.Metal3RemediationReconcileTotal.WithLabelValues(namespace, metrics.ResultSuccess)
	errorMetric := metrics.Metal3RemediationReconcileTotal.WithLabelValues(namespace, metrics.ResultError)

	expectCounterIncrement(t, successMetric, func() {
		metrics.RecordMetal3RemediationReconcile(namespace, startTime, false)
	})
	expectCounterIncrement(t, errorMetric, func() {
		metrics.RecordMetal3RemediationReconcile(namespace, startTime, true)
	})
}

func TestRecordMetal3Remediation(t *testing.T) {
	startTime := time.Now().Add(-time.Second)
	namespace := uniqueLabel(t, "ns")
	remediationType := uniqueLabel(t, "reboot")
	successMetric := metrics.Metal3RemediationTotal.WithLabelValues(namespace, remediationType, metrics.ResultSuccess)
	errorMetric := metrics.Metal3RemediationTotal.WithLabelValues(namespace, remediationType, metrics.ResultError)

	expectCounterIncrement(t, successMetric, func() {
		metrics.RecordMetal3Remediation(namespace, remediationType, startTime, nil)
	})
	expectCounterIncrement(t, errorMetric, func() {
		metrics.RecordMetal3Remediation(namespace, remediationType, startTime, errors.New("test error"))
	})
}

func TestRecordMetal3MachinePhaseTransition(t *testing.T) {
	namespace := uniqueLabel(t, "ns")
	cluster := uniqueLabel(t, "cluster")
	phase := uniqueLabel(t, "provisioning")
	metric := metrics.Metal3MachinePhaseTransitions.WithLabelValues(namespace, cluster, phase)

	expectCounterIncrement(t, metric, func() {
		metrics.RecordMetal3MachinePhaseTransition(namespace, cluster, phase)
	})
}

func TestSetMetal3MachinesCount(t *testing.T) {
	g := NewWithT(t)
	namespace := uniqueLabel(t, "ns")
	phase := uniqueLabel(t, "running")

	metrics.SetMetal3MachinesCount(namespace, phase, 5)

	g.Expect(testutil.ToFloat64(metrics.Metal3MachinesCount.WithLabelValues(namespace, phase))).To(Equal(float64(5)))
}

func TestSetMetal3ClustersCount(t *testing.T) {
	g := NewWithT(t)
	namespace := uniqueLabel(t, "ns")

	metrics.SetMetal3ClustersCount(namespace, 3)

	g.Expect(testutil.ToFloat64(metrics.Metal3ClustersCount.WithLabelValues(namespace))).To(Equal(float64(3)))
}

func TestRecordMetal3MachineTemplateReconcile(t *testing.T) {
	startTime := time.Now().Add(-time.Second)
	namespace := uniqueLabel(t, "ns")
	successMetric := metrics.Metal3MachineTemplateReconcileTotal.WithLabelValues(namespace, metrics.ResultSuccess)
	errorMetric := metrics.Metal3MachineTemplateReconcileTotal.WithLabelValues(namespace, metrics.ResultError)

	expectCounterIncrement(t, successMetric, func() {
		metrics.RecordMetal3MachineTemplateReconcile(namespace, startTime, false)
	})
	expectCounterIncrement(t, errorMetric, func() {
		metrics.RecordMetal3MachineTemplateReconcile(namespace, startTime, true)
	})
}

func TestRecordMetal3LabelSyncReconcile(t *testing.T) {
	startTime := time.Now().Add(-time.Second)
	namespace := uniqueLabel(t, "ns")
	successMetric := metrics.Metal3LabelSyncReconcileTotal.WithLabelValues(namespace, metrics.ResultSuccess)
	errorMetric := metrics.Metal3LabelSyncReconcileTotal.WithLabelValues(namespace, metrics.ResultError)

	expectCounterIncrement(t, successMetric, func() {
		metrics.RecordMetal3LabelSyncReconcile(namespace, startTime, false)
	})
	expectCounterIncrement(t, errorMetric, func() {
		metrics.RecordMetal3LabelSyncReconcile(namespace, startTime, true)
	})
}
