#!/usr/bin/env bash

# Copyright 2021 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
BIN_ROOT="${KUBE_ROOT}/hack/tools/bin"
MINIMUM_KUSTOMIZE_VERSION=4.4.1

goarch="$(go env GOARCH)"
goos="$(go env GOOS)"

# Ensure the kustomize tool exists and is a viable version, or installs it
verify_kustomize_version() {

  # If kustomize is not available on the path, get it
  if ! [ -x "$(command -v "$BIN_ROOT"/kustomize)" ]; then
    if [[ "${OSTYPE}" == "linux-gnu" ]]; then
      echo 'kustomize not found, installing'
      if ! [ -d "${BIN_ROOT}" ]; then
        mkdir -p "${BIN_ROOT}"
      fi
      archive_name="kustomize-v${MINIMUM_KUSTOMIZE_VERSION}.tar.gz"
      curl -sLo "${BIN_ROOT}/${archive_name}" "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${MINIMUM_KUSTOMIZE_VERSION}/kustomize_v${MINIMUM_KUSTOMIZE_VERSION}_${goos}_${goarch}.tar.gz"
      tar -zvxf "${BIN_ROOT}/${archive_name}" -C "${BIN_ROOT}/"
      chmod +x "${BIN_ROOT}/kustomize"
      rm "${BIN_ROOT}/${archive_name}"
    else
      echo "Missing required binary in path: kustomize"
      return 2
    fi
  fi

  local kustomize_version
  kustomize_version=$("$BIN_ROOT"/kustomize version)
  if [[ "${MINIMUM_KUSTOMIZE_VERSION}" != $(echo -e "${MINIMUM_KUSTOMIZE_VERSION}\n${kustomize_version}" | sort -s -t. -k 1,1 -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected kustomize version: ${kustomize_version}.
Requires ${MINIMUM_KUSTOMIZE_VERSION} or greater.
Please install ${MINIMUM_KUSTOMIZE_VERSION} or later.
EOF
    return 2
  fi
}

verify_kustomize_version
