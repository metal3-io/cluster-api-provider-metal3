#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
  TOP_DIR="${1:-.}"
  find "${TOP_DIR}" -type d \( -path ./vendor -o -path ./.github \) -prune -o -name '*.md' -exec mdl --style all --warnings {} \+
else
  "${CONTAINER_RUNTIME}" run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/capm3:ro,z" \
    --entrypoint sh \
    --workdir /capm3 \
    docker.io/pipelinecomponents/markdownlint:latest \
    /capm3/hack/markdownlint.sh "${@}"
fi;
