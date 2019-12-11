# Deployment workflow

## Deploying the controllers

The following controllers need to be deployed :

* CAPI
* CAPBK or alternative
* CAPBM
* Baremetal Operator, with Ironic setup

## Requirements

The cluster should either :

* be deployed with an external cloud provider, that would be deployed as part of
  the userData and would set the providerIDs based on the BareMetalHost UID.
* be deployed with each node with the label "metal3.io/uuid" set to the
  BareMetalHost UID that is provided by ironic to cloud-init through the
  metadata `ds.meta_data.uuid`. This can be achieved by setting the following in
  the KubeadmConfig :

```yaml
nodeRegistration:
  name: '{{ ds.meta_data.name }}'
  kubeletExtraArgs:
    node-labels: 'metal3.io/uuid={{ ds.meta_data.uuid }}'
```

## Deploying the CRs

You can deploy the CRs all at the same time, the controllers will take care of
following the correct flow.
An outline of the workflow is below.

1. The CAPI controller will set the OwnerRef on the BaremetalCluster referenced
   by the Cluster
1. The CAPBM controller will verify the apiEndpoint field and populate the
   status with ready field set to true and apiEndpoint to the content of
   apiEndpoint.
1. The CAPI controller will set infrastructureReady field to true on the Cluster
1. The CAPI controller will set the OwnerRef on the KubeadmConfig and on the
   BaremetalMachine referenced by the machine.
1. The CABPK controller will wait until the cluster has infrastructureReady set
   to true, and generate the cloud-init output for the first machine that has
   a controlplane label set to true. If the machine already has userData, this
   step and the following one are skipped.
1. The CAPI controller will copy the userData output into the userData field of
   the machine object and set the bootstrapReady field to true.
1. Once the machine has userdata, OwnerRef and bootstrapReady properly set, the
   CAPBM controller will select, if possible, a BareMetalHost that matches the
   criteria, or wait until one is available. If matched, the CAPBM controller
   will create a secret with the userData. and set the BareMetalHost spec
   accordingly to the BareMetalMachine specs.
1. The BareMetal Operator will then start the deployment.
1. After deployment, the BaremetalHost will be in provisioned state. However,
   initialization is not complete. If deploying without cloud provider, CAPBM
   will wait until the target cluster is up and the node appears. It will fetch
   the node by matching the label `metal3.io/uuid=<bmh-uuid>`. It will set the
   providerID to `metal3://<bmh-uuid>`. The BareMetalMachine ready status will
   be set to true and the providerID will be set to `metal3://<bmh-uuid>` on the
   BareMetalMachine.
1. CAPI will access the target cluster and compare the providerID on the node to
   the providerID of the Machine, copied from the BaremetalMachine. If matching,
   the control plane initialized status will be set to true and the machine
   state to running.
1. CAPBK will then generate the userData for all the other nodes and a similar
   workflow will happen for their deployment.

## Deletion

Deleting the cluster object will trigger the deletion of all related objects
except for KubeadmConfigTemplates and BareMetalMachineTemplates.
