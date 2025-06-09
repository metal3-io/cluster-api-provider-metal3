#!/bin/bash
# File contains e2e var exports

function clone_repo() {
  local REPO_URL="$1"
  local REPO_BRANCH="$2"
  local REPO_PATH="$3"
  if [[ -d "${REPO_PATH}" && "${FORCE_REPO_UPDATE}" == "true" ]]; then
    rm -rf "${REPO_PATH}"
  fi
  if [ ! -d "${REPO_PATH}" ]; then
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

export FROM_K8S_VERSION=${FROM_K8S_VERSION:-"v1.32.1"}
export KUBERNETES_VERSION=${KUBERNETES_VERSION:-"v1.33.0"}

# Can be overriden from jjbs
export CAPI_VERSION=${CAPI_VERSION:-"v1beta1"}
export CAPM3_VERSION=${CAPM3_VERSION:-"v1beta1"}
export M3PATH=${M3PATH:-"${HOME}/go/src/github.com/metal3-io"}
export CAPM3_LOCAL_IMAGE="${CAPM3PATH}"

# Upgrade test environment vars and config
case "${GINKGO_FOCUS:-}" in
  # clusterctl upgrade var
  clusterctl-upgrade)
    export NUM_NODES="5"
  ;;

  # Integration test environment vars and config
  basic|integration)
    export NUM_NODES=${NUM_NODES:-"2"}
    export CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-"1"}
    export WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-"1"}
  ;;

  # Pivoting, k8s-upgrade and remediation vars and config
  pivoting|k8s-upgrade|remediation)
    export NUM_NODES="4"
    export CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-"3"}
    export WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-"1"}
  ;;

  # Scalability test environment vars and config
  scalability)
    export NUM_NODES=${NUM_NODES:-"10"}
    export BMH_BATCH_SIZE=${BMH_BATCH_SIZE:-"2"}
    export CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-"1"}
    export WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-"0"}
    export KUBERNETES_VERSION_UPGRADE_FROM=${FROM_K8S_VERSION}
  ;;
esac

# IPReuse feature test environment vars and config
if [[ ${GINKGO_FOCUS:-} == "features" && ${GINKGO_SKIP:-} == "pivoting remediation" ]]; then
  export NUM_NODES="5"
  export CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-"3"}
  export WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-"2"}
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

export BARE_METAL_PROVISIONER_URL_HOST="172.22.0.1"
export CLUSTER_BARE_METAL_PROVISIONER_IP="172.22.0.2"
export CLUSTER_BARE_METAL_PROVISIONER_HOST="$CLUSTER_PROVISIONING_IP"

# Ironic config vars needed in ironic_tls_setup.sh and ironic_basic_auth.sh
export IRONIC_DATA_DIR="$WORKING_DIR/ironic"
export IRONIC_TLS_SETUP="true"
export IRONIC_BASIC_AUTH="true"

# supported providerID formats
export PROVIDER_ID_FORMAT="metal3://{{ ds.meta_data.providerid }}"

# Enable the ClusterResourceSet feature flag
export EXP_CLUSTER_RESOURCE_SET="true"
