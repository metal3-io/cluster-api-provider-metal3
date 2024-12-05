#!/bin/bash

set -euxo pipefail

REPO_ROOT=$(realpath "$(dirname "$(realpath "${BASH_SOURCE[0]}")")"/..)
cd "${REPO_ROOT}"
export CAPM3PATH="${REPO_ROOT}"
export WORKING_DIR=/opt/metal3-dev-env
FORCE_REPO_UPDATE="${FORCE_REPO_UPDATE:-false}"

export CAPM3RELEASEBRANCH="${CAPM3RELEASEBRANCH:-main}"
CAPIGOPROXY="${CAPIGOPROXY:-https://proxy.golang.org/sigs.k8s.io/cluster-api/@v/list}"
CAPM3GOPROXY="${CAPM3GOPROXY:-https://proxy.golang.org/github.com/metal3-io/cluster-api-provider-metal3/@v/list}"

# Starting from CAPI v1.5.0 version cluster-api config folder location has changed
# to XDG_CONFIG_HOME folder. Following code defines the cluster-api config folder
# location according to CAPM3(since CAPM3 minor versions are aligned to CAPI
# minors versions) release branch

if [[ ${CAPM3RELEASEBRANCH} == "release-1.3" ]]  || [[ ${CAPM3RELEASEBRANCH} == "release-1.4" ]]; then
    export CAPI_CONFIG_FOLDER="${HOME}/.cluster-api"
else
    # Default CAPI_CONFIG_FOLDER to $HOME/.config folder if XDG_CONFIG_HOME not set
    CONFIG_FOLDER="${XDG_CONFIG_HOME:-$HOME/.config}"
    export CAPI_CONFIG_FOLDER="${CONFIG_FOLDER}/cluster-api"
fi

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
EOF
if [[ ${GINKGO_FOCUS:-} == "features" ]]; then
  mkdir -p "$CAPI_CONFIG_FOLDER"
  echo "enableBMHNameBasedPreallocation: true" >"$CAPI_CONFIG_FOLDER/clusterctl.yaml"
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

# Parameterize e2e_config
# By default UPGRADE_FROM_RELEASE is v0.5. if not explicitly exported
export UPGRADE_FROM_RELEASE="${UPGRADE_FROM_RELEASE:-v0.5.}"
# Get latest capm3 patch for a minor version $UPGRADE_FROM_RELEASE
export CAPM3_FROM_RELEASE="${CAPM3_FROM_RELEASE:-$(get_latest_release_from_goproxy "${CAPM3GOPROXY}" "${UPGRADE_FROM_RELEASE}")}"
# Get latest capi patch for the compatible version with capm3 $UPGRADE_FROM_RELEASE
# for example the compatible version with capm3 0.5.x is capi 0.4.y
# If we are not upgrading from v0.5 then the capi compatible has same minor version tag as capm3 e.g v1.1.x
if [[ "${UPGRADE_FROM_RELEASE:-v0.5.}" == "v0.5." ]]; then
    CAPI_UPGRADE_FROM_RELEASE="v0.4."
else
    # If we are not upgrading from v0.5 then the capi compatible has same minor version tag e.g v1.1.x
    CAPI_UPGRADE_FROM_RELEASE=${UPGRADE_FROM_RELEASE}
fi
export CAPI_FROM_RELEASE="${CAPI_FROM_RELEASE:-$(get_latest_release_from_goproxy "${CAPIGOPROXY}" "${CAPI_UPGRADE_FROM_RELEASE}")}"
# The e2e config file parameter for the capi release compatible with main (capm3 next version)
export CAPI_TO_RELEASE="${CAPIRELEASE}"
# K8s support based on https://cluster-api.sigs.k8s.io/reference/versions.html#core-provider-cluster-api-controller
case ${CAPI_FROM_RELEASE} in
v0.4*)
    export CONTRACT_FROM="v1alpha4"
    export INIT_WITH_KUBERNETES_VERSION="v1.23.17"
    ;;
v1.2*)
    export CONTRACT_FROM="v1beta1"
    export INIT_WITH_KUBERNETES_VERSION="v1.27.4"
    ;;
v1.3*)
    export CONTRACT_FROM="v1beta1"
    export INIT_WITH_KUBERNETES_VERSION="v1.27.4"
    ;;
v1.4*)
    export CONTRACT_FROM="v1beta1"
    export INIT_WITH_KUBERNETES_VERSION="v1.28.1"
    ;;
v1.5*)
    export CONTRACT_FROM="v1beta1"
    export INIT_WITH_KUBERNETES_VERSION="v1.28.1"
    ;;
*)
    echo "UNKNOWN CAPI_FROM_RELEASE !"
    exit 1
    ;;
esac
export CONTRACT_TO="v1beta1"

# image for live iso testing
export LIVE_ISO_IMAGE="https://artifactory.nordix.org/artifactory/metal3/images/iso/minimal_linux_live-v2.iso"

# Generate credentials
BMO_OVERLAYS=(
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.5"
)
IRONIC_OVERLAYS=(
  "${REPO_ROOT}/test/e2e/data/ironic-deployment/overlays/release-23.1"
  "${REPO_ROOT}/test/e2e/data/ironic-deployment/overlays/release-24.0"
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
make e2e-tests
