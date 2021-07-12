#!/bin/bash
# File used as metal3-dev-env config and as a source for variables used by envsubst


function os_check() {
    # Check OS type and version
    # shellcheck disable=SC1091
    source /etc/os-release
    export DISTRO="${ID}${VERSION_ID%.*}"
    export OS="${ID}"
    export OS_VERSION_ID=$VERSION_ID
    export SUPPORTED_DISTROS=(centos8 rhel8 ubuntu20)

    if [[ ! "${SUPPORTED_DISTROS[*]}" =~ $DISTRO ]]; then
        echo "Supported OS distros for the host are: CentOS Stream 8 or RHEL8 or Ubuntu20.04"
        exit 1
    fi
}

# metal3-dev-env customization
export CAPI_VERSION=${CAPI_VERSION:-"v1alpha3"}
export CAPM3_VERSION=${CAPM3_VERSION:-"v1alpha4"}
export NUM_NODES="4"

# needed for variable substitution in templates
export IMAGE_CHECKSUM_TYPE="md5"

# preserve the template, to be substituted based on value defined in e2e_test.go
# shellcheck disable=SC2016
export CONTROL_PLANE_MACHINE_COUNT='${CONTROL_PLANE_MACHINE_COUNT}'
# shellcheck disable=SC2016
export WORKER_MACHINE_COUNT='${WORKER_MACHINE_COUNT}'

os_check

if [[ "${OS}" == ubuntu ]]; then
    export IMAGE_OS=${IMAGE_OS:-"Ubuntu"}
else
    export IMAGE_OS=${IMAGE_OS:-"Centos"}
fi
