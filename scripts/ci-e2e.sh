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
    export CAPM3RELEASE="v1.13.99"
    export IPAMRELEASE="v1.13.99"
    # Commenting this out as CAPI release prefix and exporting CAPIRELEASE
    # during pre-release phase of CAPI.
    # We will change when minor is released.
    export CAPI_RELEASE_PREFIX="v1.12."
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
# SKIP_NODE_IMAGE_PREPULL is set to true to avoid dev-env downloading the node
# image
cat <<-EOF >"${M3_DEV_ENV_PATH}/config_${USER}.sh"
export CAPI_VERSION="v1beta2"
export CAPM3_VERSION=${CAPM3_VERSION:-"v1beta1"}
export NUM_NODES=${NUM_NODES:-"4"}
export KUBERNETES_VERSION=${KUBERNETES_VERSION}
export IMAGE_OS=${IMAGE_OS}
export FORCE_REPO_UPDATE="false"
export SKIP_NODE_IMAGE_PREPULL="true"
export IPA_BASEURI=https://artifactory.nordix.org/artifactory/openstack-remote-cache/ironic-python-agent/dib
EOF

# Set USE_IRSO only when IMAGE_OS is not ubuntu
if [[ "${IMAGE_OS}" != "ubuntu" ]]; then
  echo 'export USE_IRSO="true"' >> "${M3_DEV_ENV_PATH}/config_${USER}.sh"
fi

# Always set DATE variable for nightly builds because it is needed to form
# the URL for CAPI nightly build components in e2e_conf.yaml even if not used.
DATE=$(date '+%Y%m%d' -d '1 day ago')
export DATE

# If CAPI_NIGHTLY_BUILD is true, it means that the tests are run against the
# nightly build of CAPI components which are built from CAPI's main branch.
if [[ "${CAPI_NIGHTLY_BUILD:-false}" == "true" ]]; then
  export CAPIRELEASE="v1.13.99"
  echo 'export CAPI_NIGHTLY_BUILD="true"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
fi

mkdir -p "${CAPI_CONFIG_FOLDER}"

case "${GINKGO_FOCUS:-}" in
  clusterctl-upgrade|k8s-upgrade|k8s-upgrade-n3|basic|integration|remediation|k8s-conformance|capi-md-tests)
    # if running basic, integration, k8s upgrade, k8s n+3 upgrade, clusterctl-upgrade, remediation, k8s conformance or capi-md tests, skip apply bmhs in dev-env
    echo 'export SKIP_APPLY_BMH="true"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
  ;;

  features)
    echo "ENABLE_BMH_NAME_BASED_PREALLOCATION: true" >"${CAPI_CONFIG_FOLDER}/clusterctl.yaml"
    echo 'export SKIP_APPLY_BMH="true"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
  ;;

  scalability)
    # if running a scalability tests, configure dev-env with fakeIPA
    export NUM_NODES="${NUM_NODES:-50}"
    echo 'export NODES_PLATFORM="fake"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
    echo 'export SKIP_APPLY_BMH="true"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
    sed -i "s/^export NUM_NODES=.*/export NUM_NODES=${NUM_NODES:-50}/" "${M3_DEV_ENV_PATH}/config_${USER}.sh"
    echo 'CLUSTER_TOPOLOGY: true' >>"${CAPI_CONFIG_FOLDER}/clusterctl.yaml"
    echo 'export BOOTSTRAP_CLUSTER="minikube"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
  ;;

  in-place-upgrade)
    # Enable Cluster Topology and in-place updates features
    echo 'CLUSTER_TOPOLOGY: true' >>"${CAPI_CONFIG_FOLDER}/clusterctl.yaml"
    echo 'EXP_RUNTIME_SDK: true' >>"${CAPI_CONFIG_FOLDER}/clusterctl.yaml"
    echo 'EXP_IN_PLACE_UPDATES: true' >>"${CAPI_CONFIG_FOLDER}/clusterctl.yaml"
    echo 'export SKIP_APPLY_BMH="true"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
  ;;
esac

echo 'EXP_MACHINE_TAINT_PROPAGATION: true' >> "${CAPI_CONFIG_FOLDER}/clusterctl.yaml"

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

# If running in-place-upgrade tests, ensure extension namespace and ssh key secret exist
if [[ "${GINKGO_FOCUS:-}" == "in-place-upgrade" ]]; then
  EXT_NS="test-extension-system"
  kubectl get ns "${EXT_NS}" >/dev/null 2>&1 || kubectl create ns "${EXT_NS}"
  # Recreate the secret to ensure freshest key is used
  kubectl -n "${EXT_NS}" delete secret ssh-key >/dev/null 2>&1 || true
  kubectl -n "${EXT_NS}" create secret generic ssh-key \
    --from-file=id_rsa="${HOME}/.ssh/id_rsa"
fi
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
# shellcheck disable=SC1091,SC1090
source "${M3_DEV_ENV_PATH}/lib/common.sh"
# shellcheck disable=SC1091,SC1090
source "${M3_DEV_ENV_PATH}/lib/network.sh"

update_kustomize_image() {
  local image_name="$1"      # e.g., quay.io/metal3-io/ironic
  local env_var_name="$2"    # e.g., IRONIC_IMAGE
  local kustomize_dir="$3"   # e.g., ./overlays/dev

  local full_image="${!env_var_name}"  # Resolve the env var value

  if [[ -z "${full_image}" ]]; then
    echo "Environment variable ${env_var_name} is not set."
    return 1
  fi

  if [[ ! -f "${kustomize_dir}/kustomization.yaml" ]]; then
    echo "No kustomization.yaml found in ${kustomize_dir}"
    return 1
  fi

  echo "Updating image for ${image_name} to ${full_image} in ${kustomize_dir}/kustomization.yaml"
  (cd "${kustomize_dir}" && kustomize edit set image "${image_name}=${full_image}")
}

yaml_envsubst() {
  local dir="$1"

  for file in "${dir}"/*.yaml; do
    if [[ -f "${file}" ]]; then
      local tmp_file
      tmp_file=$(mktemp)
      envsubst < "${file}" > "${tmp_file}" && mv "${tmp_file}" "${file}"
    fi
  done
}

# Generate credentials
BMO_OVERLAYS=(
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.10"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.11"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.12"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/pr-test"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-main"
)
IRSO_IRONIC_OVERLAYS=(
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/ironic/overlays/release-31.0"
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/ironic/overlays/release-32.0"
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/ironic/overlays/release-33.0"
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/ironic/overlays/pr-test"
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/ironic/overlays/main"
)

# Update BMO and Ironic images in kustomization.yaml files to use the same image that was used before pivot in the metal3-dev-env
case "${REPO_NAME:-}" in
  baremetal-operator)
    # shellcheck disable=SC2034
    export BARE_METAL_OPERATOR_IMAGE="${REGISTRY}/localimages/tested_repo:latest"
    ;;

  ironic-image)
    # shellcheck disable=SC2034
    export IRONIC_IMAGE="${REGISTRY}/localimages/tested_repo:latest"
    ;;
esac

export IRONIC_IMAGE="${IRONIC_IMAGE:-quay.io/metal3-io/ironic:main}"
update_kustomize_image quay.io/metal3-io/baremetal-operator BARE_METAL_OPERATOR_IMAGE "${REPO_ROOT}"/test/e2e/data/bmo-deployment/overlays/pr-test

# Apply envsubst to kustomization.yaml files in BMO and Ironic overlays
yaml_envsubst "${REPO_ROOT}"/test/e2e/data/bmo-deployment/overlays/pr-test
yaml_envsubst "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/base/
yaml_envsubst "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/basic-auth/
yaml_envsubst "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/tls/
yaml_envsubst "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/operator

for overlay in "${IRSO_IRONIC_OVERLAYS[@]}"; do
  yaml_envsubst "${overlay}"
done

# Create usernames and passwords and other files related to ironic basic auth if they
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

  export IRONIC_USERNAME
  export IRONIC_PASSWORD
fi

if [[ "${IRONIC_BASIC_AUTH}" == "true" ]]; then
  echo "${IRONIC_USERNAME}" > "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/basic-auth/ironic-username
  echo "${IRONIC_PASSWORD}" > "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/basic-auth/ironic-password
fi

if [[ "${IRONIC_TLS_SETUP}" == "true" ]]; then
cp "${IRONIC_KEY_FILE}" "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/tls/
cp "${IRONIC_CERT_FILE}" "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/tls/
fi

for overlay in "${BMO_OVERLAYS[@]}"; do
  echo "${IRONIC_USERNAME}" > "${overlay}/ironic-username"
  echo "${IRONIC_PASSWORD}" > "${overlay}/ironic-password"
  if [[ "${overlay}" =~ release-0\.[1-5]$ ]]; then
    echo "${IRONIC_INSPECTOR_USERNAME}" > "${overlay}/ironic-inspector-username"
    echo "${IRONIC_INSPECTOR_PASSWORD}" > "${overlay}/ironic-inspector-password"
  fi
done

# run e2e tests
if [[ -n "${CLUSTER_TOPOLOGY:-}" ]]; then
  export CLUSTER_TOPOLOGY=true
  make e2e-clusterclass-tests
else
  export EXP_MACHINE_TAINT_PROPAGATION=true
  make e2e-tests
fi
