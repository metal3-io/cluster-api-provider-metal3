# E2E testing

This doc gives short instructions how to run e2e tests. For the developing e2e tests, please refer to [Developing E2E tests](https://cluster-api.sigs.k8s.io/developer/e2e.html).

## Prerequisites

- Make sure that `make` and `go` are available
- Make sure that 'docker run' works without sudo, you can check it with

  ```sh
  docker run hello-world
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

The e2e tests currently run one after the other, using the same environment for all of them.
They run in this order:

1. Remediation
1. Inspection¹
1. Pivoting
1. Upgrade BMO
1. Upgrade Ironic
1. Certificate rotation
1. Node reuse
1. Re-pivoting

¹ Inspection is actually run [in the middle of the remediation test](https://github.com/metal3-io/cluster-api-provider-metal3/blob/8d08f375de93a793f839b42b5ec40e6bebf98664/test/e2e/remediation_test.go#L108) for practical reasons at the moment.

The ephemeral cluster is first launched using [metal3-dev-env](https://github.com/metal3-io/metal3-dev-env).
The remediation and inspection tests are then run with the controllers still in the ephemeral cluster.
After this, the controllers are moved to the target cluster during pivoting.
From this point on all tests are run in the target cluster, until the controllers are moved back to the ephemeral cluster during re-pivoting.

### Note on BMO and Ironic upgrade

Both Ironic and BMO upgrade tests currently only check that a new version can be rolled out (by going from `latest` to `main`).
However, they do not actually upgrade from some older release to a newer since they are not yet integrated with the e2e upgrade test.
The idea is to move them to the e2e upgrade test and then upgrade Ironic and BMO together with CAPM3 from the previous minor release to the latest.
