#!/bin/bash

set -euxo pipefail

REPO_ROOT=$(realpath "$(dirname "$(realpath "${BASH_SOURCE[0]}")")"/..)
cd "${REPO_ROOT}"
export CAPM3PATH="${REPO_ROOT}"
export WORKING_DIR=/opt/metal3-dev-env
FORCE_REPO_UPDATE="${FORCE_REPO_UPDATE:-false}"

export CAPM3RELEASEBRANCH="${CAPM3RELEASEBRANCH:-main}"
export IPAMRELEASEBRANCH="${IPAMRELEASEBRANCH:-main}"

# Extract release version from release-branch name
if [[ "${CAPM3RELEASEBRANCH}" == release-* ]]; then
    CAPM3_RELEASE_PREFIX="${CAPM3RELEASEBRANCH#release-}"
    export CAPM3RELEASE="v${CAPM3_RELEASE_PREFIX}.99"
    export IPAMRELEASE="v${CAPM3_RELEASE_PREFIX}.99"
    export CAPI_RELEASE_PREFIX="v${CAPM3_RELEASE_PREFIX}."
else
    export CAPM3RELEASE="v1.11.99"
    export IPAMRELEASE="v1.11.99"
    export CAPI_RELEASE_PREFIX="v1.10."
fi

# Default CAPI_CONFIG_FOLDER to $HOME/.config folder if XDG_CONFIG_HOME not set
CONFIG_FOLDER="${XDG_CONFIG_HOME:-$HOME/.config}"
export CAPI_CONFIG_FOLDER="${CONFIG_FOLDER}/cluster-api"

# shellcheck source=./scripts/environment.sh
source "${REPO_ROOT}/scripts/environment.sh"

# Clone dev-env repo
sudo mkdir -p ${WORKING_DIR}
sudo chown "${USER}":"${USER}" ${WORKING_DIR}
M3_DEV_ENV_REPO="https://github.com/metal3-io/metal3-dev-env.git"
M3_DEV_ENV_BRANCH=main
M3_DEV_ENV_PATH="${M3_DEV_ENV_PATH:-${WORKING_DIR}/metal3-dev-env}"
clone_repo "${M3_DEV_ENV_REPO}" "${M3_DEV_ENV_BRANCH}" "${M3_DEV_ENV_PATH}"

# Config devenv
cat <<-EOF >"${M3_DEV_ENV_PATH}/config_${USER}.sh"
export CAPI_VERSION=${CAPI_VERSION:-"v1beta1"}
export CAPM3_VERSION=${CAPM3_VERSION:-"v1beta1"}
export NUM_NODES=${NUM_NODES:-"4"}
export KUBERNETES_VERSION=${KUBERNETES_VERSION}
export IMAGE_OS=${IMAGE_OS}
export FORCE_REPO_UPDATE="false"
export USE_IRSO="${USE_IRSO:-false}"
EOF

case "${GINKGO_FOCUS:-}" in
  clusterctl-upgrade|k8s-upgrade|basic|integration)
    # if running basic, integration, k8s upgrade or clusterctl-upgrade test, skip apply bmhs in dev-env
    echo 'export SKIP_APPLY_BMH="true"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
  ;;

  features)
    mkdir -p "${CAPI_CONFIG_FOLDER}"
    echo "ENABLE_BMH_NAME_BASED_PREALLOCATION: true" >"${CAPI_CONFIG_FOLDER}/clusterctl.yaml"
    echo 'export SKIP_APPLY_BMH="true"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
  ;;

  scalability)
    # if running a scalability tests, configure dev-env with fakeIPA
    export NUM_NODES="${NUM_NODES:-100}"
    echo 'export NODES_PLATFORM="fake"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
    echo 'export SKIP_APPLY_BMH="true"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
    sed -i "s/^export NUM_NODES=.*/export NUM_NODES=${NUM_NODES:-100}/" "${M3_DEV_ENV_PATH}/config_${USER}.sh"
    mkdir -p "${CAPI_CONFIG_FOLDER}"
    echo 'CLUSTER_TOPOLOGY: true' >"${CAPI_CONFIG_FOLDER}/clusterctl.yaml"
    echo 'export EPHEMERAL_CLUSTER="minikube"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
  ;;
esac

if [[ ${GINKGO_FOCUS:-} != "scalability" ]]; then
  # Don't run scalability tests if not asked for.
    export GINKGO_SKIP="${GINKGO_SKIP:-} scalability"
fi

# Run make devenv to boot the source cluster
pushd "${M3_DEV_ENV_PATH}" || exit 1
make
popd || exit 1

# Binaries checked below should have been installed by metal3-dev-env make.
# Verify they are available and have correct versions.
PATH=$PATH:/usr/local/go/bin
PATH=$PATH:$(go env GOPATH)/bin

# shellcheck source=./hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=./hack/ensure-kind.sh
source "${REPO_ROOT}/hack/ensure-kind.sh"
# shellcheck source=./hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"
# Ensure kustomize
make kustomize

# shellcheck disable=SC1091,SC1090
source "${M3_DEV_ENV_PATH}/lib/images.sh"
# shellcheck disable=SC1091,SC1090
source "${M3_DEV_ENV_PATH}/lib/releases.sh"
# shellcheck disable=SC1091,SC1090
source "${M3_DEV_ENV_PATH}/lib/ironic_basic_auth.sh"
# shellcheck disable=SC1091,SC1090
source "${M3_DEV_ENV_PATH}/lib/ironic_tls_setup.sh"

# Generate credentials
BMO_OVERLAYS=(
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.8"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.9"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.10"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-latest"
)
IRONIC_OVERLAYS=(
  "${REPO_ROOT}/test/e2e/data/ironic-deployment/overlays/release-26.0"
  "${REPO_ROOT}/test/e2e/data/ironic-deployment/overlays/release-27.0"
  "${REPO_ROOT}/test/e2e/data/ironic-deployment/overlays/release-29.0"
  "${REPO_ROOT}/test/e2e/data/ironic-deployment/overlays/release-latest"
)

# Create usernames and passwords and other files related to ironi basic auth if they
# are missing
if [[ "${IRONIC_BASIC_AUTH}" == "true" ]]; then
  IRONIC_AUTH_DIR="${IRONIC_AUTH_DIR:-${IRONIC_DATA_DIR}/auth}"
  mkdir -p "${IRONIC_AUTH_DIR}"

  # If usernames and passwords are unset, read them from file or generate them
  if [[ -z "${IRONIC_USERNAME:-}" ]]; then
    if [[ ! -f "${IRONIC_AUTH_DIR}/ironic-username" ]]; then
      IRONIC_USERNAME="$(uuid-gen)"
      echo "${IRONIC_USERNAME}" > "${IRONIC_AUTH_DIR}/ironic-username"
    else
      IRONIC_USERNAME="$(cat "${IRONIC_AUTH_DIR}/ironic-username")"
    fi
  fi
  if [[ -z "${IRONIC_PASSWORD:-}" ]]; then
    if [ ! -f "${IRONIC_AUTH_DIR}/ironic-password" ]; then
      IRONIC_PASSWORD="$(uuid-gen)"
      echo "${IRONIC_PASSWORD}" > "${IRONIC_AUTH_DIR}/ironic-password"
    else
      IRONIC_PASSWORD="$(cat "${IRONIC_AUTH_DIR}/ironic-password")"
    fi
  fi
  IRONIC_INSPECTOR_USERNAME="${IRONIC_INSPECTOR_USERNAME:-${IRONIC_USERNAME}}"
  IRONIC_INSPECTOR_PASSWORD="${IRONIC_INSPECTOR_PASSWORD:-${IRONIC_PASSWORD}}"

  export IRONIC_USERNAME
  export IRONIC_PASSWORD
  export IRONIC_INSPECTOR_USERNAME
  export IRONIC_INSPECTOR_PASSWORD
fi

for overlay in "${BMO_OVERLAYS[@]}"; do
  echo "${IRONIC_USERNAME}" > "${overlay}/ironic-username"
  echo "${IRONIC_PASSWORD}" > "${overlay}/ironic-password"
  if [[ "${overlay}" =~ release-0\.[1-5]$ ]]; then
    echo "${IRONIC_INSPECTOR_USERNAME}" > "${overlay}/ironic-inspector-username"
    echo "${IRONIC_INSPECTOR_PASSWORD}" > "${overlay}/ironic-inspector-password"
  fi
done

for overlay in "${IRONIC_OVERLAYS[@]}"; do
  echo "IRONIC_HTPASSWD=$(htpasswd -n -b -B "${IRONIC_USERNAME}" "${IRONIC_PASSWORD}")" > \
    "${overlay}/ironic-htpasswd"
  envsubst < "${REPO_ROOT}/test/e2e/data/ironic-deployment/components/basic-auth/ironic-auth-config-tpl" > \
  "${overlay}/ironic-auth-config"
  IRONIC_INSPECTOR_AUTH_CONFIG_TPL="/tmp/ironic-inspector-auth-config-tpl"
  curl -o "${IRONIC_INSPECTOR_AUTH_CONFIG_TPL}" https://raw.githubusercontent.com/metal3-io/baremetal-operator/release-0.5/ironic-deployment/components/basic-auth/ironic-inspector-auth-config-tpl
  envsubst < "${IRONIC_INSPECTOR_AUTH_CONFIG_TPL}" > \
    "${overlay}/ironic-inspector-auth-config"
  echo "INSPECTOR_HTPASSWD=$(htpasswd -n -b -B "${IRONIC_INSPECTOR_USERNAME}" \
    "${IRONIC_INSPECTOR_PASSWORD}")" > "${overlay}/ironic-inspector-htpasswd"
done

# run e2e tests
if [[ -n "${CLUSTER_TOPOLOGY:-}" ]]; then
  export CLUSTER_TOPOLOGY=true
  make e2e-clusterclass-tests
else
  make e2e-tests
fi
