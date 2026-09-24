#!/usr/bin/env bash

# Copyright 2023 The Metal3 Authors.
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

# Download golangci-lint and verify checksum from official checksums.txt

set -o errexit
set -o nounset
set -o pipefail

if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

IPAM_DIR="$(cd "$(dirname "$0")/.." && pwd -P)"
GOLANGCI_LINT="golangci-lint"
GOLANGCI_VERSION="2.10.1"
KERNEL_OS="$(uname | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')"
CHECKSUMS_URL="https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_VERSION}/golangci-lint-${GOLANGCI_VERSION}-checksums.txt"
CHECKSUMS_FILE="checksums.txt"
BINARY_FILE="${GOLANGCI_LINT}-${GOLANGCI_VERSION}-${KERNEL_OS}-${ARCH}.tar.gz"
URL="https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_VERSION}/${BINARY_FILE}"

download_and_install_golangci_lint()
{
    local tmp_dir
    local bin_dir="${1:?Binary path missing}"

    if ! command -v sha256sum &>/dev/null; then
        echo "ERROR: sha256sum not found. On macOS, install coreutils: brew install coreutils" >&2
        exit 1
    fi

    tmp_dir="$(mktemp -d)"

    # shellcheck disable=SC2064 # Intentional: expand tmp_dir now since it's local
    trap "rm -rf '${tmp_dir}'" RETURN EXIT

    # Download checksums file
    if ! curl --proto '=https' --tlsv1.3 -sSfL \
        --retry 3 --retry-delay 5 --max-time 120 \
        -o "${tmp_dir}/${CHECKSUMS_FILE}" "${CHECKSUMS_URL}"; then
        echo >&2 "fatal: failed to download checksums from ${CHECKSUMS_URL}"
        return 1
    fi

    # Extract the checksum for this platform
    GOLANGCI_SHA256="$(grep -F -- "${BINARY_FILE}" "${tmp_dir}/${CHECKSUMS_FILE}" | awk '{print $1;}')" || true
    if [[ -z "${GOLANGCI_SHA256}" ]]; then
        echo >&2 "fatal: could not find checksum for ${BINARY_FILE} in ${CHECKSUMS_URL}"
        return 1
    fi

    # Download binary with security flags
    if ! curl --proto '=https' --tlsv1.3 -sSfL \
        --retry 3 --retry-delay 5 --max-time 120 \
        -o "${tmp_dir}/${BINARY_FILE}" "${URL}"; then
        echo >&2 "fatal: failed to download golangci-lint from ${URL}"
        return 1
    fi

    # Verify checksum before extraction
    checksum="$(sha256sum "${tmp_dir}/${BINARY_FILE}" | awk '{print $1;}')"
    if [[ "${checksum}" != "${GOLANGCI_SHA256}" ]]; then
        echo >&2 "fatal: ${URL} checksum '${checksum}' differs from expected checksum '${GOLANGCI_SHA256}'"
        return 1
    else
        echo "golangci-lint checksum ${checksum} verified"
    fi

    tar -C "${tmp_dir}" -zxvf "${tmp_dir}/${BINARY_FILE}"
    mkdir -p "${IPAM_DIR}/${bin_dir}"
    mv "${tmp_dir}/${GOLANGCI_LINT}-${GOLANGCI_VERSION}-${KERNEL_OS}-${ARCH}/${GOLANGCI_LINT}" "${IPAM_DIR}/${bin_dir}/"
    chmod 0755 "${IPAM_DIR}/${bin_dir}/${GOLANGCI_LINT}"
}

download_and_install_golangci_lint "$1"
