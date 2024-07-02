#!/bin/bash

set -euxo pipefail
#!/bin/bash

set -euxo pipefail

REPO_ROOT=$(realpath "$(dirname "$(realpath "${BASH_SOURCE[0]}")")"/..)
cd "${REPO_ROOT}"
export CAPM3PATH="${REPO_ROOT}"
export WORKING_DIR=/opt/metal3-dev-env
FORCE_REPO_UPDATE="${FORCE_REPO_UPDATE:-false}"

export CAPM3RELEASEBRANCH="${CAPM3RELEASEBRANCH:-main}"

# Starting from CAPI v1.5.0 version cluster-api config folder location has changed
# to XDG_CONFIG_HOME folder. Following code defines the cluster-api config folder
# location according to CAPM3(since CAPM3 minor versions are aligned to CAPI
# minors versions) release branch

if [[ ${CAPM3RELEASEBRANCH} == "release-1.4" ]]; then
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

if [[ ${GINKGO_FOCUS:-} == "remediation" ]]; then
    sleep 120
    exit 1
fi
