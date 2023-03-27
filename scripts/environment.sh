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

export FROM_K8S_VERSION="v1.25.2"
export KUBERNETES_VERSION="v1.26.0"
export PATCH_KUBERNETES_VERSION="v1.26.1"

# Can be overriden from jjbs
export CAPI_VERSION=${CAPI_VERSION:-"v1beta1"}
export CAPM3_VERSION=${CAPM3_VERSION:-"v1beta1"}
export M3PATH=${M3PATH:-"${HOME}/go/src/github.com/metal3-io"}
export CAPM3_LOCAL_IMAGE="${CAPM3PATH}"

export PATH=$PATH:$HOME/.krew/bin

# Upgrade test environment vars and config
if [[ ${GINKGO_FOCUS:-} == "upgrade" ]]; then
  export NUM_NODES=${NUM_NODES:-"5"}
fi

# Integration test environment vars and config
if [[ ${GINKGO_FOCUS:-} == "integration" ]]; then
  export NUM_NODES=${NUM_NODES:-"2"}
  export CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-"1"}
  export WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-"1"}
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

export BARE_METAL_RPOVISIONER_URL_HOST="172.22.0.1"
export CLUSTER_BARE_METAL_PROVISIONER_IP="172.22.0.2"
export CLUSTER_BM_PROVISIONER_HOST="$CLUSTER_PROVISIONING_IP"

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

# Enable the ClusterResourceSet feature flag
export EXP_CLUSTER_RESOURCE_SET="true"
