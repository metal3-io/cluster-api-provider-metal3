# Deployment workflow

## Deploying the controllers

The following controllers need to be deployed :

* CAPI
* CAPBK or alternative
* CACPK or alternative
* CAPM3
* Baremetal Operator, with Ironic setup

## Requirements

The cluster should either:

* be deployed with an external cloud provider that sets `spec.providerID` on
  nodes using the BareMetalHost UID, or
* be deployed with each node carrying the label `metal3.io/uuid` set to the
  BareMetalHost UID, passed through Ironic cloud-init metadata as
  `ds.meta_data.uuid`.

In both cases CAPM3 sets `spec.providerID` on the `Metal3Machine` using the
format `metal3://<namespace>/<bmh-name>/<m3m-name>`. The legacy format
`metal3://<bmh-uuid>` will be deprecated in CAPM3 v1.13 and removed in
CAPM3 v1.14.

See
[ProviderID Workflow](https://book.metal3.io/capm3/providerid-workflow.html)
for the full explanation of how CAPM3 assigns and propagates the ProviderID,
including the required kubeadm configuration snippet and the complete
reconciler decision tree.

## Deploying the CRs

You can deploy the CRs all at the same time, the controllers will take care of
following the correct flow.
An outline of the workflow is below.

1. The CAPI controller will set the OwnerRef on the Metal3Cluster referenced
   by the Cluster, on the KubeadmControlPlane, and all machines, KubeadmConfig
   and Metal3Machines created by the user or by a MachineDeployment.
1. The CAPM3 controller will verify the controlPlaneEndpoint field and populate
   the status with ready field set to true.
1. The CAPI controller will set infrastructureReady field to true on the Cluster
1. The CAPI controller will set the OwnerRef
1. The KubeadmControlPlane controller will wait until the cluster has
   infrastructureReady set to true, and generate the first machine,
   Metal3Machine and KubeadmConfig.
1. CABPK will generate the cloud-init output for this machine and create a
   secret containing it.
1. The CAPI controller will copy the userData secret name into the machine
   object and set the bootstrapReady field to true.
1. Once the secrets exists, the CAPM3 controller will select, if
   possible, a BareMetalHost that matches the criteria, or wait until one is
   available.
1. Once the machine has been associated with a BaremetalHost, the CAPM3
   controller will check if the Metal3Machine references a
   Metal3DataTemplate object. In that case, it will create Metal3DataClaim and
   waits for the metadata and/or network data secrets to be created.
1. The CAPM3 controller reconciling the Metal3DataTemplate object will select
   the lowest available index for the new machine and create a Metal3Data
   object that will then create the secrets containing the rendered data.
1. The CAPM3 controller will then set the BareMetalHost spec accordingly to the
   Metal3Machine specs.
1. The BareMetal Operator will then start the deployment.
1. After deployment, the BareMetalHost will be in provisioned state. However,
   initialization is not complete. If deploying without a cloud provider, CAPM3
   waits until the workload-cluster node appears, then locates it by matching
   the label `metal3.io/uuid=<BMH-UID>` and sets `spec.providerID` to
   `metal3://<namespace>/<bmh-name>/<m3m-name>` on both the `Metal3Machine`
   and the node. The `Metal3Machine` ready status is then set to true. See
   [ProviderID Workflow](https://book.metal3.io/capm3/providerid-workflow.html)
   for details of all paths, including the hostname-based fallback.
1. CAPI will access the target cluster and compare the providerID on the node to
   the providerID of the Machine, copied from the metal3machine. If matching,
   the control plane initialized status will be set to true and the machine
   state to running.
1. CACPK will then do the same for each further controller node until reaching
   the desired replicas number, one by one, triggering the same workflow.
   Meanwhile, as soon as the controlplane is initialized, CABPK will generate
   the user data for all other machines, triggering their deployment.

## Deletion

Deleting the cluster object will trigger the deletion of all related objects
except for KubeadmConfigTemplates, Metal3MachineTemplates, Metal3DataTemplates
and BareMetalHosts, and the secrets related to the BareMetalHosts.
