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
export CAPI_VERSION=${CAPI_VERSION:-"v1beta1"}
export CAPM3_VERSION=${CAPM3_VERSION:-"v1beta1"}
export NUM_NODES=${NUM_NODES:-"4"}

# needed for variable substitution in templates
export IMAGE_CHECKSUM_TYPE="md5"

# shellcheck disable=SC2016
export CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-3}
# shellcheck disable=SC2016
export WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-1}

# These two variables below are required to render the cluster template dynamically from the metal3-dev-env.
# This template is used by default, but users can change to use the static templates by changing the flavor.
export NUM_OF_CONTROLPLANE_REPLICAS=${CONTROL_PLANE_MACHINE_COUNT}
export NUM_OF_WORKER_REPLICAS=${WORKER_MACHINE_COUNT}

# The e2e test framework would itself handle the cloning. It clones all the repos that are cloned in M3-DEV-ENV expect CAPM3.
# It would use the local CAPM3 repo where the e2e test is running.
# Set this variable to false to avoid the dev-env to override what the test framework cloned.
export FORCE_REPO_UPDATE="false"

os_check

if [[ "${OS}" == ubuntu ]]; then
    export IMAGE_OS=${IMAGE_OS:-"ubuntu"}
else
    export IMAGE_OS=${IMAGE_OS:-"centos"}
fi
M3_DEV_ENV_PATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# shellcheck disable=SC1091
# shellcheck disable=SC1090
source "${M3_DEV_ENV_PATH}/scripts/feature_tests/feature_test_vars.sh"
# shellcheck disable=SC1091
# shellcheck disable=SC1090
source "${M3_DEV_ENV_PATH}/scripts/feature_tests/node_reuse/node_reuse_vars.sh"

# Pin Calico version
export CALICO_PATCH_RELEASE="v3.21.0"
export CALICO_MINOR_RELEASE="v3.21"
