#!/usr/bin/env bash

version=1.0.8
arch=amd64

curl -L -O "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${version}/kubebuilder_${version}_linux_${arch}.tar.gz"

tar -zxvf kubebuilder_${version}_linux_${arch}.tar.gz

mv kubebuilder_${version}_linux_${arch} kubebuilder
