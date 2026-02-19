#!/usr/bin/env bash

set -eux

MINIMUM_GO_VERSION=go1.24.0
# Ensure the go tool exists and is a viable version, or installs it
verify_go_version()
{
    # If go is not available on the path, get it
    if ! [ -x "$(command -v go)" ]; then
        if [[ "${OSTYPE}" == "linux-gnu" ]]; then
            echo 'go not found, installing'
            curl -sLo "/tmp/${MINIMUM_GO_VERSION}.linux-amd64.tar.gz" "https://go.dev/dl/${MINIMUM_GO_VERSION}.linux-amd64.tar.gz"
            sudo tar -C /usr/local -xzf "/tmp/${MINIMUM_GO_VERSION}.linux-amd64.tar.gz"
            export PATH=/usr/local/go/bin:$PATH
        else
            echo "Missing required binary in path: go"
            return 2
        fi
    fi

    local go_version
    IFS=" " read -ra go_version <<< "$(go version)"
    local minimum_go_version
    minimum_go_version=go1.24
    if [[ "${minimum_go_version}" != $(echo -e "${minimum_go_version}\n${go_version[2]}" | sort -s -t. -k 1,1 -k 2,2n -k 3,3n | head -n1) ]] && [[ "${go_version[2]}" != "devel" ]]; then
        cat << EOF
Detected go version: ${go_version[*]}.
Requires ${minimum_go_version} or greater.
Please install ${minimum_go_version} or later.
EOF
        return 2
    fi
}

verify_go_version

# Explicitly opt into go modules, even though we're inside a GOPATH directory
export GO111MODULE=on
