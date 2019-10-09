#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}

if [ "${IS_CONTAINER}" != "false" ]; then
  TOP_DIR="${1:-$(pwd)}"
  cd /
  ${TOP_DIR}/tools/install_kustomize.sh
  ${TOP_DIR}/tools/install_kubebuilder.sh
  mv kubebuilder /usr/local/.
  cd ${TOP_DIR}
  go test ./pkg/... ./cmd/... -coverprofile /cover.out
else
  podman run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/github.com/metal3-io/cluster-api-provider-baremetal:ro,z" \
    --entrypoint sh \
    --workdir /go/src/github.com/metal3-io/cluster-api-provider-baremetal \
    registry.hub.docker.com/library/golang:1.12 \
    /go/src/github.com/metal3-io/cluster-api-provider-baremetal/hack/unit.sh "${@}"
fi;
