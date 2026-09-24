#!/usr/bin/env bash

# Copyright 2019 The Kubernetes Authors.
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

if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

GOPATH_BIN="$(go env GOPATH)/bin"
MINIMUM_KIND_VERSION=v0.20.0
goarch="$(go env GOARCH)"
goos="$(go env GOOS)"
KIND_URL="https://github.com/kubernetes-sigs/kind/releases/download/${MINIMUM_KIND_VERSION}/kind-${goos}-${goarch}"

# Ensure the kind tool exists and is a viable version, or installs it
verify_kind_version()
{
    # If kind is not available on the path, get it
    if [[ ! -x "$(command -v kind)" ]]; then
        if [[ "${goos}" == "linux" ]] || [[ "${goos}" == "darwin" ]]; then

            local tmp_dir checksum expected_checksum

            echo "kind not found, installing"

            if ! command -v sha256sum &>/dev/null; then
                echo "ERROR: sha256sum not found. On macOS, install coreutils: brew install coreutils" >&2
                exit 1
            fi

            tmp_dir="$(mktemp -d)"

            # shellcheck disable=SC2064 # Intentional: expand tmp_dir now since it's local
            trap "rm -rf '${tmp_dir}'" RETURN EXIT

            # Download the checksum for the kind binary
            if ! curl --proto '=https' --tlsv1.3 -sSfL \
                --retry 3 --retry-delay 5 --max-time 120 \
                -o  "${tmp_dir}/kind.sha256sum" "${KIND_URL}.sha256sum"; then
                echo >&2 "fatal: failed to download kind checksum from ${KIND_URL}.sha256sum"
                return 1
            fi

            # Download the kind binary
            if ! curl --proto '=https' --tlsv1.3 -sSfL \
                --retry 3 --retry-delay 5 --max-time 120 \
                -o  "${tmp_dir}/kind" "${KIND_URL}"; then
                echo >&2 "fatal: failed to download kind from ${KIND_URL}"
                return 1
            fi

            # Verify checksum before using
            checksum="$(sha256sum "${tmp_dir}/kind" | awk '{print $1;}')"
            expected_checksum="$(awk '{print $1;}' "${tmp_dir}/kind.sha256sum")"
            if [[ "${checksum}" != "${expected_checksum}" ]]; then
                echo >&2 "fatal: ${KIND_URL} checksum '${checksum}' differs from expected '${expected_checksum}'"
                return 1
            else
                echo "kind checksum ${checksum} verified"
            fi

            # Install binary
            if [[ ! -d "${GOPATH_BIN}" ]]; then
                mkdir -p "${GOPATH_BIN}"
            fi
            mv "${tmp_dir}/kind" "${GOPATH_BIN}/kind"
            chmod +x "${GOPATH_BIN}/kind"
        else
            echo "Missing required binary in path: kind"
            return 2
        fi
    fi

    local kind_version
    if [[ -x "$(command -v kind)" ]]; then
        kind_version="v$(kind version -q)"
    else
        echo "warning: GOPATH_BIN=${GOPATH_BIN} not in your path"
        kind_version="v$("${GOPATH_BIN}"/kind version -q)"
    fi

    if [[ "${MINIMUM_KIND_VERSION}" != $(echo -e "${MINIMUM_KIND_VERSION}\n${kind_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
        cat << EOF
Detected kind version: ${kind_version}.
Requires ${MINIMUM_KIND_VERSION} or greater.
Please install ${MINIMUM_KIND_VERSION} or later.
EOF
        return 2
    fi
}

verify_kind_version
