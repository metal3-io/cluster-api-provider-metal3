#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
  TOP_DIR="${1:-.}"
  find "${TOP_DIR}" -path ./vendor -prune -o -name '*.sh' -exec shellcheck -s bash {} \+
else
  "${CONTAINER_RUNTIME}" run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/capm3:ro,z" \
    --entrypoint sh \
    --workdir /capm3 \
    docker.io/koalaman/shellcheck-alpine:v0.9.0@sha256:e19ed93c22423970d56568e171b4512c9244fc75dd9114045016b4a0073ac4b7 \
    /capm3/hack/shellcheck.sh "${@}"
fi;
