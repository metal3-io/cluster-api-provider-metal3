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
    docker.io/koalaman/shellcheck-alpine:stable \
    /capm3/hack/shellcheck.sh "${@}"
fi;
