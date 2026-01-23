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

// Package metrics provides custom Prometheus metrics for CAPM3.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	// Subsystem is the metrics subsystem name for CAPM3.
	Subsystem = "capm3"

	// Labels used across metrics.
	LabelNamespace   = "namespace"
	LabelCluster     = "cluster"
	LabelMachine     = "machine"
	LabelController  = "controller"
	LabelResult      = "result"
	LabelPhase       = "phase"
	LabelErrorType   = "error_type"
	LabelRemediation = "remediation_type"

	// Result label values.
	ResultSuccess = "success"
	ResultError   = "error"

	// Error type label values.
	ErrorTypeTransient = "transient"
	ErrorTypeTerminal  = "terminal"
)

var (
	// Metal3MachineReconcileTotal counts the total number of Metal3Machine reconciliations.
	Metal3MachineReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "metal3machine_reconcile_total",
			Help:      "Total number of Metal3Machine reconciliations.",
		},
		[]string{LabelNamespace, LabelCluster, LabelResult},
	)

	// Metal3MachineReconcileDuration measures the duration of Metal3Machine reconciliations.
	Metal3MachineReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: Subsystem,
			Name:      "metal3machine_reconcile_duration_seconds",
			Help:      "Duration of Metal3Machine reconciliation in seconds.",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
		},
		[]string{LabelNamespace, LabelCluster, LabelResult},
	)

	// Metal3MachineProvisioningDuration measures the time from machine creation to ready state.
	Metal3MachineProvisioningDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: Subsystem,
			Name:      "metal3machine_provisioning_duration_seconds",
			Help:      "Duration from Metal3Machine creation to ready state in seconds.",
			Buckets:   []float64{30, 60, 120, 300, 600, 900, 1200, 1800, 3600},
		},
		[]string{LabelNamespace, LabelCluster},
	)

	// Metal3ClusterReconcileTotal counts the total number of Metal3Cluster reconciliations.
	Metal3ClusterReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "metal3cluster_reconcile_total",
			Help:      "Total number of Metal3Cluster reconciliations.",
		},
		[]string{LabelNamespace, LabelCluster, LabelResult},
	)

	// Metal3ClusterReconcileDuration measures the duration of Metal3Cluster reconciliations.
	Metal3ClusterReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: Subsystem,
			Name:      "metal3cluster_reconcile_duration_seconds",
			Help:      "Duration of Metal3Cluster reconciliation in seconds.",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{LabelNamespace, LabelCluster, LabelResult},
	)

	// BMHAssociationTotal counts the total number of BareMetalHost associations.
	BMHAssociationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "bmh_association_total",
			Help:      "Total number of BareMetalHost association attempts.",
		},
		[]string{LabelNamespace, LabelCluster, LabelResult},
	)

	// BMHAssociationDuration measures the time taken to associate a BMH.
	BMHAssociationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: Subsystem,
			Name:      "bmh_association_duration_seconds",
			Help:      "Duration of BareMetalHost association in seconds.",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{LabelNamespace, LabelCluster, LabelResult},
	)

	// ReconcileErrorsTotal counts reconciliation errors by type.
	ReconcileErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "reconcile_errors_total",
			Help:      "Total number of reconciliation errors by controller and error type.",
		},
		[]string{LabelController, LabelNamespace, LabelErrorType},
	)

	// Metal3DataReconcileTotal counts the total number of Metal3Data reconciliations.
	Metal3DataReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "metal3data_reconcile_total",
			Help:      "Total number of Metal3Data reconciliations.",
		},
		[]string{LabelNamespace, LabelResult},
	)

	// Metal3DataReconcileDuration measures the duration of Metal3Data reconciliations.
	Metal3DataReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: Subsystem,
			Name:      "metal3data_reconcile_duration_seconds",
			Help:      "Duration of Metal3Data reconciliation in seconds.",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{LabelNamespace, LabelResult},
	)

	// Metal3DataTemplateReconcileTotal counts the total number of Metal3DataTemplate reconciliations.
	Metal3DataTemplateReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "metal3datatemplate_reconcile_total",
			Help:      "Total number of Metal3DataTemplate reconciliations.",
		},
		[]string{LabelNamespace, LabelResult},
	)

	// Metal3DataTemplateReconcileDuration measures the duration of Metal3DataTemplate reconciliations.
	Metal3DataTemplateReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: Subsystem,
			Name:      "metal3datatemplate_reconcile_duration_seconds",
			Help:      "Duration of Metal3DataTemplate reconciliation in seconds.",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{LabelNamespace, LabelResult},
	)

	// Metal3RemediationReconcileDuration measures the duration of Metal3Remediation reconciliations.
	Metal3RemediationReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: Subsystem,
			Name:      "metal3remediation_reconcile_duration_seconds",
			Help:      "Duration of Metal3Remediation reconciliation in seconds.",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{LabelNamespace, LabelResult},
	)

	// Metal3RemediationTotal counts the total number of remediation operations.
	Metal3RemediationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "metal3remediation_total",
			Help:      "Total number of Metal3Remediation operations.",
		},
		[]string{LabelNamespace, LabelRemediation, LabelResult},
	)

	// Metal3RemediationDuration measures the duration of remediation operations.
	Metal3RemediationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: Subsystem,
			Name:      "metal3remediation_duration_seconds",
			Help:      "Duration of Metal3Remediation operations in seconds.",
			Buckets:   []float64{10, 30, 60, 120, 300, 600, 1200},
		},
		[]string{LabelNamespace, LabelRemediation, LabelResult},
	)

	// Metal3MachinePhaseTransitions tracks phase transitions for Metal3Machines.
	Metal3MachinePhaseTransitions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "metal3machine_phase_transitions_total",
			Help:      "Total number of Metal3Machine phase transitions.",
		},
		[]string{LabelNamespace, LabelCluster, LabelPhase},
	)

	// Metal3MachinesCount is a gauge showing current count of Metal3Machines by phase.
	Metal3MachinesCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: Subsystem,
			Name:      "metal3machines_count",
			Help:      "Current count of Metal3Machines by namespace and phase.",
		},
		[]string{LabelNamespace, LabelPhase},
	)

	// Metal3ClustersCount is a gauge showing current count of Metal3Clusters.
	Metal3ClustersCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: Subsystem,
			Name:      "metal3clusters_count",
			Help:      "Current count of Metal3Clusters by namespace.",
		},
		[]string{LabelNamespace},
	)

	// Metal3MachineTemplateReconcileTotal counts the total number of Metal3MachineTemplate reconciliations.
	Metal3MachineTemplateReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "metal3machinetemplate_reconcile_total",
			Help:      "Total number of Metal3MachineTemplate reconciliations.",
		},
		[]string{LabelNamespace, LabelResult},
	)

	// Metal3MachineTemplateReconcileDuration measures the duration of Metal3MachineTemplate reconciliations.
	Metal3MachineTemplateReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: Subsystem,
			Name:      "metal3machinetemplate_reconcile_duration_seconds",
			Help:      "Duration of Metal3MachineTemplate reconciliation in seconds.",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{LabelNamespace, LabelResult},
	)

	// Metal3LabelSyncReconcileTotal counts the total number of Metal3LabelSync reconciliations.
	Metal3LabelSyncReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "metal3labelsync_reconcile_total",
			Help:      "Total number of Metal3LabelSync reconciliations.",
		},
		[]string{LabelNamespace, LabelResult},
	)

	// Metal3LabelSyncReconcileDuration measures the duration of Metal3LabelSync reconciliations.
	Metal3LabelSyncReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: Subsystem,
			Name:      "metal3labelsync_reconcile_duration_seconds",
			Help:      "Duration of Metal3LabelSync reconciliation in seconds.",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{LabelNamespace, LabelResult},
	)
)

func init() {
	// Register all metrics with the controller-runtime metrics registry.
	metrics.Registry.MustRegister(
		Metal3MachineReconcileTotal,
		Metal3MachineReconcileDuration,
		Metal3MachineProvisioningDuration,
		Metal3ClusterReconcileTotal,
		Metal3ClusterReconcileDuration,
		BMHAssociationTotal,
		BMHAssociationDuration,
		ReconcileErrorsTotal,
		Metal3DataReconcileTotal,
		Metal3DataReconcileDuration,
		Metal3DataTemplateReconcileTotal,
		Metal3DataTemplateReconcileDuration,
		Metal3RemediationTotal,
		Metal3RemediationDuration,
		Metal3RemediationReconcileDuration,
		Metal3MachinePhaseTransitions,
		Metal3MachinesCount,
		Metal3ClustersCount,
		Metal3MachineTemplateReconcileTotal,
		Metal3MachineTemplateReconcileDuration,
		Metal3LabelSyncReconcileTotal,
		Metal3LabelSyncReconcileDuration,
	)
}

// RecordMetal3MachineReconcile records a Metal3Machine reconciliation metric.
func RecordMetal3MachineReconcile(namespace, cluster string, startTime time.Time, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	duration := time.Since(startTime).Seconds()
	Metal3MachineReconcileTotal.WithLabelValues(namespace, cluster, result).Inc()
	Metal3MachineReconcileDuration.WithLabelValues(namespace, cluster, result).Observe(duration)
}

// RecordMetal3ClusterReconcile records a Metal3Cluster reconciliation metric.
func RecordMetal3ClusterReconcile(namespace, cluster string, startTime time.Time, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	duration := time.Since(startTime).Seconds()
	Metal3ClusterReconcileTotal.WithLabelValues(namespace, cluster, result).Inc()
	Metal3ClusterReconcileDuration.WithLabelValues(namespace, cluster, result).Observe(duration)
}

// RecordBMHAssociation records a BareMetalHost association metric.
func RecordBMHAssociation(namespace, cluster string, startTime time.Time, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	duration := time.Since(startTime).Seconds()
	BMHAssociationTotal.WithLabelValues(namespace, cluster, result).Inc()
	BMHAssociationDuration.WithLabelValues(namespace, cluster, result).Observe(duration)
}

// RecordReconcileError records a reconciliation error metric.
func RecordReconcileError(controller, namespace string, isTransient bool) {
	errorType := ErrorTypeTerminal
	if isTransient {
		errorType = ErrorTypeTransient
	}
	ReconcileErrorsTotal.WithLabelValues(controller, namespace, errorType).Inc()
}

// RecordMetal3MachineProvisioning records the provisioning duration when a machine becomes ready.
func RecordMetal3MachineProvisioning(namespace, cluster string, creationTime time.Time) {
	duration := time.Since(creationTime).Seconds()
	Metal3MachineProvisioningDuration.WithLabelValues(namespace, cluster).Observe(duration)
}

// RecordMetal3DataReconcile records a Metal3Data reconciliation metric.
func RecordMetal3DataReconcile(namespace string, startTime time.Time, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	duration := time.Since(startTime).Seconds()
	Metal3DataReconcileTotal.WithLabelValues(namespace, result).Inc()
	Metal3DataReconcileDuration.WithLabelValues(namespace, result).Observe(duration)
}

// RecordMetal3DataTemplateReconcile records a Metal3DataTemplate reconciliation metric.
func RecordMetal3DataTemplateReconcile(namespace string, startTime time.Time, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	duration := time.Since(startTime).Seconds()
	Metal3DataTemplateReconcileTotal.WithLabelValues(namespace, result).Inc()
	Metal3DataTemplateReconcileDuration.WithLabelValues(namespace, result).Observe(duration)
}

// RecordMetal3RemediationReconcile records a Metal3Remediation reconciliation metric.
func RecordMetal3RemediationReconcile(namespace string, startTime time.Time, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	duration := time.Since(startTime).Seconds()
	Metal3RemediationTotal.WithLabelValues(namespace, "reconcile", result).Inc()
	Metal3RemediationReconcileDuration.WithLabelValues(namespace, result).Observe(duration)
}

// RecordMetal3Remediation records a remediation metric.
func RecordMetal3Remediation(namespace, remediationType string, startTime time.Time, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	duration := time.Since(startTime).Seconds()
	Metal3RemediationTotal.WithLabelValues(namespace, remediationType, result).Inc()
	Metal3RemediationDuration.WithLabelValues(namespace, remediationType, result).Observe(duration)
}

// RecordMetal3MachinePhaseTransition records a phase transition for a Metal3Machine.
func RecordMetal3MachinePhaseTransition(namespace, cluster, phase string) {
	Metal3MachinePhaseTransitions.WithLabelValues(namespace, cluster, phase).Inc()
}

// SetMetal3MachinesCount sets the gauge for Metal3Machines count.
func SetMetal3MachinesCount(namespace, phase string, count float64) {
	Metal3MachinesCount.WithLabelValues(namespace, phase).Set(count)
}

// SetMetal3ClustersCount sets the gauge for Metal3Clusters count.
func SetMetal3ClustersCount(namespace string, count float64) {
	Metal3ClustersCount.WithLabelValues(namespace).Set(count)
}

// RecordMetal3MachineTemplateReconcile records a Metal3MachineTemplate reconciliation metric.
func RecordMetal3MachineTemplateReconcile(namespace string, startTime time.Time, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	duration := time.Since(startTime).Seconds()
	Metal3MachineTemplateReconcileTotal.WithLabelValues(namespace, result).Inc()
	Metal3MachineTemplateReconcileDuration.WithLabelValues(namespace, result).Observe(duration)
}

// RecordMetal3LabelSyncReconcile records a Metal3LabelSync reconciliation metric.
func RecordMetal3LabelSyncReconcile(namespace string, startTime time.Time, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	duration := time.Since(startTime).Seconds()
	Metal3LabelSyncReconcileTotal.WithLabelValues(namespace, result).Inc()
	Metal3LabelSyncReconcileDuration.WithLabelValues(namespace, result).Observe(duration)
}
