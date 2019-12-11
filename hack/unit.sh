#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
  export XDG_CACHE_HOME=/tmp/.cache
  mkdir /tmp/unit
  cp -r ./* /tmp/unit
  cp -r /usr/local/kubebuilder/bin /tmp/unit/hack/tools
  cd /tmp/unit
  make test
else
  "${CONTAINER_RUNTIME}" run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/github.com/metal3-io/cluster-api-provider-baremetal:ro,z" \
    --entrypoint sh \
    --workdir /go/src/github.com/metal3-io/cluster-api-provider-baremetal \
    quay.io/metal3-io/capbm-unit:master \
    /go/src/github.com/metal3-io/cluster-api-provider-baremetal/hack/unit.sh "${@}"
fi;
