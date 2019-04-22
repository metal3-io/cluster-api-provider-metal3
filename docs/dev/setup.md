# Setting up a development environment

The cluster-api requires two external tools for running the tests
during development.

## Install kustomize

```bash
eval $(go env)
export GOPATH
./tools/install_kustomize.sh
```

## Install kubebuilder

```bash
./tools/install_kubebuilder.sh
sudo mv kubebuilder /usr/local
```
