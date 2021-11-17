#!/usr/bin/env bash

[[ -f bin/kubebuilder ]] && exit 0

# kubebuilder version
kb_version=3.2.0

mkdir -p ./bin
cd ./bin || exit
curl -L -o kubebuilder "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${kb_version}/kubebuilder_$(go env GOOS)_$(go env GOARCH)"
chmod +x kubebuilder
