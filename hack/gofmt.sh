#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
  export XDG_CACHE_HOME=/tmp/.cache
  mkdir /tmp/unit
  cp -r . /tmp/unit
  cd /tmp/unit
  make fmt > /tmp/fmt-output.log
  FILE_LENGTH="$(wc -l /tmp/fmt-output.log | awk '{ print $1 }')"

  # File length to be checked should be 2 since the following headers should be present
  # go fmt ./controllers/... ./baremetal/... .
  # cd api; go fmt  ./...
  if [ "${FILE_LENGTH}" != "2" ]; then
    echo "Formatting error! Please run 'make fmt' to correct the problem."
    echo "The problematic files are listed below, after the command that should be run"
    cat /tmp/fmt-output.log
    exit 1
  fi
else
  "${CONTAINER_RUNTIME}" run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/capm3:ro,z" \
    --entrypoint sh \
    --workdir /capm3 \
    docker.io/golang:1.17 \
    /capm3/hack/gofmt.sh
fi;
