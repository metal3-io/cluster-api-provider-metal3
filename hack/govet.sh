#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}

if [ "${IS_CONTAINER}" != "false" ]; then
  TOP_DIR="${1:-.}"
  go vet ${TOP_DIR}/pkg/... ${TOP_DIR}/cmd/...
else
  podman run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/github.com/metal3-io/cluster-api-provider-baremetal:ro,z" \
    --entrypoint sh \
    --workdir /go/src/github.com/metal3-io/cluster-api-provider-baremetal \
    registry.hub.docker.com/library/golang:1.12 \
    /go/src/github.com/metal3-io/cluster-api-provider-baremetal/hack/govet.sh "${@}"
fi;
