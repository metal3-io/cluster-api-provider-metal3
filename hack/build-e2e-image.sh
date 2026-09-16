#!/usr/bin/env bash

# Builds the CAPM3 controller image from source and publishes it to the local
# registry so the e2e run always tests the code in this tree, mirroring how
# metal3-dev-env published CAPM3 to ${REGISTRY}/localimages.
#
# Both target and management cluster consume it: the kind management cluster loads the image
# directly (test/e2e/config/e2e_conf.yaml images: mustLoad) and the target
# (self-hosted) cluster created during pivoting pulls it from ${REGISTRY}, which
# the bare metal nodes trust as an insecure registry (see the *-kubeadm-config
# registries.conf).
#
# Can be run standalone or from scripts/ci-e2e.sh. Honored env vars:
#   REGISTRY             - registry host:port to publish to
#                          (default ${PROVISIONING_URL_HOST}:${REGISTRY_PORT})
#   PROVISIONING_URL_HOST- provisioning-network host serving the registry
#   REGISTRY_PORT        - registry port (default 5000)
#   E2E_TAG              - image tag to build/push (default e2e)
#   CAPM3_E2E_IMAGE      - full image ref (default ${REGISTRY}/localimages/cluster-api-provider-metal3:${E2E_TAG})

set -euxo pipefail

REPO_ROOT=$(realpath "$(dirname "$(realpath "${BASH_SOURCE[0]}")")"/..)

PROVISIONING_URL_HOST="${PROVISIONING_URL_HOST:-172.22.0.1}"
REGISTRY_PORT="${REGISTRY_PORT:-5000}"
REGISTRY="${REGISTRY:-${PROVISIONING_URL_HOST}:${REGISTRY_PORT}}"
E2E_TAG="${E2E_TAG:-e2e}"
REGISTRY_PORT="${REGISTRY##*:}"
CAPM3_LOCAL_REPO="${REGISTRY}/localimages/cluster-api-provider-metal3"
CAPM3_E2E_IMAGE="${CAPM3_E2E_IMAGE:-${CAPM3_LOCAL_REPO}:${E2E_TAG}}"

# Start a throwaway local registry if one is not already running, bound on all
# interfaces so the bare metal nodes reach it via ${REGISTRY}.
if [[ -z "$(docker ps -q -f name="^registry$")" ]]; then
  if [[ -n "$(docker ps -aq -f name="^registry$")" ]]; then
    docker start registry
  else
    docker run -d --restart=always -p "${REGISTRY_PORT}:5000" --name registry registry:2
  fi
fi

make -C "${REPO_ROOT}" docker-build-e2e \
  CONTROLLER_IMG="${CAPM3_LOCAL_REPO}" \
  E2E_TAG="${E2E_TAG}"

# Push through localhost so the Docker daemon treats the registry as insecure
# (loopback is insecure by default) without editing daemon.json; the target
# nodes pull the identical blob via ${REGISTRY}.
docker tag "${CAPM3_E2E_IMAGE}" "localhost:${REGISTRY_PORT}/localimages/cluster-api-provider-metal3:${E2E_TAG}"
docker push "localhost:${REGISTRY_PORT}/localimages/cluster-api-provider-metal3:${E2E_TAG}"
