#!/bin/sh

set -eux

IS_CONTAINER="${IS_CONTAINER:-false}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
GO_CONCURRENCY="${GO_CONCURRENCY:-8}"

if [ "${IS_CONTAINER}" != "false" ]; then
    export XDG_CACHE_HOME="/tmp/.cache"
    # It seems like gosec does not handle submodules well. Therefore we skip them and run separately.
    gosec -severity medium --confidence medium -quiet \
        -concurrency "${GO_CONCURRENCY}" -exclude-dir=api -exclude-dir=test ./...
    (cd api && gosec -severity medium --confidence medium -quiet \
        -concurrency "${GO_CONCURRENCY}" ./...)
    (cd test && gosec -severity medium --confidence medium -quiet \
        -concurrency "${GO_CONCURRENCY}" ./...)
else
    "${CONTAINER_RUNTIME}" run --rm \
        --env IS_CONTAINER=TRUE \
        --volume "${PWD}:/workdir:ro,z" \
        --entrypoint sh \
        --workdir /workdir \
        docker.io/securego/gosec:2.17.0@sha256:4ea9b6053eac43abda841af5885bbd31ee1bf7289675545b8858bcedb40b4fa8 \
        /workdir/hack/gosec.sh
fi
