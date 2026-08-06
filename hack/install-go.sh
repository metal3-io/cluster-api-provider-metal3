#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Install Go if not found in PATH
install_go() {
    # Check if go is already available
    if command -v go >/dev/null 2>&1; then
        echo "Go is already installed: $(go version)"
        return 0
    fi

    local go_version="${GO_VERSION:-1.26.4}"
    local os
    local arch
    
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)
    
    # Normalize architecture names
    case "${arch}" in
        x86_64)
            arch="amd64"
            ;;
        aarch64)
            arch="arm64"
            ;;
    esac

    local install_dir="/usr/local/go"
    
    # Check if we have write access to /usr/local
    if [[ ! -w /usr/local ]]; then
        # Fall back to user local directory
        install_dir="${HOME}/.local/go"
    fi

    if [[ ! -d "${install_dir}" ]]; then
        mkdir -p "${install_dir}"
    fi

    local go_tar="go${go_version}.${os}-${arch}.tar.gz"
    local download_url="https://dl.google.com/go/${go_tar}"

    echo "Installing Go ${go_version} from ${download_url}..."
    
    # Download and extract Go
    local tmp_dir
    tmp_dir=$(mktemp -d)

    # shellcheck disable=SC2064 # Intentional: expand tmp_dir now since it's local
    trap "rm -rf '${tmp_dir}'" RETURN

    if ! curl -sL "${download_url}" -o "${tmp_dir}/${go_tar}"; then
        echo "Failed to download Go from ${download_url}"
        return 1
    fi

    # Remove existing installation if present
    if [[ -d "${install_dir}/bin" ]]; then
        rm -rf "${install_dir:?}/bin" "${install_dir:?}/lib" "${install_dir:?}/pkg"
    fi

    # Extract to parent directory and move contents
    tar -C "${tmp_dir}" -xzf "${tmp_dir}/${go_tar}"
    cp -r "${tmp_dir}/go/"* "${install_dir}/"

    # Add to PATH if not already there
    if [[ ":${PATH}:" != *":${install_dir}/bin:"* ]]; then
        export PATH="${install_dir}/bin:${PATH}"
    fi

    echo "Go installed successfully: $("${install_dir}/bin/go" version)"
}

install_go
