#!/bin/bash

set -euxo pipefail

REPO_ROOT=$(realpath "$(dirname "$(realpath "${BASH_SOURCE[0]}")")"/..)
cd "${REPO_ROOT}"
export CAPM3PATH="${REPO_ROOT}"
export WORKING_DIR=/opt/metal3-dev-env
FORCE_REPO_UPDATE="${FORCE_REPO_UPDATE:-false}"

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
    mkdir "${HOME}/.cluster-api/"
    echo "enableBMHNameBasedPreallocation: true" >"${HOME}/.cluster-api/clusterctl.yaml"
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
# shellcheck source=./hack/ensure-kustomize.sh
source "${REPO_ROOT}/hack/ensure-kustomize.sh"

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
export CAPM3_FROM_RELEASE="${CAPM3_FROM_RELEASE:-$(get_latest_release "${CAPM3RELEASEPATH}" "${UPGRADE_FROM_RELEASE}")}"
# Get latest capi patch for the compatible version with capm3 $UPGRADE_FROM_RELEASE
# for example the compatible version with capm3 0.5.x is capi 0.4.y
# If we are not upgrading from v0.5 then the capi compatible has same minor version tag as capm3 e.g v1.1.x
if [[ "${UPGRADE_FROM_RELEASE:-v0.5.}" == "v0.5." ]]; then
    CAPI_UPGRADE_FROM_RELEASE="v0.4."
else
    # If we are not upgrading from v0.5 then the capi compatible has same minor version tag e.g v1.1.x
    CAPI_UPGRADE_FROM_RELEASE=${UPGRADE_FROM_RELEASE}
fi
export CAPI_FROM_RELEASE="${CAPI_FROM_RELEASE:-$(get_latest_release "${CAPIRELEASEPATH}" "${CAPI_UPGRADE_FROM_RELEASE}")}"
# The e2e config file parameter for the capi release compatible with main (capm3 next version)
export CAPI_TO_RELEASE="${CAPIRELEASE}"
# K8s support based on https://cluster-api.sigs.k8s.io/reference/versions.html#core-provider-cluster-api-controller
case ${CAPI_FROM_RELEASE} in
v0.4*)
    export CONTRACT_FROM="v1alpha4"
    export INIT_WITH_KUBERNETES_VERSION="v1.23.8"
    ;;
v1.2*)
    export CONTRACT_FROM="v1beta1"
    export INIT_WITH_KUBERNETES_VERSION="v1.26.4"
    ;;
v1.3*)
    export CONTRACT_FROM="v1beta1"
    export INIT_WITH_KUBERNETES_VERSION="v1.26.4"
    ;;
v1.4*)
    export CONTRACT_FROM="v1beta1"
    export INIT_WITH_KUBERNETES_VERSION="v1.27.1"
    ;;
*)
    echo "UNKNOWN CAPI_FROM_RELEASE !"
    exit 1
    ;;
esac
export CONTRACT_TO="v1beta1"

# image for live iso testing
export LIVE_ISO_IMAGE="https://artifactory.nordix.org/artifactory/metal3/images/iso/minimal_linux_live-v2.iso"

# run e2e tests
make e2e-tests
