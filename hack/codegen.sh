#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}
ARTIFACTS=${ARTIFACTS:-/tmp}
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
  export XDG_CACHE_HOME=/tmp/.cache
  mkdir /tmp/unit
  cp -r ./* /tmp/unit
  cd /tmp/unit
  INPUT_FILES="api/v1alpha2/zz_generated.*.go api/v1alpha2/zz_generated.*.go api/v1alpha4/zz_generated.*.go baremetal/mocks/zz_generated.*.go"
  cksum $INPUT_FILES > "$ARTIFACTS/lint.cksums.before"
  export VERBOSE="--verbose"
  make generate
  cksum $INPUT_FILES > "$ARTIFACTS/lint.cksums.after"
  diff "$ARTIFACTS/lint.cksums.before" "$ARTIFACTS/lint.cksums.after"
else
  "${CONTAINER_RUNTIME}" run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/github.com/metal3-io/cluster-api-provider-metal3:ro,z" \
    --entrypoint sh \
    --workdir /go/src/github.com/metal3-io/cluster-api-provider-metal3 \
    registry.hub.docker.com/library/golang:1.13.7 \
    /go/src/github.com/metal3-io/cluster-api-provider-metal3/hack/codegen.sh
fi;
