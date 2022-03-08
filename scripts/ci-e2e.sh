#!/bin/bash

set -o nounset
set -o pipefail
set -o errexit
set -x

REPO_ROOT=$(dirname "$(realpath "${BASH_SOURCE[0]}")")/..
cd "${REPO_ROOT}"

function clone_repo() {
  local REPO_URL="$1"
  local REPO_BRANCH="$2"
  local REPO_PATH="$3"
  if [[ -d "${REPO_PATH}" ]]; then
    rm -rf "${REPO_PATH}"
  fi
  if [ ! -d "${REPO_PATH}" ] ; then
    git clone "${REPO_URL}" "${REPO_PATH}"
    pushd "${REPO_PATH}"
    git checkout "${REPO_BRANCH}"
    git pull -r || true
    popd
  fi
}

function clone_repos() {
  mkdir -p "${M3PATH}"
  pushd "${M3PATH}"
  clone_repo "${BMOREPO}" "${BMOBRANCH}" "${BMOPATH}"
  clone_repo "${IPAMREPO}" "${IPAMBRANCH}" "${IPAMPATH}"
  popd
}

WORKING_DIR=/opt/metal3-dev-env
sudo mkdir -p ${WORKING_DIR}
sudo chown "${USER}":"${USER}" ${WORKING_DIR}

M3_DEV_ENV_REPO="https://github.com/metal3-io/metal3-dev-env.git"
M3_DEV_ENV_BRANCH=main
M3_DEV_ENV_PATH="${WORKING_DIR}"/metal3-dev-env
clone_repo "${M3_DEV_ENV_REPO}" "${M3_DEV_ENV_BRANCH}" "${M3_DEV_ENV_PATH}"

cp "${REPO_ROOT}"/hack/e2e/environment.sh "${M3_DEV_ENV_PATH}/config_${USER}.sh"

pushd ${M3_DEV_ENV_PATH}
# Golang needs to be installed before cloning the needed repos to the Go source directory.
make install_requirements configure_host 
# shellcheck disable=SC1091
source lib/common.sh
clone_repos
# The old path ends with '/..', making cp to copy the content of the directory instead of the whole one.  
REPO_ROOT=$(realpath "$REPO_ROOT") 

# Get correct CAPM3 path when testing locally
export UPGRADE_TEST=${UPGRADE_TEST:-false}
if ! $UPGRADE_TEST; then
  # Copy the current CAPM3 repo to the Go source directory
  rm -rf "${M3PATH}/cluster-api-provider-metal3" # To avoid 'permission denied' error when overriding .git/
  cp -R "${REPO_ROOT}" "${M3PATH}/cluster-api-provider-metal3/"
fi
make launch_mgmt_cluster verify
popd

SSH_PUB_KEY_CONTENT=$(cat "$HOME/.ssh/id_rsa.pub")
export SSH_PUB_KEY_CONTENT

PATH=$PATH:/usr/local/go/bin
PATH=$PATH:$(go env GOPATH)/bin

# Binaries checked below should have been installed by metal3-dev-env make.
# Verify they are available and have correct versions.

# shellcheck source=./hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=./hack/ensure-kind.sh
source "${REPO_ROOT}/hack/ensure-kind.sh"
# shellcheck source=./hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"
# shellcheck source=./hack/ensure-kustomize.sh
source "${REPO_ROOT}/hack/ensure-kustomize.sh"

# This will run the tests with env variabls defined in environment.sh
# or exported by metal3-dev-env scripts
${M3_DEV_ENV_PATH}/scripts/run_command.sh make e2e-tests

pushd ${M3_DEV_ENV_PATH} || exit 1
make clean
popd || exit 1
