# CAPM3 Prometheus Metrics

Cluster API Provider Metal3 (CAPM3) exposes custom Prometheus metrics to provide
visibility into controller operations, provisioning workflows, and error rates.

## Overview

CAPM3 automatically registers custom metrics with the controller-runtime metrics
server (default port: 8443). These metrics are exposed at the `/metrics` endpoint
and can be scraped by Prometheus.

All metrics use the `capm3_` prefix to distinguish them from controller-runtime
and Kubernetes metrics.

## Available Metrics

### Controller Reconciliation Metrics

These metrics track the performance and success rate of each controller's
reconciliation loop.

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `capm3_metal3machine_reconcile_total` | Counter | `namespace`, `cluster`, `result` | Total number of Metal3Machine reconciliations |
| `capm3_metal3machine_reconcile_duration_seconds` | Histogram | `namespace`, `cluster`, `result` | Duration of Metal3Machine reconciliations in seconds |
| `capm3_metal3cluster_reconcile_total` | Counter | `namespace`, `result` | Total number of Metal3Cluster reconciliations |
| `capm3_metal3cluster_reconcile_duration_seconds` | Histogram | `namespace`, `result` | Duration of Metal3Cluster reconciliations in seconds |
| `capm3_metal3data_reconcile_total` | Counter | `namespace`, `result` | Total number of Metal3Data reconciliations |
| `capm3_metal3data_reconcile_duration_seconds` | Histogram | `namespace`, `result` | Duration of Metal3Data reconciliations in seconds |
| `capm3_metal3datatemplate_reconcile_total` | Counter | `namespace`, `result` | Total number of Metal3DataTemplate reconciliations |
| `capm3_metal3datatemplate_reconcile_duration_seconds` | Histogram | `namespace`, `result` | Duration of Metal3DataTemplate reconciliations in seconds |
| `capm3_metal3remediation_reconcile_total` | Counter | `namespace`, `result` | Total number of Metal3Remediation reconciliations |
| `capm3_metal3remediation_reconcile_duration_seconds` | Histogram | `namespace`, `result` | Duration of Metal3Remediation reconciliations in seconds |
| `capm3_metal3machinetemplate_reconcile_total` | Counter | `namespace`, `result` | Total number of Metal3MachineTemplate reconciliations |
| `capm3_metal3machinetemplate_reconcile_duration_seconds` | Histogram | `namespace`, `result` | Duration of Metal3MachineTemplate reconciliations in seconds |
| `capm3_metal3labelsync_reconcile_total` | Counter | `namespace`, `result` | Total number of Metal3LabelSync reconciliations |
| `capm3_metal3labelsync_reconcile_duration_seconds` | Histogram | `namespace`, `result` | Duration of Metal3LabelSync reconciliations in seconds |

### BareMetalHost Association Metrics

These metrics track the association between Metal3Machines and BareMetalHosts.

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `capm3_bmh_association_total` | Counter | `namespace`, `cluster`, `result` | Total number of BMH association attempts |
| `capm3_bmh_association_duration_seconds` | Histogram | `namespace`, `cluster`, `result` | Duration of BMH association operations in seconds |

### Provisioning Metrics

These metrics provide visibility into machine provisioning workflows.

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `capm3_metal3machine_provisioning_total` | Counter | `namespace`, `cluster`, `result` | Total number of Metal3Machine provisioning completions |
| `capm3_metal3machine_provisioning_duration_seconds` | Histogram | `namespace`, `cluster`, `result` | Duration from machine creation to Ready state |
| `capm3_metal3machine_phase_transitions_total` | Counter | `namespace`, `cluster`, `phase` | Total phase transitions for Metal3Machines |

### Remediation Metrics

These metrics track machine remediation operations.

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `capm3_metal3remediation_total` | Counter | `namespace`, `strategy`, `result` | Total number of remediation operations |
| `capm3_metal3remediation_duration_seconds` | Histogram | `namespace`, `strategy`, `result` | Duration of remediation operations in seconds |

### Error Metrics

These metrics track reconciliation errors by controller type.

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `capm3_reconcile_errors_total` | Counter | `namespace`, `controller` | Total number of reconcile errors by controller |

### Gauge Metrics

These gauge metrics are available for tracking current state counts.
Note: These require additional implementation to update values.

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `capm3_metal3machines_count` | Gauge | `namespace`, `phase` | Current count of Metal3Machines by phase |
| `capm3_metal3clusters_count` | Gauge | `namespace` | Current count of Metal3Clusters |

## Label Values

### Result Labels

The `result` label indicates the outcome of an operation:

- `success` - Operation completed successfully
- `error` - Operation failed with an error

### Phase Labels

The `phase` label indicates the current phase of a Metal3Machine:

- `provisioning` - Machine is being provisioned
- `provisioned` - Machine has been provisioned
- `deleting` - Machine is being deleted

### Controller Labels

The `controller` label identifies the source controller for error metrics:

- `metal3machine`
- `metal3cluster`
- `metal3data`
- `metal3datatemplate`
- `metal3remediation`
- `metal3machinetemplate`
- `metal3labelsync`

## Histogram Buckets

All duration histograms use the following bucket boundaries (in seconds):

```text
0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300
```

These buckets are optimized for typical reconciliation and provisioning durations,
ranging from sub-second operations to multi-minute workflows.

## Example Prometheus Queries

### Reconciliation Success Rate

```promql
# Metal3Machine reconciliation success rate (last 5 minutes)
sum(rate(capm3_metal3machine_reconcile_total{result="success"}[5m]))
/
sum(rate(capm3_metal3machine_reconcile_total[5m]))
```

### Average Reconciliation Duration

```promql
# Average Metal3Machine reconciliation duration
rate(capm3_metal3machine_reconcile_duration_seconds_sum[5m])
/
rate(capm3_metal3machine_reconcile_duration_seconds_count[5m])
```

### BMH Association Duration P99

```promql
# 99th percentile BMH association duration
histogram_quantile(0.99, rate(capm3_bmh_association_duration_seconds_bucket[5m]))
```

### Provisioning Duration P50

```promql
# Median time from machine creation to Ready state
histogram_quantile(0.50, rate(capm3_metal3machine_provisioning_duration_seconds_bucket[5m]))
```

### Error Rate by Controller

```promql
# Error rate by controller (last 5 minutes)
sum by (controller) (rate(capm3_reconcile_errors_total[5m]))
```

### Phase Transition Rate

```promql
# Rate of machines entering provisioned state
rate(capm3_metal3machine_phase_transitions_total{phase="provisioned"}[5m])
```

## Grafana Dashboard

A sample Grafana dashboard configuration can be created using these metrics.
Key panels to include:

1. **Overview Panel** - Total machines, clusters, current error rate
2. **Reconciliation Performance** - Duration histograms, success rates
3. **Provisioning Workflow** - BMH association time, provisioning duration
4. **Error Tracking** - Error counts by controller, error trends
5. **Remediation** - Remediation counts by strategy, success rate

## Alerting Examples

### High Error Rate Alert

```yaml
groups:
- name: capm3-alerts
  rules:
  - alert: CAPM3HighReconcileErrorRate
    expr: |
      sum(rate(capm3_reconcile_errors_total[5m]))
      /
      sum(rate(capm3_metal3machine_reconcile_total[5m])) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High CAPM3 reconcile error rate"
      description: "More than 10% of reconciliations are failing"
```

### Slow Provisioning Alert

```yaml
- alert: CAPM3SlowProvisioning
  expr: |
    histogram_quantile(0.95, rate(capm3_metal3machine_provisioning_duration_seconds_bucket[15m])) > 600
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "Slow Metal3Machine provisioning"
    description: "95th percentile provisioning time exceeds 10 minutes"
```

## Integration with ServiceMonitor

If using the Prometheus Operator, create a ServiceMonitor to scrape CAPM3 metrics:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: capm3-controller-manager
  namespace: capm3-system
spec:
  selector:
    matchLabels:
      control-plane: controller-manager
  endpoints:
  - port: https
    scheme: https
    path: /metrics
    tlsConfig:
      insecureSkipVerify: true
    bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
```

## Troubleshooting

### Metrics Not Appearing

1. Verify the controller manager is running and healthy
2. Check that port 8443 is accessible
3. Ensure the `/metrics` endpoint returns data:

   ```bash
   kubectl port-forward -n capm3-system svc/capm3-controller-manager-metrics-service 8443:8443
   curl -k https://localhost:8443/metrics | grep capm3_
   ```

### High Cardinality Concerns

The metrics are designed with low-cardinality labels. If you have many namespaces
or clusters, consider using recording rules to aggregate metrics:

```promql
# Recording rule for aggregate error rate
record: capm3:reconcile_errors:rate5m
expr: sum(rate(capm3_reconcile_errors_total[5m]))
```
