#!/bin/bash

set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "$(realpath "${BASH_SOURCE[0]}")")/..
cd "${REPO_ROOT}" || exit 1

function clone_repo() {
  local REPO_URL="$1"
  local REPO_BRANCH="$2"
  local REPO_PATH="$3"
  if [[ -d "${REPO_PATH}" ]]; then
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

WORKING_DIR=/opt/metal3-dev-env
sudo mkdir -p ${WORKING_DIR}
sudo chown "${USER}":"${USER}" ${WORKING_DIR}

M3_DEV_ENV_REPO="https://github.com/metal3-io/metal3-dev-env.git"
M3_DEV_ENV_BRANCH=master
M3_DEV_ENV_PATH="${WORKING_DIR}"/metal3-dev-env
clone_repo "${M3_DEV_ENV_REPO}" "${M3_DEV_ENV_BRANCH}" "${M3_DEV_ENV_PATH}"

cp "${REPO_ROOT}"/hack/e2e/config_ubuntu.sh ${M3_DEV_ENV_PATH}

pushd ${M3_DEV_ENV_PATH} || exit 1
make || exit 1
popd || exit 1

SSH_PUB_KEY_CONTENT=$(cat "$HOME/.ssh/id_rsa.pub")
export SSH_PUB_KEY_CONTENT

PATH=$PATH:/usr/local/go/bin
go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega/...

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

make e2e-tests

pushd ${M3_DEV_ENV_PATH} || exit 1
make clean
popd || exit 1
