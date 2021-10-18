#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
  export XDG_CACHE_HOME=/tmp/.cache
  mkdir /tmp/unit
  cp -r . /tmp/unit
  cd /tmp/unit
  make unit-cover-verbose
else
  "${CONTAINER_RUNTIME}" run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/capm3:ro,z" \
    --entrypoint sh \
    --workdir /capm3 \
    docker.io/golang:1.17 \
    /capm3/hack/unit.sh "${@}"
fi;
