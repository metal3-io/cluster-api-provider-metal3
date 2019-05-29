# Setting up a development environment

The cluster-api requires two external tools for running the tests
during development.

## Install kustomize

```bash
go get sigs.k8s.io/kustomize
```

## Install kubebuilder

```bash
./tools/install_kubebuilder.sh
sudo mv kubebuilder /usr/local
```
