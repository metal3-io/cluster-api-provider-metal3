#!/usr/bin/env bash
# Copyright 2025 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Standalone setup: creates a dedicated 3-node kind cluster and deploys Alloy.
# Use this when you want a separate log-collection cluster (not the capm3 dev cluster).
#
# Required env vars:
#   LOKI_USERNAME — basic auth username
#   LOKI_PASSWORD — basic auth password
#
# Optional env vars:
#   LOKI_ADDR — Loki push API endpoint (default: https://log.apps.staging.metal3.io/store/api/v1/push)

set -o errexit
set -o nounset
set -o pipefail

CLUSTER_NAME="${CLUSTER_NAME:-log-collection}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
err()  { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; }

require() {
  if ! command -v "$1" >/dev/null 2>&1; then
    err "'$1' is not installed or not on PATH. Please install it first."
    exit 1
  fi
}

log "Checking prerequisites (docker, kind, kubectl)"
require docker
require kind
require kubectl

if ! docker info >/dev/null 2>&1; then
  err "Docker daemon is not running. Start Docker and retry."
  exit 1
fi

if [[ -z "${LOKI_USERNAME:-}" || -z "${LOKI_PASSWORD:-}" ]]; then
  err "LOKI_USERNAME and/or LOKI_PASSWORD are not set in the environment."
  err "Export them first, e.g.:"
  err "  export LOKI_USERNAME='your-user'"
  err "  export LOKI_PASSWORD='your-password'"
  exit 1
fi

if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  log "kind cluster '${CLUSTER_NAME}' already exists, skipping creation"
else
  log "Creating multi-node kind cluster '${CLUSTER_NAME}'"
  kind create cluster --name "${CLUSTER_NAME}" --config "${SCRIPT_DIR}/kind-config.yaml"
fi

log "Waiting for all nodes to be Ready"
kubectl --context "kind-${CLUSTER_NAME}" wait --for=condition=Ready nodes --all --timeout=120s
kubectl --context "kind-${CLUSTER_NAME}" get nodes -o wide

export ALLOY_KUBE_CONTEXT="kind-${CLUSTER_NAME}"
"${SCRIPT_DIR}/deploy-alloy.sh"

log "Done. Verify with:  ./hack/log-collection/verify.sh"
