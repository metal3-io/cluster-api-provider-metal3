#!/bin/bash -x

[[ -f bin/kustomize ]] && exit 0

mkdir -p ./bin
curl -L https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv3.2.3/kustomize_kustomize.v3.2.3_linux_amd64 -o ./bin/kustomize
chmod +x ./bin/kustomize
