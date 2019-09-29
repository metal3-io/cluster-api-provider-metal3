#!/usr/bin/env bash

set -e

dir=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"
kubectl cluster-info

# CAPI
cd ~/go/src/sigs.k8s.io/cluster-api
if [ -n "${BUILD_CAPI}" ]; then
	make docker-build	
fi
kind load docker-image gcr.io/arvinders-1st-project/cluster-api-controller-amd64:dev
make release-manifests
kubectl apply -f ~/go/src/sigs.k8s.io/cluster-api/out/cluster-api-components.yaml

# CABPK
cd ~/go/src/sigs.k8s.io/cluster-api-bootstrap-provider-kubeadm
if [ -n "${BUILD_CABPK}" ]; then
	make docker-build	
fi
kind load docker-image gcr.io/arvinders-1st-project/cluster-api-kubeadm-controller-amd64:dev
make deploy

# CAPBM
cd ~/go/src/github.com/metal3-io/cluster-api-provider-baremetal
if [ -n "${BUILD_CAPBM}" ]; then
	make docker-build	
fi
kind load docker-image controller:dev
make deploy

# Baremetal-operator 
cd ~/go/src/github.com/metal3-io/baremetal-operator
if [ -n "${BUILD_BMO}" ]; then
	make docker-build	
fi
kind load docker-image baremetal-operator:dev
make deploy

cd "${dir}"