#!/bin/sh
# shellcheck disable=SC2292

set -eux

IS_CONTAINER="${IS_CONTAINER:-false}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
WORKDIR="${WORKDIR:-/workdir}"

if [ "${IS_CONTAINER}" != "false" ]; then
    export XDG_CACHE_HOME=/tmp/.cache
    mkdir /tmp/unit
    cp -r . /tmp/unit
    cd /tmp/unit
    make unit-cover-verbose
else
    "${CONTAINER_RUNTIME}" run --rm \
        --env IS_CONTAINER=TRUE \
        --volume "${PWD}:${WORKDIR}:ro,z" \
        --entrypoint sh \
        --workdir "${WORKDIR}" \
        docker.io/golang:1.25 \
        "${WORKDIR}"/hack/unit.sh "$@"
fi
