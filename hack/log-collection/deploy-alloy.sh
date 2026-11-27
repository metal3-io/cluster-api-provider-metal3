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

# deploy-alloy.sh — Deploy Grafana Alloy to the specified kubectl context.
#
# Required env vars:
#   LOKI_USERNAME — basic auth username
#   LOKI_PASSWORD — basic auth password
#
# Optional env vars:
#   LOKI_ADDR           — Loki push API endpoint (default: https://log.apps.test.metal3.io/store/api/v1/push)
#   ALLOY_KUBE_CONTEXT  — kubectl context to target (default: current context)
#   ALLOY_CLUSTER_LABEL — value for the "cluster" external label (default: capm3)
#   ALLOY_BUILD_NUMBER  — CI build number (default: 0)
#   ALLOY_JOB           — CI job name (default: metal3ci)
#   ALLOY_PIPELINE_ID   — CI pipeline identifier (default: local)
#   ALLOY_SERVICE_NAME  — service name label (default: alloy)

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOKI_ADDR="${LOKI_ADDR:-https://log.apps.test.metal3.io/store/api/v1/push}"
CTX="${ALLOY_KUBE_CONTEXT:-$(kubectl config current-context 2>/dev/null || echo "")}"

# Label defaults
CLUSTER_LABEL="${ALLOY_CLUSTER_LABEL:-capm3}"
BUILD_NUMBER="${ALLOY_BUILD_NUMBER:-0}"
JOB="${ALLOY_JOB:-metal3ci}"
PIPELINE_ID="${ALLOY_PIPELINE_ID:-local}"
SERVICE_NAME="${ALLOY_SERVICE_NAME:-alloy}"

if [[ -z "${LOKI_USERNAME:-}" || -z "${LOKI_PASSWORD:-}" ]]; then
  echo "WARN: LOKI_USERNAME/LOKI_PASSWORD not set. Skipping Alloy log-shipping deployment." >&2
  exit 0
fi

if [[ -z "${CTX}" ]]; then
  echo "WARN: No kube context available. Skipping Alloy deployment." >&2
  exit 0
fi

# Verify Loki endpoint is reachable and credentials are valid
echo "==> Verifying Loki endpoint (${LOKI_ADDR})..."
if ! HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 \
  -u "${LOKI_USERNAME}:${LOKI_PASSWORD}" \
  -H "Content-Type: application/json" \
  -d '{"streams":[]}' \
  "${LOKI_ADDR}" 2>/dev/null); then
  HTTP_CODE="000"
fi

if [[ "${HTTP_CODE}" == "000" ]]; then
  echo "ERROR: Cannot reach Loki at '${LOKI_ADDR}' — DNS or network issue." >&2
  exit 1
elif [[ "${HTTP_CODE}" == "401" || "${HTTP_CODE}" == "403" ]]; then
  echo "ERROR: Loki returned ${HTTP_CODE} — credentials are invalid." >&2
  exit 1
elif [[ "${HTTP_CODE}" == "404" ]]; then
  echo "ERROR: Loki returned 404 — push URL '${LOKI_ADDR}' is incorrect." >&2
  exit 1
elif [[ "${HTTP_CODE}" == "422" ]]; then
  : # Loki rejected the empty payload but auth and URL are valid
elif [[ "${HTTP_CODE}" =~ ^[45] ]]; then
  echo "ERROR: Loki returned ${HTTP_CODE} — unexpected error." >&2
  exit 1
fi
echo "==> Loki endpoint verified (HTTP ${HTTP_CODE})"

echo "==> Deploying Grafana Alloy to context '${CTX}' (cluster=${CLUSTER_LABEL})..."

sed \
  -e "s|__LOKI_ADDR__|${LOKI_ADDR}|g" \
  -e "s|__LOKI_USERNAME__|${LOKI_USERNAME}|g" \
  -e "s|__LOKI_PASSWORD__|${LOKI_PASSWORD}|g" \
  -e "s|__ALLOY_CLUSTER_LABEL__|${CLUSTER_LABEL}|g" \
  -e "s|__ALLOY_BUILD_NUMBER__|${BUILD_NUMBER}|g" \
  -e "s|__ALLOY_JOB__|${JOB}|g" \
  -e "s|__ALLOY_PIPELINE_ID__|${PIPELINE_ID}|g" \
  -e "s|__ALLOY_SERVICE_NAME__|${SERVICE_NAME}|g" \
  "${SCRIPT_DIR}/alloy-manifests.yaml" | kubectl --context "${CTX}" apply -f -

echo "==> Waiting for Alloy DaemonSet rollout..."
kubectl --context "${CTX}" -n monitoring rollout status daemonset/alloy --timeout=5m

echo "==> Waiting for Alloy to attempt first push..."
sleep 15
if kubectl --context "${CTX}" -n monitoring logs -l app.kubernetes.io/name=alloy --tail=50 2>/dev/null \
    | grep -qiE "level=error|err=|failed|401|403|404|connection refused"; then
  echo "WARN: Alloy may be failing to push logs. Recent logs:" >&2
  kubectl --context "${CTX}" -n monitoring logs -l app.kubernetes.io/name=alloy --tail=10 >&2
fi

echo "==> Alloy deployed successfully. Logs are being shipped to ${LOKI_ADDR}"
