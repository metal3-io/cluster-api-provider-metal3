#!/bin/sh

set -eux

IS_CONTAINER="${IS_CONTAINER:-false}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
    TOP_DIR="${1:-.}"
    find "${TOP_DIR}" -type d \( -path ./vendor -o -path ./.github \) -prune -o -name '*.md' -exec mdl --style all --warnings {} \+
else
    "${CONTAINER_RUNTIME}" run --rm \
        --env IS_CONTAINER=TRUE \
        --volume "${PWD}:/workdir:ro,z" \
        --entrypoint sh \
        --workdir /workdir \
        docker.io/pipelinecomponents/markdownlint:0.12.0@sha256:0b8f9fcf0410257b2f3548f67ffe25934cfc9877a618b9f85afcf345a25804a2 \
        /workdir/hack/markdownlint.sh "$@"
fi
