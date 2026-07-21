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

GOPATH_BIN="$(go env GOPATH)/bin/"
MINIMUM_KUBECTL_VERSION=${KUBERNETES_VERSION:-"v1.36.2"}

# Download the required kubectl version into the given target directory
download_kubectl()
{
    local target_dir="$1"
    local tmp
    tmp="$(mktemp)"
    echo "Installing kubectl ${MINIMUM_KUBECTL_VERSION} to ${target_dir}"
    curl -fsSLo "${tmp}" "https://dl.k8s.io/release/${MINIMUM_KUBECTL_VERSION}/bin/linux/amd64/kubectl"
    chmod +x "${tmp}"
    # Install to the target dir, using sudo if the location is not writable
    if [[ -w "${target_dir}" ]]; then
        mv -f "${tmp}" "${target_dir}/kubectl"
    elif command -v sudo &>/dev/null; then
        sudo mv -f "${tmp}" "${target_dir}/kubectl"
    else
        rm -f "${tmp}"
        echo "ERROR: ${target_dir} is not writable and sudo is unavailable."
        return 2
    fi
}

# Install the required kubectl version, replacing the currently active binary
# on PATH when present so the upgrade actually takes effect.
install_kubectl()
{
    if [[ "${OSTYPE}" != "linux-gnu"* ]]; then
        echo "Automatic kubectl installation is only supported on linux-gnu"
        echo "Please install ${MINIMUM_KUBECTL_VERSION} or later manually."
        return 2
    fi

    # Prefer replacing the kubectl currently resolved on PATH, otherwise fall
    # back to GOPATH/bin.
    local active_kubectl target_dir
    active_kubectl="$(command -v kubectl 2>/dev/null || true)"
    if [[ -n "${active_kubectl}" ]]; then
        target_dir="$(cd "$(dirname "${active_kubectl}")" && pwd)"
    else
        target_dir="${GOPATH_BIN%/}"
        if ! [ -d "${target_dir}" ]; then
            mkdir -p "${target_dir}"
        fi
    fi

    download_kubectl "${target_dir}" || return $?

    if [[ ":${PATH}:" != *":${target_dir}:"* ]]; then
        echo "WARNING: ${target_dir} is not on your PATH; the newly installed kubectl may not take precedence."
        echo "         Add it to your PATH, e.g.: export PATH=\"${target_dir}:\${PATH}\""
    fi
}

# Return the detected kubectl client version, or empty if it can't be determined
get_kubectl_version()
{
    local version
    version="$(kubectl version --client -o yaml 2>/dev/null | grep -F gitVersion | awk '{print $2}')" || true
    if [[ -z "${version}" ]]; then
        # Fallback: try parsing non-yaml output (e.g. "Client Version: v1.36.0")
        version="$(kubectl version --client 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+')" || true
    fi
    echo "${version}"
}

# Ensure the kubectl tool exists and is a viable version, or installs it
verify_kubectl_version()
{
    # Remove any broken kubectl binary in GOPATH/bin
    if [[ -f "${GOPATH_BIN}/kubectl" ]] && ! "${GOPATH_BIN}/kubectl" version --client &>/dev/null; then
        echo "Removing broken kubectl binary at ${GOPATH_BIN}/kubectl"
        rm -f "${GOPATH_BIN}/kubectl"
    fi

    # If kubectl is not available on the path, or not a working binary, get it
    if ! kubectl version --client &>/dev/null; then
        echo 'kubectl not found or not working, installing'
        install_kubectl || return $?
        hash -r
    fi

    local kubectl_version
    kubectl_version="$(get_kubectl_version)"
    if [[ -z "${kubectl_version}" ]]; then
        echo "WARNING: Unable to determine kubectl version, skipping version check"
        return 0
    fi

    # If the detected version is older than the minimum, install the required version
    if [[ "${MINIMUM_KUBECTL_VERSION}" != $(echo -e "${MINIMUM_KUBECTL_VERSION}\n${kubectl_version}" | sort -s -t. -k 1,1 -k 2,2n -k 3,3n | head -n1) ]]; then
        cat << EOF
Detected kubectl version: ${kubectl_version}.
Requires ${MINIMUM_KUBECTL_VERSION} or greater.
EOF
        install_kubectl || return $?
        hash -r

        # Re-check the version after installation
        kubectl_version="$(get_kubectl_version)"
        if [[ -n "${kubectl_version}" && "${MINIMUM_KUBECTL_VERSION}" != $(echo -e "${MINIMUM_KUBECTL_VERSION}\n${kubectl_version}" | sort -s -t. -k 1,1 -k 2,2n -k 3,3n | head -n1) ]]; then
            cat << EOF
Detected kubectl version after install: ${kubectl_version}.
Requires ${MINIMUM_KUBECTL_VERSION} or greater.
Please ensure the installed kubectl precedes other kubectl locations on your PATH.
EOF
            return 2
        fi
    fi
}

verify_kubectl_version
