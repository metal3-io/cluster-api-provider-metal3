#!/bin/bash -x
mkdir -p $GOPATH/bin
curl -L https://github.com/kubernetes-sigs/kustomize/releases/download/v2.0.3/kustomize_2.0.3_linux_amd64 -o $GOPATH/bin/kustomize
chmod +x $GOPATH/bin/kustomize
