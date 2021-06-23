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
