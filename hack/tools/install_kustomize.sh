#!/bin/bash -x

[[ -f bin/kustomize ]] && exit 0

version=3.5.4
arch=amd64

mkdir -p ./bin
curl -L -O "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${version}/kustomize_v${version}_linux_${arch}.tar.gz"

tar -xzvf kustomize_v${version}_linux_${arch}.tar.gz
mv kustomize ./bin

rm kustomize_v${version}_linux_${arch}.tar.gz
