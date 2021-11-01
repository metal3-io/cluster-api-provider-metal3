# CAPM3 machine health check and remediation

## Introduction

The ```Cluster API``` includes an optional [MachineHealthCheck (MHC)](https://cluster-api.sigs.k8s.io/tasks/healthcheck.html) component that implements automated health checking capability. With ```CAPM3 Remediation Controller``` it is possible to plug in Metal3 specific remediation strategies to remediate an unhealthy nodes while relying on Cluster API MHC to determine those nodes as unhealthy.

## CAPI MachineHealthCheck

A MachineHealthCheck is a Cluster API resource, which allows users to define conditions under which Machines within a Cluster should be considered unhealthy. Users can also specify a timeout for each of the conditions that they define to check on the Machine’s Node. If any of these conditions are met for the duration of the timeout, the Machine will be remediated. Within CAPM3 we use MHC to create remediation requests based on ```Metal3RemediationTemplate``` and ```Metal3Remediation``` CRDs to plug in our own remediation solution.

## Remediation Controller

```CAPM3 Remediation Controller (RC)``` reconciles ```Metal3Remediation``` objects created by CAPI MachineHealthCheck. The RC locates a Machine with the same name as the Metal3Remediation CR and uses existing BMO and CAPM3 APIs to remediate associated unhealthy baremetal nodes. Our remediation controller supports ```reboot strategy``` specified in Metal3Remediation CRD and uses the same object to store state of the current remediation cycle.

### Basic Remediation workflow

* RC watches for the presence of Metal3Remediation CR.
* Based on the remediation strategy defined in ```.spec.strategy.type``` in Metal3Remediation, RC uses BMO APIs to get hosts back into a healthy or manageable state.
* RC uses ```.status.phase``` to save the states of the remediation. Available states are ```running```, ```waiting```, ```deleting machine```.
* After RC have finished its remediation, it will wait for the Metal3Remediation CR to be removed. (When using CAPI MachineHealthCheck controller, MHC will noticed the Node becomes healthy and deletes the instantiated MachineRemediation CR.).

### Workflow during retry and after remediation failure

* ```.spec.strategy.retryLimit``` and  ```.spec.strategy.timeout``` defined in Metal3Remediation are used to set limit for reboot retries and time to wait between retries.
* If RCs last ```.spec.strategy.timeout``` for Node to become healthy expires, it sets ```capi.MachineOwnerRemediatedCondition``` to False on Machine object to start deletion of the unhealthy Machine and the corresponding Metal3Remediation.
* If RCs last ```.spec.strategy.timeout``` for Node to become healthy expires, it annotates BareMetalHost with ```capi.metal3.io/unhealthyannotation```.

---

### Configuration

Use the following examples as a basis for creating a MachineHealthCheck and Metal3Remediation for worker nodes:

```yaml

apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineHealthCheck
metadata:
  name: worker-healthcheck
  namespace: metal3
spec:
  # clusterName is required to associate this MachineHealthCheck with a particular cluster
  clusterName: test1
  # (Optional) maxUnhealthy prevents further remediation if the cluster is already partially unhealthy
  maxUnhealthy: 100%
  # (Optional) nodeStartupTimeout determines how long a MachineHealthCheck should wait for
  # a Node to join the cluster, before considering a Machine unhealthy.
  # Defaults to 10 minutes if not specified.
  # Set to 0 to disable the node startup timeout.
  # Disabling this timeout will prevent a Machine from being considered unhealthy when
  # the Node it created has not yet registered with the cluster. This can be useful when
  # Nodes take a long time to start up or when you only want condition based checks for
  # Machine health.
  nodeStartupTimeout: 0m
  # selector is used to determine which Machines should be health checked
  selector:
    matchLabels:
      nodepool: nodepool-0
  # Conditions to check on Nodes for matched Machines, if any condition is matched for the duration of its timeout, the Machine is considered unhealthy
  unhealthyConditions:
  - type: Ready
    status: Unknown
    timeout: 300s
  - type: Ready
    status: "False"
    timeout: 300s
  remediationTemplate: # added infrastructure reference
    kind: Metal3RemediationTemplate
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    name: worker-remediation-request

```

```yaml

apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: Metal3RemediationTemplate
metadata:
    name: worker-remediation-request
    namespace: metal3
spec:
  template:
    spec:
      strategy:
        type: "Reboot"
        retryLimit: 2
        timeout: 300s

```

Use the following examples as a basis for creating a MachineHealthCheck and Metal3Remediation for controlplane nodes:

```yaml

apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineHealthCheck
metadata:
  name: controlplane-healthcheck
  namespace: metal3
spec:
  clusterName: test1
  maxUnhealthy: 100%
  nodeStartupTimeout: 0m
  selector:
    matchLabels:
      cluster.x-k8s.io/control-plane: ""
  unhealthyConditions:
    - type: Ready
      status: Unknown
      timeout: 300s
    - type: Ready
      status: "False"
      timeout: 300s
  remediationTemplate: # added infrastructure reference
    kind: Metal3RemediationTemplate
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    name: controlplane-remediation-request

```

```yaml

apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: Metal3RemediationTemplate
metadata:
    name: controlplane-remediation-request
    namespace: metal3
spec:
  template:
    spec:
      strategy:
        type: "Reboot"
        retryLimit: 1
        timeout: 300s

```
