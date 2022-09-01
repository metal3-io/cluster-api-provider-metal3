#!/bin/bash
# File contains e2e var exports

function clone_repo() {
  local REPO_URL="$1"
  local REPO_BRANCH="$2"
  local REPO_PATH="$3"
  if [[ -d "${REPO_PATH}" && "${FORCE_REPO_UPDATE}" == "true" ]]; then
    rm -rf "${REPO_PATH}"
  fi
  if [ ! -d "${REPO_PATH}" ] ; then
    git clone "${REPO_URL}" "${REPO_PATH}"
    pushd "${REPO_PATH}" || exit 1
    git checkout "${REPO_BRANCH}"
    git pull -r || true
    popd || exit 1
  fi
}

function os_check() {
    # Check OS type and version
    # shellcheck disable=SC1091
    source /etc/os-release
    export DISTRO="${ID}${VERSION_ID%.*}"
    export OS="${ID}"
    export OS_VERSION_ID=$VERSION_ID
    export SUPPORTED_DISTROS=(centos8 centos9 rhel8 ubuntu20 ubuntu22)

    if [[ ! "${SUPPORTED_DISTROS[*]}" =~ $DISTRO ]]; then
        echo "Supported OS distros for the host are: CentOS Stream 8/9 or RHEL8/9 or Ubuntu20.04 or Ubuntu22.04"
        exit 1
    fi
}

os_check

if [[ "${OS}" == ubuntu ]]; then
    export IMAGE_OS="ubuntu"
    export CONTAINER_RUNTIME="docker"
else
    export IMAGE_OS="centos"
    export CONTAINER_RUNTIME="podman"
fi

if [ "${CONTAINER_RUNTIME}" == "docker" ]; then
  export EPHEMERAL_CLUSTER="kind"
else
  export EPHEMERAL_CLUSTER="minikube"
fi

# Until the upgrade process starts with CAPI 0.4.x
# CAPM 0.5.x and K8s 1.23.x it is not possible to set
# the starting version of the target cluster (FROM_K8S_VERSION)
# higher than 1.23.x
export FROM_K8S_VERSION="v1.23.8"
export KUBERNETES_VERSION=${FROM_K8S_VERSION}
export UPGRADED_K8S_VERSION="v1.24.1"
# Can be overriden from jjbs
export CAPI_VERSION=${CAPI_VERSION:-"v1beta1"}
export CAPM3_VERSION=${CAPM3_VERSION:-"v1beta1"}
export M3PATH=${M3PATH:-"${HOME}/go/src/github.com/metal3-io"}
export CAPM3_LOCAL_IMAGE="${CAPM3PATH}"

# Upgrade test environment vars and config
if [[ ${GINKGO_FOCUS:-} == "upgrade" ]]; then
    export CAPI_VERSION="v1alpha4"
    export CAPM3_VERSION="v1alpha5"
    # Ironic and BMO images to start from. They will then upgrade to main/latest
    export IRONIC_TAG="capm3-v0.5.5"
    export BAREMETAL_OPERATOR_TAG="capm3-v0.5.5"
fi
# Override project infra vars that point to
# the current branch to build capm3 image and crds
if [[ "${CAPM3_VERSION}" == "v1alpha5" ]]; then
    export CAPM3BRANCH="release-0.5"
    # This var is set in project infra to use the current repo location for
    # building CAPM3 image while upgrade needs an old version
    unset CAPM3_LOCAL_IMAGE
    export CAPM3PATH="${M3PATH}/cluster-api-provider-metal3"
    export CAPM3RELEASEBRANCH="${CAPM3RELEASEBRANCH:-release-0.5}"
fi
if [[ "${IMAGE_OS}" == "ubuntu" ]]; then
  export UPGRADED_IMAGE_NAME="UBUNTU_20.04_NODE_IMAGE_K8S_${UPGRADED_K8S_VERSION}.qcow2"
  export UPGRADED_RAW_IMAGE_NAME="UBUNTU_20.04_NODE_IMAGE_K8S_${UPGRADED_K8S_VERSION}-raw.img"
else
  export UPGRADED_IMAGE_NAME="CENTOS_9_NODE_IMAGE_K8S_${UPGRADED_K8S_VERSION}.qcow2"
  export UPGRADED_RAW_IMAGE_NAME="CENTOS_9_NODE_IMAGE_K8S_${UPGRADED_K8S_VERSION}-raw.img"
fi

# Exported to the cluster templates
# Generate user ssh key
if [ ! -f "${HOME}/.ssh/id_rsa" ]; then
  ssh-keygen -f "${HOME}/.ssh/id_rsa" -P ""
fi
SSH_PUB_KEY_CONTENT=$(cat "$HOME/.ssh/id_rsa.pub")
export SSH_PUB_KEY_CONTENT
# The host that has images for provisioning, this should be in the
# format of a URL host, e.g. with IPv6, it should be surrounded
# by brackets
export PROVISIONING_URL_HOST="172.22.0.1"
export CLUSTER_PROVISIONING_IP="172.22.0.2"
export CLUSTER_URL_HOST="$CLUSTER_PROVISIONING_IP"

# Ironic config vars needed in ironic_tls_setup.sh and ironic_basic_auth.sh
export IRONIC_DATA_DIR="$WORKING_DIR/ironic"
export IRONIC_TLS_SETUP="true"
export IRONIC_BASIC_AUTH="true"

# supported providerID formats
if [[ $CAPM3_VERSION == "v1alpha5" ]]; then
  export PROVIDER_ID_FORMAT="metal3://{{ ds.meta_data.uuid }}"
else
  export PROVIDER_ID_FORMAT="metal3://{{ ds.meta_data.providerid }}"
fi
