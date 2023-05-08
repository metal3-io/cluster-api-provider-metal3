# E2E testing

This doc gives short instructions how to run e2e tests. For the developing e2e tests, please refer to [Developing E2E tests](https://cluster-api.sigs.k8s.io/developer/e2e.html).

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

After a finished test, cleanup is performed automatically. If the test setup failed or the test was interrupted, you can perform cleanup manually:

```sh
make clean -C /opt/metal3-dev-env/metal3-dev-env/ && sudo rm -rf /opt/metal3-dev-env/
```

## Included tests

The e2e tests currently include three different sets:

1. pivoting_based_feature_test
1. remediation_based_feature_test
1. upgrade_tests
1. live_iso_tests

`pivoting_based_feature_tests`: Because these tests run mainly in the target cluster, they are dependent on the pivoting test and need to run in the following order:

- Pivoting
- Certificate rotation
- Node reuse
- Re-pivoting

However, in case we need to run them in the ephemeral cluster pivoting and re-pivoting should be ignored.

`remediation_feature_tests`: independent from the previous tests and can run independently includes:

- Remediation
- Inspection¹
- Metal3Remediation

¹ Inspection is actually run [in the middle of the remediation test](https://github.com/metal3-io/cluster-api-provider-metal3/blob/8d08f375de93a793f839b42b5ec40e6bebf98664/test/e2e/remediation_test.go#L108) for practical reasons at the moment.

The ephemeral cluster is first launched using [metal3-dev-env](https://github.com/metal3-io/metal3-dev-env).
The remediation, inspection and Metal3Remediation tests are then run with the controllers still in the ephemeral cluster either before pivoting or after re-pivoting.

`upgrade_management_cluster_test`:

- Upgrade BMO
- Upgrade Ironic
- Upgrade CAPI/CAPM3

`live_iso_test`: independent from the previous tests and can run independently. This is testing the booting of target cluster's nodes with the live ISO.

Guidelines to follow when adding new E2E tests:

- Tests should be defined in a new file and separate test spec, unless the new test depends on existing tests.
- Tests are categorized with a custom label that can be used to filter a set of E2E tests to be run. To run a subset of tests, a combination of either one or both of the `GINKGO_FOCUS` and `GINKGO_SKIP` env variables can be set. The labels are defined in the description of the `Describe` container between `[]`, for example:

`[upgrade]` => runs only existing upgrade tests including CAPI, CAPM3, Ironic and Baremetal Operator.
`[remediation]` => runs only remediation and inspection tests.
`[live-iso]` => runs only live ISO test.

For instance, to skip the upgrade E2E tests set `GINKGO_SKIP="[upgrade]"`

`GINKGO_FOCUS` and `GINKGO_SKIP` are defined in the Makefile, and should be initialized in the CI JJBs to select the target e2e tests.

### Note on BMO and Ironic upgrade

Both Ironic and BMO upgrade tests currently only check that a new version can be rolled out (by going from `latest` to `main`).
However, they do not actually upgrade from some older release to a newer since they are not yet integrated with the e2e upgrade test.
The idea is to move them to the e2e upgrade test and then upgrade Ironic and BMO together with CAPM3 from the previous minor release to the latest.

### Test matrix for k8s version

Current e2e tests use the following Kubernetes versions for source and target clusters:

| tests | bootstrap cluster | metal3 cluster init  | metal3 cluster final  |
| ----------- | ------------ | ----------- | ------------- |
| integration | v1.27.1 | v1.27.1 | x |
| remediation | v1.27.1 | v1.27.1 | x |
| pivot based feature | v1.27.1 | v1.26.4 | v1.27.1 |
| upgrade | v1.27.1 | v1.26.4 | v1.27.1 |
