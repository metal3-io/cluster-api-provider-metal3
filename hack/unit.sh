#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}

if [ "${IS_CONTAINER}" != "false" ]; then
  #TODO Temporary hack : Remove after the image is fixed
  exit 0
  export XDG_CACHE_HOME=/tmp/.cache
  cp -r ./* /tmp/unittests
  cd /tmp/unittests
  make test
else
  podman run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/github.com/metal3-io/cluster-api-provider-baremetal:ro,z" \
    --entrypoint sh \
    --workdir /go/src/github.com/metal3-io/cluster-api-provider-baremetal \
    quay.io/metal3-io/capbm-unit:v1alpha2 \
    /go/src/github.com/metal3-io/cluster-api-provider-baremetal/hack/unit.sh "${@}"
fi;
