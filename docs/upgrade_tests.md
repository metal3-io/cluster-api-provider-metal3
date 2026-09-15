# Upgrade Tests

The test upgrades Kubernetes from one version to a newer version,
updating both control plane and worker nodes.

| Variable          | Value   |
| ---------------------- | ------- |
| `kubernetesVersion`    | Initial Kubernetes version |
| `upgradedK8sVersion`   | Target Kubernetes version |
| `numberOfControlplane` | 3       |
| `numberOfWorkers`      | 1       |

## upgrade_kubernetes_test

### Initial preparation

Apply BMH resources for the target cluster. Create the target
cluster with Kubernetes version `kubernetesVersion`. Provision
control plane and worker nodes with the quantities controlled by
`numberOfControlplane` and `numberOfWorkers` respectively. By
default: 3 control plane machines and 1 worker machine, resulting
in a total of 4 nodes. Also create the management cluster objects
`KubeadmControlPlane` and `MachineDeployment` to control the
control plane and worker nodes respectively.

Log the following variables:

- `kubernetesVersion`
- `upgradedK8sVersion`
- `numberOfControlplane`
- `numberOfWorkers`

Then list:

- `BareMetalHosts`
- `Metal3Machines`
- `Machines`
- `Nodes`

Then trigger the upgrade flow.

### Upgrade control plane machines

1. Get the `upgradedK8sVersion` of Kubernetes and download the
   image.
1. Create a new `Metal3MachineTemplate` for the
   `KubeadmControlPlane` and attach the new `imageURL` and
   `imageChecksum` to it.
1. Get the `KubeadmControlPlane` and start patching it. Change
   its `MachineTemplate` to the new `Metal3MachineTemplate`.
   Change its `version` to the `upgradedK8sVersion` and set
   `MaxSurge` to 0 to ensure a strict replacement-style rollout.
1. The KCP controller notices the changes and starts a rolling
   replacement of control-plane machines that no longer match
   desired state (i.e. still running `kubernetesVersion`).
1. Wait until a single BMH reaches the deprovisioning state.
1. Wait until all three control plane machines become running and
   updated with the `upgradedK8sVersion`.
1. Remove `NoSchedule` taints from the CP nodes to enable easier
   scheduling for future steps and restore `maxSurge` to 1.

### Upgrade worker machines

1. Create a new `Metal3MachineTemplate` for the
   `MachineDeployment` and attach the new `imageURL` and
   `imageChecksum` to it.
1. Get the `MachineDeployment` object and start patching it.
   Change its `MachineTemplate` to the new
   `Metal3MachineTemplate`. Change its `version` to the
   `upgradedK8sVersion`. Set `MaxSurge` to 0 and
   `maxUnavailable` to 1 to ensure a strict replacement-style
   rollout where one machine is replaced at a time.
1. The MD controller notices the changes and starts a rolling
   replacement of the worker node that no longer matches the
   desired state (i.e. still running `kubernetesVersion`).
1. Wait until the BMH reaches the deprovisioning state, then
   wait until a single BMH reaches the provisioning state.
1. Wait for the BMH to become provisioned.
1. Wait until it starts running with the `upgradedK8sVersion`.

Verify that nodes are running and their version is the
`upgradedK8sVersion`. If so, the test is successful.

### Logging and Cleanup

List:

- `BareMetalHosts`
- `Metal3Machines`
- `Machines`
- `Nodes`

Call the `DumpSpecResourcesAndCleanup` function which removes the
`clusterctlLogFolder`. Write all the logs from the target cluster
to a file.

Dump all the resources in the spec namespace to artifacts.

Check the `skipCleanup` flag and if false, send the signal to
delete all clusters. Wait until all `Metal3Data`,
`Metal3DataTemplate`, `Metal3DataClaim` objects are gone, then
end the test.

---

## E2E Upgrade Tests

### Initial setup

Import helper functions and set up variables to ensure access to
the proper versions of `capm3` and `IPAM` repos.

#### Setting up the bare metal lab with vbmctl

The `ci-e2e.sh` script builds a `vbmctl` CLI binary and
generates a configuration file from the template at
`test/e2e/config/vbmctl.yaml.tmpl`. It then calls
`vbmctl create bml` to set up the virtual bare metal lab which
provisions VMs, networks, a BMC emulator, and an image server.

The script ensures Docker and Go are available (using
`hack/ensure-docker.sh` and `hack/install-go.sh`), then
`hack/build-vbmctl.sh` compiles the vbmctl binary and
`hack/setup-bml.sh` orchestrates the lab creation.

#### Bootstrapping the cluster

Start a bootstrap cluster and create the metal3 namespace.
Configure and initialize the clusterctl provider, installing
CAPI, CAPM3, and IPAM providers and controllers. Launch BMO and
Ironic. Generate BMH resources programmatically (via
`bmh_generator.go`) rather than relying on pre-generated YAML
files.

Verify that all expected CRDs, deployments, and containers exist
without issues.

#### Finalizing the start

`ci-e2e.sh` configures image overrides and updates them if
necessary. It then runs the Go E2E upgrade tests.

There are 4 upgrade tests available.

### upgrade_kubernetes_test (E2E)

This test manually updates both control plane and worker nodes.

Create BMH resources for the target. Create the target cluster
with an outdated version of Kubernetes and a number of control
plane and worker nodes. Create `KubeadmControlPlane` and
`MachineDeployment` objects to control the control plane and
worker nodes.

Trigger the upgrade flow.

Find the latest version of Kubernetes and download the image.
Create a `Metal3MachineTemplate` for the `KubeadmControlPlane`
and patch the `KubeadmControlPlane` to point to the new template,
setting it to use the latest Kubernetes version. Set `maxSurge`
to 0 to ensure a strict rollout with one-by-one in-place
replacement.

The KCP controller notices the changes and starts a rolling
replacement of control-plane machines that no longer match desired
state. Wait until one BMH moves to a deprovisioning state and
replacement control plane machines are provisioned and begin
running on the newest version. Wait until all machines are running
and updated.

Remove `NoSchedule` taints from the CP nodes to enable easier
scheduling for future steps and restore `maxSurge` to 1.

The worker-side upgrade then begins.

Create a new `Metal3MachineTemplate` for `MachineDeployment`
pointing to the updated image. Patch `MachineDeployment` to point
to the new template and upgrade to the newest version. Set
`maxSurge` to 0 and `maxUnavailable` to 1 to ensure a one-by-one
rollout where one machine is replaced at a time.

Worker replacement begins. Worker nodes are deprovisioned, updated
to the latest version, and provisioned again. Wait until all
machines are provisioned.

Verify that all machines are running and upgraded. If so, mark the
test as a success and pass the upgrade logs.

### upgrade_kubernetes_n3_test

This test ensures multiple minor version updates can happen
consecutively.

Create BMH resources for the target. Create the target cluster
with Kubernetes version `N0` and a number of control plane and
worker nodes. Create `KubeadmControlPlane` and
`MachineDeployment` objects to control the control plane and
worker nodes.

Trigger the upgrade flow. Load and log Kubernetes versions `N0`,
`N1`, `N2`, and `N3`.

Begin the first control plane upgrade from `N0` to `N1`.

Download the new image. Create a `Metal3MachineTemplate` for the
`KubeadmControlPlane` and patch the `KubeadmControlPlane` to
point to the new template, setting the new Kubernetes version. Set
`maxSurge` to 0 to ensure a strict rollout with one-by-one
in-place replacement.

The KCP controller notices the changes and starts a rolling
replacement of control-plane machines that no longer match desired
state. Wait until one BMH moves to a deprovisioning state and
replacement control plane machines are provisioned and begin
running on the newest version.

Wait until all machines are running and updated. Set `maxSurge`
to 1 and verify the upgrades.

Subsequent upgrades use the `UpgradeControlPlane` function instead
of manual patching. Repeat this process from `N1` to `N2` and
from `N2` to `N3`.

### k8s_in_place_upgrade_test

Create BMH resources for the target. Create the target cluster
with an outdated Kubernetes version and a number of control plane
and worker nodes. Create `KubeadmControlPlane` and
`MachineDeployment` objects to control the control plane and
worker nodes.

Apply an ExtensionConfig which makes the extension server register
in-place hooks `CanUpdateMachine`, `CanUpdateMachineSet`, and
`UpdateMachine`. These hooks tell CAPI to update in place.

Download the new image. Create a `Metal3MachineTemplate` for the
`KubeadmControlPlane` and patch the `KubeadmControlPlane` to
point to the new template, setting the new Kubernetes version.

CAPI requests the updates to be done in place. The `CanUpdate`
hooks return patch responses describing fields which can be safely
upgraded in place.

For each machine, `upgradeKubernetesInPlace` is called by CAPI
which upgrades kubeadm first, then applies it to the node. Then
it updates the kubelet configuration and binary, reloads the
systemd daemon, and restarts the kubelet to apply the new version.

Check the UUID of the machines to ensure no machines have been
replaced.

Then cleanup occurs.

### Cleanup

Cleanup is the same for all tests.

Log all BMHs, Metal3 machines, machines, and nodes. Delete any
temporary log folders. Collect the workload and target cluster
logs (via `logcollector.go`). Dump all Cluster API resources in
the namespace. Delete all clusters. Wait until all `Metal3Data`,
`Metal3DataTemplate`, `Metal3DataClaim` objects are gone, then
end.

**Note:** If `skipCleanup` is set to true, the clusters are not
deleted.
