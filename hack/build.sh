#!/bin/sh
# shellcheck disable=SC2292

set -eux

IS_CONTAINER="${IS_CONTAINER:-false}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
WORKDIR="${WORKDIR:-/workdir}"
BUILD_FKAS="${BUILD_FKAS:-false}"


if [ "${IS_CONTAINER}" != "false" ]; then
    export XDG_CACHE_HOME=/tmp/.cache
    mkdir /tmp/build
    cp -r . /tmp/build
    cd /tmp/build

    if [ "${BUILD_FKAS}" != "false" ]; then
        make build-fkas
    else
        make build
    fi
else
    "${CONTAINER_RUNTIME}" run --rm \
        --env IS_CONTAINER=TRUE \
        --volume "${PWD}:${WORKDIR}:ro,z" \
        --entrypoint sh \
        --workdir "${WORKDIR}" \
        docker.io/golang:1.25 \
        "${WORKDIR}"/hack/build.sh "$@"
fi
