#!/bin/bash -x
mkdir -p $GOPATH/bin
curl -L https://github.com/kubernetes-sigs/kustomize/releases/download/v1.0.11/kustomize_1.0.11_linux_amd64 -o $GOPATH/bin/kustomize
chmod +x $GOPATH/bin/kustomize
