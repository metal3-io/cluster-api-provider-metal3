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

# Start a host-level Alloy container that collects logs from all Docker
# containers running on this machine and pushes them to Loki.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

log() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
err() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; }

export LOKI_ADDR="${LOKI_ADDR:-https://log.apps.test.metal3.io/store/api/v1/push}"

if ! command -v docker >/dev/null 2>&1; then
  err "docker is not installed or not on PATH."
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  err "Docker daemon is not running. Start Docker and retry."
  exit 1
fi
if [[ -z "${LOKI_USERNAME:-}" || -z "${LOKI_PASSWORD:-}" ]]; then
  err "LOKI_USERNAME and/or LOKI_PASSWORD are not set. Export them first:"
  err "  export LOKI_USERNAME='your-user'"
  err "  export LOKI_PASSWORD='your-password'"
  exit 1
fi

log "Starting host-docker Alloy collector (LOKI_ADDR=${LOKI_ADDR})"
docker compose -f docker-compose.host-logs.yml up -d

log "Container status:"
docker ps --filter name=alloy-host-logs --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
