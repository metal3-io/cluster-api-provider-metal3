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

# Stop and remove the host-level Alloy Docker log collector.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

printf '\033[1;34m==>\033[0m Stopping host-docker Alloy collector\n'
# The compose file marks LOKI_USERNAME/LOKI_PASSWORD as required (:?), which
# would abort teardown if they are unset. They are unused during "down", so
# supply harmless placeholders.
LOKI_USERNAME="${LOKI_USERNAME:-unused}" LOKI_PASSWORD="${LOKI_PASSWORD:-unused}" \
  docker compose -f docker-compose.host-logs.yml down
