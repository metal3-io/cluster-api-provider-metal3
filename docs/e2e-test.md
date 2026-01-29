# E2E testing

This doc gives short instructions how to run e2e tests. For the developing e2e
tests, please refer to
[Developing E2E tests](https://cluster-api.sigs.k8s.io/developer/core/e2e).

## Prerequisites

1. Make sure that `make` is available

   To install make:

   - Ubuntu

   ```sh
   sudo apt install build-essential
   ```

   - Centos

   ```sh
   sudo yum groupinstall "Development Tools"
   ```

1. Make sure that 'docker run' works without sudo, you can check it with

   ```sh
   docker ps
   ```

   Else add the user to `docker` group:

   ```sh
   sudo groupadd docker && sudo usermod -aG docker $USER && newgrp docker
   ```

## Running

The whole e2e test suite is very big and it is not recommended to run whole
suite at a time. You can easily choose the test you want to run or skip by
using`GINKGO_FOCUS` and `GINKGO_SKIP`.
For example following will run integration test:

```sh
export GINKGO_FOCUS=integration
make test-e2e
```

Below are the tests that you can use with `GINKGO_FOCUS` and `GINKGO_SKIP`

- features
   - ip-reuse
   - healthcheck
   - remediation
   - pivoting
- k8s-upgrade
- k8s-upgrade-n3
- k8s-conformance
- clusterctl-upgrade
- scalability
- integration
- basic
- capi-md-tests
- in-place-upgrade

You can combine both `GINKGO_FOCUS` and `GINKGO_SKIP` to run multiple tests
according to your requirements. For example following will run ip-reuse and
pivoting tests:

```sh
export GINKGO_FOCUS=features
export GINKGO_SKIP=healthcheck remediation
make test-e2e
```

### Testing against CAPI nightly builds

Cluster API publishes nightly versions of the project components’ manifests from
the main branch. If you want to run tests against the nightly builds you can set:

```sh
export CAPI_NIGHTLY_BUILD=true
```

The used build is from the previous day.

## Cleanup

After a finished test, cleanup is performed automatically. If the test setup
failed or the test was interrupted, you can perform cleanup manually:

```sh
make clean -C /opt/metal3-dev-env/metal3-dev-env/
sudo rm -rf /opt/metal3-dev-env/
```

## Included tests

The e2e tests currently include seven different sets:

1. Pivoting based feature tests
1. Remediation based feature tests
1. clusterctl upgrade tests
1. K8s upgrade tests
1. K8s conformance tests
1. CAPI MachineDeployment tests
1. In place upgrade test

### Pivoting based feature tests

Because these tests run mainly in the target cluster,
they are dependent on the pivoting test and need to run in the following
order:

- Pivoting
- Certificate rotation
- Node reuse
- Re-pivoting

However, in case we need to run them in the bootstrap cluster pivoting and
re-pivoting should be ignored.

### Remediation based feature tests

Independent from the previous tests and can run independently includes:

- Remediation
- Inspection¹
- Metal3Remediation

¹ Inspection is actually run
[in the middle of the remediation test](https://github.com/metal3-io/cluster-api-provider-metal3/blob/8d08f375de93a793f839b42b5ec40e6bebf98664/test/e2e/remediation_test.go#L108)
for practical reasons at the moment.

The bootstrap cluster is first launched using
[metal3-dev-env](https://github.com/metal3-io/metal3-dev-env). The remediation,
inspection and Metal3Remediation tests are then run with the controllers still
in the bootstrap cluster either before pivoting or after re-pivoting.

### clusterctl upgrade tests

- Upgrade BMO
- Upgrade Ironic
- Upgrade CAPI/CAPM3

| tests          | CAPM3 from             | CAPM3 to  | CAPI from             | CAPI to         |
| ---------------| ---------------------- | --------- | --------------------- |---------------- |
| v1.12=>current | v1.12 latest patch     | main      | v1.12 latest patch    | latest release  |
| v1.11=>current | v1.11 latest patch     | main      | v1.11 latest patch    | latest release  |
| v1.10=>current | v1.10 latest patch     | main      | v1.10 latest patch    | latest release  |

### K8s upgrade tests

Kubernetes version upgrade in target nodes. We run latest version
upgrade in k8s-upgrade tests for main branch and one kubernetes upgrade
version for each release branch. When a new Kubernetes minor release is
available, we will try to support it in an upcoming CAPM3 patch release
(only in the latest supported CAPM3 minor release).

For example:

Main branch k8s-upgrade tests:

- `v1.34` => `v1.35`

Release 1.12 branch k8s-upgrade test:

- `v1.34` => `v1.35`

Release 1.11 branch k8s-upgrade test:

- `v1.33` => `v1.34`

Release 1.10 branch k8s-upgrade test:

- `v1.32` => `v1.33`

When Kubernetes 1.36 is released, k8s-upgrade `v1.35` => `v1.36` will be
supported in latest release branch and main branch.

### K8s N+3 upgrade tests

Kubernetes N+3(v1.34) version upgrade in target control plane nodes.
We start the test with version N(v1.31) and gradually upgrade the
target cluster control plane one by one for main branch. We are
excluding the worker node upgrade and keep it to initial N version.
When a new Kubernetes minor release is available, we will try to support
it in main branch. This will have a check on weekly basis.

```sh
export GINKGO_FOCUS=k8s-upgrade-n3
```

Main branch k8s-upgrade-n3 tests:

- `v1.31` => `v1.32`

- `v1.32` => `v1.33`

- `v1.33` => `v1.34`

When Kubernetes 1.35 is released, k8s-upgrade-n3 test will be updated accordingly.

### Test matrix for n+3 k8s upgrade version

Kubernetes n+3 e2e test uses the following Kubernetes versions for n+3 upgrade control
planes:

<!-- markdownlint-disable MD013 -->
| KUBERNETES_N0_VERSION | KUBERNETES_N1_VERSION | KUBERNETES_N2_VERSION | KUBERNETES_N3_VERSION |
| --------------------- | --------------------- | --------------------- | --------------------- |
|       v1.32.9         |        v1.33.5        |       v1.34.1         |        v1.35.0        |
<!-- markdownlint-enable MD013 -->

### K8s conformance tests

The conformance tests are a subset of Kubernetes' E2E test set. The standard set
of conformance tests includes those defined by the [Conformance] tag in the
[kubernetes e2e suite](https://github.com/kubernetes/kubernetes/blob/master/test/conformance/testdata/conformance.yaml).
Refer to [Conformance Tests per Release](https://github.com/cncf/k8s-conformance/blob/master/docs/README.md)
for more information on which tests are required for each Kubernetes release.

### CAPI MachineDeployment tests

Includes the following MachineDeployment tests adopted from the Cluster API's
e2e tests:

- [MachineDeployment rolling upgrades](https://github.com/kubernetes-sigs/cluster-api/blob/main/test/e2e/md_rollout.go)
- [MachineDeployment scale](https://github.com/kubernetes-sigs/cluster-api/blob/main/test/e2e/md_scale.go)

### In place upgrade test

The in-place upgrade test upgrades Kubernetes on existing cluster nodes
(control plane) without reprovisioning or restarting machines. This approach
upgrades the Kubernetes components directly on running nodes via SSH, avoiding
the time and resource overhead of machine replacement.

The test leverages CAPI's Runtime SDK and the experimental
`EXP_IN_PLACE_UPDATES` feature gate to intercept machine update operations.
A runtime extension server implements the necessary hooks
(`DoCanUpdateMachine`, `DoCanUpdateMachineSet`, `DoUpdateMachine`) to perform
the actual upgrade via SSH.
See [test/extension/handlers/inplaceupdate/handlers.go](../test/extension/handlers/inplaceupdate/handlers.go)
for the detailed implementation.

**Test flow:**

1. Test starts 5 bmh, 3 CP nodes and 0 worker node
1. Get uids of machines before upgrade
1. Upgrades Kubernetes
1. Waits for control plane nodes to upgrade
1. Check uids of upgraded CP machines with before upgrade uids to check
   in-place upgrade
1. Scales CP machines from 3→5 to validate new node uses the new image.
1. Verifies all machines are running with the new version

**Note:** Currently, the SSH-based upgrade implementation is only available
for CentOS. The test runs exclusively with the CentOS flavor.

## Guidelines to follow when adding new E2E tests

- Tests should be defined in a new file and separate test spec, unless the new
  test depends on existing tests.
- Tests are categorized with a custom label that can be used to filter a set of
  E2E tests to be run. To run a subset of tests, a combination of either one or
  both of the `GINKGO_FOCUS` and `GINKGO_SKIP` env variables can be set. The
  labels are defined in the description of the `Describe` container between
  `[]`, for example:
   - `[clusterctl-upgrade]` => runs only existing upgrade tests including CAPI,
CAPM3, Ironic and Baremetal Operator.
   - `[remediation]` => runs only remediation and inspection tests.
   - `[k8s-upgrade]` => runs only k8s upgrade tests.

For instance, to skip the upgrade E2E tests set `GINKGO_SKIP="[upgrade]"`

`GINKGO_FOCUS` and `GINKGO_SKIP` are defined in the Makefile, and should be
initialized in the CI JJBs to select the target e2e tests.

### Note on BMO and Ironic upgrade

Both Ironic and BMO upgrade tests currently only check that a new version can be
rolled out (by going from `latest` to `main`). However, they do not actually
upgrade from some older release to a newer since they are not yet integrated
with the e2e upgrade test. The idea is to move them to the e2e upgrade test and
then upgrade Ironic and BMO together with CAPM3 from the previous minor release
to the latest.

### Test matrix for k8s version

Current e2e tests use the following Kubernetes versions for source and target
clusters:

| tests               | bootstrap cluster | metal3 cluster init | metal3 cluster final |
| ------------------- | ----------------- | --------------------| -------------------- |
| integration         | v1.35.0           | v1.35.0             | x                    |
| remediation         | v1.35.0           | v1.35.0             | x                    |
| pivot based feature | v1.35.0           | v1.35.0             | v1.35.0              |
| upgrade             | v1.35.0           | v1.35.0             | v1.35.0              |
