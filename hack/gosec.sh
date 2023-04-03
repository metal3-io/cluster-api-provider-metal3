#!/bin/sh

set -eux

IS_CONTAINER="${IS_CONTAINER:-false}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
    export XDG_CACHE_HOME="/tmp/.cache"
    gosec -exclude=G107 -severity medium -confidence medium -concurrency 8 -quiet ./...
else
    "${CONTAINER_RUNTIME}" run --rm \
        --env IS_CONTAINER=TRUE \
        --volume "${PWD}:/workdir:ro,z" \
        --entrypoint sh \
        --workdir /workdir \
        docker.io/securego/gosec:2.14.0@sha256:73858f8b1b9b7372917677151ec6deeceeaa40c5b02753080bd647dede14e213 \
        /workdir/hack/gosec.sh
fi
