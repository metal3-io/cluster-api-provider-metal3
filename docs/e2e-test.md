# E2E testing

This doc gives short instructions how to run e2e tests. For the developing e2e
tests, please refer to
[Developing E2E tests](https://cluster-api.sigs.k8s.io/developer/e2e.html).

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

Run e2e tests with

```sh
make test-e2e
```

## Cleanup

After a finished test, cleanup is performed automatically. If the test setup
failed or the test was interrupted, you can perform cleanup manually:

```sh
make clean -C /opt/metal3-dev-env/metal3-dev-env/
sudo rm -rf /opt/metal3-dev-env/
```

## Included tests

The e2e tests currently include three different sets:

1. Pivoting based feature tests
1. Remediation based feature tests
1. clusterctl upgrade tests
1. K8s upgrade tests

### Pivoting based feature tests

Because these tests run mainly in the target cluster,
they are dependent on the pivoting test and need to run in the following
order:

- Pivoting
- Certificate rotation
- Node reuse
- Re-pivoting

However, in case we need to run them in the ephemeral cluster pivoting and
re-pivoting should be ignored.

### Remediation based feature tests

Independent from the previous tests and can run independently includes:

- Remediation
- Inspection¹
- Metal3Remediation

¹ Inspection is actually run
[in the middle of the remediation test](https://github.com/metal3-io/cluster-api-provider-metal3/blob/8d08f375de93a793f839b42b5ec40e6bebf98664/test/e2e/remediation_test.go#L108)
for practical reasons at the moment.

The ephemeral cluster is first launched using
[metal3-dev-env](https://github.com/metal3-io/metal3-dev-env). The remediation,
inspection and Metal3Remediation tests are then run with the controllers still
in the ephemeral cluster either before pivoting or after re-pivoting.

### clusterctl upgrade tests

- Upgrade BMO
- Upgrade Ironic
- Upgrade CAPI/CAPM3

| tests         | CAPM3 from  | CAPM3 to  | CAPI from  | CAPI to         |
| --------------| ----------- | --------- | ---------- |---------------- |
| v1.7=>current | v1.7.0      | main      | v1.7.3     | latest release  |
| v1.6=>current | v1.6.1      | main      | v1.6.3     | latest release  |
| v1.5=>current | v1.5.3      | main      | v1.5.7     | latest release  |

### K8s upgrade tests

Kubernetes version upgrade in target nodes. We run three latest version
upgrades in k8s-upgrade tests for main branch and one kubernetes upgrade
version for each release branch. When a new Kubernetes minor release is
available, we will try to support it in an upcoming CAPM3 patch release
(only in the latest supported CAPM3 minor release).

For example:

Main branch k8s-upgrade tests:

- `v1.27` => `v1.28`
- `v1.28` => `v1.29`
- `v1.29` => `v1.30`

Release 1.7 branch k8s-upgrade test:

- `v1.29` => `v1.30`

Release 1.6 branch k8s-upgrade test:

- `v1.28` => `v1.29`

Release 1.5 branch k8s-upgrade test:

- `v1.26` => `v1.27`

When Kubernetes 1.31 is released, k8s-upgrade `v1.30` => `v1.31` will be
supported in v1.7.x (but not in v1.6.x)

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

<!-- markdownlint-disable MD013 -->

| tests               | bootstrap cluster | metal3 cluster init | metal3 cluster final |
| ------------------- | ----------------- | ------------------- | -------------------- |
| integration         | v1.29.0           | v1.29.0             | x                    |
| remediation         | v1.29.0           | v1.29.0             | x                    |
| pivot based feature | v1.29.0           | v1.28.1             | v1.29.0              |
| upgrade             | v1.29.0           | v1.28.1             | v1.29.0              |

<!-- markdownlint-enable MD013 -->
