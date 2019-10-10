#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}

if [ "${IS_CONTAINER}" != "false" ]; then
  eval "$(go env)"
  cd "${GOPATH}"/src/github.com/metal3-io/cluster-api-provider-baremetal
  go test ./pkg/... ./cmd/... -coverprofile "${ARTIFACTS}"/cover.out
else
  podman run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/root/go/src/github.com/metal3-io/cluster-api-provider-baremetal:ro,z" \
    --entrypoint sh \
    --workdir /root/go/src/github.com/metal3-io/cluster-api-provider-baremetal \
    quay.io/metal3-io/capbm-unit \
    /root/go/src/github.com/metal3-io/cluster-api-provider-baremetal/hack/unit.sh "${@}"
fi;
