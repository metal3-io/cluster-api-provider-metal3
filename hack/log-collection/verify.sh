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

# Verify the Alloy DaemonSet is rolled out and pods are running.
#
# This script is designed for the standalone kind cluster created by setup.sh.
# It constructs the kubectl context as "kind-<CLUSTER_NAME>" which is the
# default context name that kind assigns when creating a cluster. If you
# deployed Alloy to a non-kind cluster (e.g. via deploy-alloy.sh in CI with a
# custom ALLOY_KUBE_CONTEXT), override CTX directly:
#
#   CTX=my-context ./hack/log-collection/verify.sh

set -o errexit
set -o nounset
set -o pipefail

CLUSTER_NAME="${CLUSTER_NAME:-log-collection}"
NAMESPACE="${NAMESPACE:-monitoring}"
CTX="${CTX:-kind-${CLUSTER_NAME}}"

log() { printf '==> %s\n' "$*"; }
err() { printf 'FAIL: %s\n' "$*" >&2; }

log "Checking DaemonSet rollout status..."
if ! kubectl --context "${CTX}" -n "${NAMESPACE}" rollout status daemonset/alloy --timeout=60s; then
  err "Alloy DaemonSet rollout has not completed."
  kubectl --context "${CTX}" -n "${NAMESPACE}" get pods -l app.kubernetes.io/name=alloy -o wide
  exit 1
fi

log "Verifying all Alloy pods are in Running state..."
NOT_RUNNING=$(kubectl --context "${CTX}" -n "${NAMESPACE}" get pods -l app.kubernetes.io/name=alloy \
  --field-selector=status.phase!=Running -o name 2>/dev/null || true)

if [[ -n "${NOT_RUNNING}" ]]; then
  err "The following Alloy pods are NOT Running:"
  echo "${NOT_RUNNING}"
  kubectl --context "${CTX}" -n "${NAMESPACE}" get pods -l app.kubernetes.io/name=alloy -o wide
  exit 1
fi

log "Verifying pod count matches node count..."
NODE_COUNT=$(kubectl --context "${CTX}" get nodes --no-headers | wc -l)
POD_COUNT=$(kubectl --context "${CTX}" -n "${NAMESPACE}" get pods -l app.kubernetes.io/name=alloy --no-headers | wc -l)

if [[ "${POD_COUNT}" -ne "${NODE_COUNT}" ]]; then
  err "Expected ${NODE_COUNT} Alloy pods (one per node), but found ${POD_COUNT}."
  kubectl --context "${CTX}" -n "${NAMESPACE}" get pods -l app.kubernetes.io/name=alloy -o wide
  exit 1
fi

log "All ${POD_COUNT} Alloy pods are Running (one per node)."

log "Recent Alloy logs (check for push errors like 401/403):"
kubectl --context "${CTX}" -n "${NAMESPACE}" logs -l app.kubernetes.io/name=alloy --tail=20 --prefix || true
