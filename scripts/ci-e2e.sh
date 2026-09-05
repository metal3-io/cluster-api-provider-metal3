#!/bin/bash

set -euxo pipefail

REPO_ROOT=$(realpath "$(dirname "$(realpath "${BASH_SOURCE[0]}")")"/..)
cd "${REPO_ROOT}"
export CAPM3PATH="${REPO_ROOT}"

# The e2e lab setup (hack/setup-bml.sh) needs write access to the libvirt socket
# via the 'libvirt' group. Group membership changes (e.g. a prior run adding the
# user to the group) do not apply to an already-running shell. If the user is a
# member in the group database but the membership is not yet effective in this
# process, re-exec the script under `sg` so the group becomes active without
# requiring a logout/login. The CAPM3_LIBVIRT_SG guard prevents infinite re-exec.
if [[ -z "${CAPM3_LIBVIRT_SG:-}" ]]; then
  LIBVIRT_GROUP="libvirt"
  getent group libvirt &>/dev/null || LIBVIRT_GROUP="libvirtd"
  if getent group "${LIBVIRT_GROUP}" 2>/dev/null | awk -F: '{print $4}' | tr ',' '\n' | grep -qx "${USER}" \
     && ! id -nG | tr ' ' '\n' | grep -qx "${LIBVIRT_GROUP}"; then
    echo "Activating '${LIBVIRT_GROUP}' group membership for this session via sg (avoids logout/login)..."
    export CAPM3_LIBVIRT_SG=1
    exec sg "${LIBVIRT_GROUP}" -c "$(printf '%q ' "${BASH_SOURCE[0]}" "$@")"
  fi
fi

export CAPM3RELEASEBRANCH="${CAPM3RELEASEBRANCH:-main}"
export IPAMRELEASEBRANCH="${IPAMRELEASEBRANCH:-main}"

# Extract release version from release-branch name
if [[ "${CAPM3RELEASEBRANCH}" == release-* ]]; then
    CAPM3_RELEASE_PREFIX="${CAPM3RELEASEBRANCH#release-}"
    export CAPM3RELEASE="v${CAPM3_RELEASE_PREFIX}.99"
    export IPAMRELEASE="v${CAPM3_RELEASE_PREFIX}.99"
    export CAPI_RELEASE_PREFIX="v${CAPM3_RELEASE_PREFIX}."
else
    export CAPM3RELEASE="v1.15.99"
    export IPAMRELEASE="v1.15.99"
    export CAPI_RELEASE_PREFIX="v1.14."
fi

# Default CAPI_CONFIG_FOLDER to $HOME/.config folder if XDG_CONFIG_HOME not set
CONFIG_FOLDER="${XDG_CONFIG_HOME:-$HOME/.config}"
export CAPI_CONFIG_FOLDER="${CONFIG_FOLDER}/cluster-api"

# Default to the "basic" scenario when no focus is given, so running ci-e2e.sh
# directly picks a single well-defined test instead of the whole suite.
export GINKGO_FOCUS="${GINKGO_FOCUS:-basic}"

# shellcheck source=./scripts/environment.sh
source "${REPO_ROOT}/scripts/environment.sh"

# Force docker as the container runtime
export CONTAINER_RUNTIME="docker"

# Always set DATE variable for nightly builds because it is needed to form
# the URL for CAPI nightly build components in e2e_conf.yaml even if not used.
DATE=$(date '+%Y%m%d' -d '1 day ago')
export DATE

# If CAPI_NIGHTLY_BUILD is true, it means that the tests are run against the
# nightly build of CAPI components which are built from CAPI's main branch.
if [[ "${CAPI_NIGHTLY_BUILD:-false}" == "true" ]]; then
  export CAPIRELEASE="v1.15.99"
  echo 'export CAPI_NIGHTLY_BUILD="true"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
fi

mkdir -p "${CAPI_CONFIG_FOLDER}"

# Feature-gate variables are exported (not written to clusterctl.yaml): the e2e
# framework builds its own clusterctl config and does not read that file, but
# clusterctl does merge OS environment variables when resolving components.
#
# USE_CLUSTERCLASS_TEMPLATES selects which template set the run builds/uses
# (clusterclass vs normal). It is deliberately separate from CLUSTER_TOPOLOGY,
# which only enables the ClusterTopology feature gate in the CAPI controller:
# most tests enable the gate but still provision from normal templates.
USE_CLUSTERCLASS_TEMPLATES=false
case "${GINKGO_FOCUS:-}" in
  features)
    export ENABLE_BMH_NAME_BASED_PREALLOCATION=true
  ;;

  scalability)
    export CLUSTER_TOPOLOGY=true
  ;;

  in-place-upgrade)
    export CLUSTER_TOPOLOGY=true
    export EXP_RUNTIME_SDK=true
    export EXP_IN_PLACE_UPDATES=true
  ;;
esac

if [[ ${GINKGO_FOCUS:-} != "scalability" ]]; then
  # Don't run scalability tests if not asked for.
    export GINKGO_SKIP="${GINKGO_SKIP:-} scalability"
fi

# Ensure required tools are available.
# shellcheck source=./hack/install-go.sh
source "${REPO_ROOT}/hack/install-go.sh"
hash -r

# Verify Go installation
# shellcheck source=./hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
PATH=$PATH:$(go env GOPATH)/bin
# shellcheck source=./hack/ensure-kind.sh
source "${REPO_ROOT}/hack/ensure-kind.sh"
# shellcheck source=./hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"
# shellcheck source=./hack/ensure-docker.sh
source "${REPO_ROOT}/hack/ensure-docker.sh"

if [[ -z "${CAPM3_DOCKER_SG:-}" ]] && getent group docker &>/dev/null \
   && getent group docker | awk -F: '{print $4}' | tr ',' '\n' | grep -qx "${USER}" \
   && ! id -nG | tr ' ' '\n' | grep -qx docker; then
  echo "Activating 'docker' group membership for this session via sg (avoids logout/login)..."
  export CAPM3_DOCKER_SG=1
  exec sg docker -c "$(printf '%q ' "${BASH_SOURCE[0]}" "$@")"
fi

# Build FKAS image from source so the scalability test uses the latest code.
if [[ "${GINKGO_FOCUS:-}" == "scalability" ]]; then
  FKAS_TAG=ci make docker-build-fkas
fi

# If running in-place-upgrade tests, ensure extension namespace and ssh key secret exist
if [[ "${GINKGO_FOCUS:-}" == "in-place-upgrade" ]]; then
  EXT_NS="test-extension-system"
  kubectl get ns "${EXT_NS}" >/dev/null 2>&1 || kubectl create ns "${EXT_NS}"
  # Recreate the secret to ensure freshest key is used
  kubectl -n "${EXT_NS}" delete secret ssh-key >/dev/null 2>&1 || true
  kubectl -n "${EXT_NS}" create secret generic ssh-key \
    --from-file=id_rsa="${HOME}/.ssh/id_rsa"
fi

# Ensure kustomize + envsubst (tooling used below)
make kustomize envsubst
export PATH="${REPO_ROOT}/hack/tools/bin:${PATH}"

## --- Ironic credentials and TLS setup ---
## These were previously sourced from metal3-dev-env libs.
## Now managed directly here.

# Helper to generate a random UUID-like string for credentials.
uuid_gen() {
  cat /proc/sys/kernel/random/uuid 2>/dev/null || python3 -c "import uuid; print(uuid.uuid4())"
}

export IRONIC_DATA_DIR="${IRONIC_DATA_DIR:-/opt/metal3/ironic}"
export IRONIC_AUTH_DIR="${IRONIC_AUTH_DIR:-${IRONIC_DATA_DIR}/auth}"

sudo mkdir -p "${IRONIC_DATA_DIR}"
sudo chown -R "${USER}:$(id -gn)" "${IRONIC_DATA_DIR}"

export IRONIC_NAMESPACE="${IRONIC_NAMESPACE:-baremetal-operator-system}"
export IRONIC_BASIC_AUTH="${IRONIC_BASIC_AUTH:-true}"
export IRONIC_TLS_SETUP="${IRONIC_TLS_SETUP:-true}"
export IRONIC_KEEPALIVED="${IRONIC_KEEPALIVED:-true}"
export IRONIC_USE_MARIADB="${IRONIC_USE_MARIADB:-false}"
export REGISTRY="${REGISTRY:-172.22.0.1:5000}"
if [[ "${CAPM3RELEASEBRANCH}" == "main" ]]; then
  export BARE_METAL_OPERATOR_IMAGE="${BARE_METAL_OPERATOR_IMAGE:-quay.io/metal3-io/baremetal-operator:main}"
  export IRONIC_IMAGE="${IRONIC_IMAGE:-quay.io/metal3-io/ironic:main}"
  export IPA_DOWNLOADER_IMAGE="${IPA_DOWNLOADER_IMAGE:-registry.nordix.org/quay-io-proxy/metal3-io/ironic-ipa-downloader:latest}"
  export IRONIC_KEEPALIVED_IMAGE="${IRONIC_KEEPALIVED_IMAGE:-registry.nordix.org/quay-io-proxy/metal3-io/keepalived:latest}"
  export IPA_BRANCH="${IPA_BRANCH:-master}"
  export IRSO_IRONIC_VERSION="${IRSO_IRONIC_VERSION:-latest}"
else
  # For future releases, set versions according to the compatibility matrix:
  # https://book.metal3.io/version_support.html
  :
fi

# Provisioning network vars (previously set by metal3-dev-env)
export CLUSTER_PROVISIONING_IP="${CLUSTER_PROVISIONING_IP:-172.22.0.2}"
export CLUSTER_BARE_METAL_PROVISIONER_IP="${CLUSTER_BARE_METAL_PROVISIONER_IP:-172.22.0.2}"
export CLUSTER_DHCP_RANGE_START="${CLUSTER_DHCP_RANGE_START:-172.22.0.10}"
export CLUSTER_DHCP_RANGE_END="${CLUSTER_DHCP_RANGE_END:-172.22.0.100}"
export BARE_METAL_PROVISIONER_NETWORK="${BARE_METAL_PROVISIONER_NETWORK:-172.22.0.0/24}"
export BARE_METAL_PROVISIONER_CIDR="${BARE_METAL_PROVISIONER_CIDR:-24}"
export BARE_METAL_PROVISIONER_INTERFACE="${BARE_METAL_PROVISIONER_INTERFACE:-ironicendpoint}"

update_kustomize_image() {
  local image_name="$1"
  local env_var_name="$2"
  local kustomize_dir="$3"
  local full_image="${!env_var_name}"

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

BMO_OVERLAYS=(
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.12"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.13"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.14"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/pr-test"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-main"
)
IRSO_IRONIC_OVERLAYS=(
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/ironic/overlays/release-33.0"
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/ironic/overlays/release-35.0"
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/ironic/overlays/release-37.0"
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/ironic/overlays/pr-test"
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/ironic/overlays/main"
)
IRSO_OPERATOR_OVERLAYS=(
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/operator/overlays/release-0.8.0"
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/operator/overlays/release-0.9.0"
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/operator/overlays/release-0.10.0"
  "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/operator/overlays/release-0.11.0"
)

# Snapshot the tracked overlay/data directories before mutating them in place
# below (envsubst substitution, generated credentials/certs, kustomize image
# patches)
E2E_DATA_BACKUP_DIR="${REPO_ROOT}/_out/e2e-data-backup"
rm -rf "${E2E_DATA_BACKUP_DIR}"
mkdir -p "${E2E_DATA_BACKUP_DIR}"
cp -a "${REPO_ROOT}/test/e2e/data/bmo-deployment" "${E2E_DATA_BACKUP_DIR}/bmo-deployment"
cp -a "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator" "${E2E_DATA_BACKUP_DIR}/ironic-standalone-operator"

# Update BMO image in overlays
case "${REPO_NAME:-}" in
  baremetal-operator)
    export BARE_METAL_OPERATOR_IMAGE="${REGISTRY}/localimages/tested_repo:latest"
    ;;
  ironic-image)
    export IRONIC_IMAGE="${REGISTRY}/localimages/tested_repo:latest"
    ;;
esac

update_kustomize_image quay.io/metal3-io/baremetal-operator BARE_METAL_OPERATOR_IMAGE "${REPO_ROOT}"/test/e2e/data/bmo-deployment/overlays/pr-test

# Apply envsubst to kustomization.yaml files in BMO and Ironic overlays
yaml_envsubst "${REPO_ROOT}"/test/e2e/data/bmo-deployment/overlays/pr-test
yaml_envsubst "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/base/
yaml_envsubst "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/basic-auth/
yaml_envsubst "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/tls/
yaml_envsubst "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/operator/components/configmap/

for overlay in "${IRSO_IRONIC_OVERLAYS[@]}"; do
  yaml_envsubst "${overlay}"
done

for overlay in "${IRSO_OPERATOR_OVERLAYS[@]}"; do
  yaml_envsubst "${overlay}"
done

# Generate Ironic basic auth credentials
if [[ "${IRONIC_BASIC_AUTH}" == "true" ]]; then
  mkdir -p "${IRONIC_AUTH_DIR}"

  if [[ -z "${IRONIC_USERNAME:-}" ]]; then
    if [[ ! -f "${IRONIC_AUTH_DIR}/ironic-username" ]]; then
      IRONIC_USERNAME="$(uuid_gen)"
      echo "${IRONIC_USERNAME}" > "${IRONIC_AUTH_DIR}/ironic-username"
    else
      IRONIC_USERNAME="$(cat "${IRONIC_AUTH_DIR}/ironic-username")"
    fi
  fi
  if [[ -z "${IRONIC_PASSWORD:-}" ]]; then
    if [[ ! -f "${IRONIC_AUTH_DIR}/ironic-password" ]]; then
      IRONIC_PASSWORD="$(uuid_gen)"
      echo "${IRONIC_PASSWORD}" > "${IRONIC_AUTH_DIR}/ironic-password"
    else
      IRONIC_PASSWORD="$(cat "${IRONIC_AUTH_DIR}/ironic-password")"
    fi
  fi

  export IRONIC_USERNAME
  export IRONIC_PASSWORD

  echo "${IRONIC_USERNAME}" > "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/basic-auth/ironic-username
  echo "${IRONIC_PASSWORD}" > "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/basic-auth/ironic-password
fi

# Generate TLS certificates for Ironic if TLS is enabled
if [[ "${IRONIC_TLS_SETUP}" == "true" ]]; then
  IRONIC_TLS_DIR="${IRONIC_DATA_DIR}/tls"
  mkdir -p "${IRONIC_TLS_DIR}"

  IRONIC_KEY_FILE="${IRONIC_KEY_FILE:-${IRONIC_TLS_DIR}/ironic-key.pem}"
  IRONIC_CERT_FILE="${IRONIC_CERT_FILE:-${IRONIC_TLS_DIR}/ironic-cert.pem}"

  if [[ ! -f "${IRONIC_CERT_FILE}" ]]; then
    openssl req -x509 -newkey rsa:4096 \
      -keyout "${IRONIC_KEY_FILE}" \
      -out "${IRONIC_CERT_FILE}" \
      -days 365 -nodes \
      -subj "/CN=ironic" \
      -addext "subjectAltName=IP:${PROVISIONING_IP:-172.22.0.1},IP:${EXTERNAL_SUBNET_V4_HOST:-192.168.111.1}"
  fi

  cp "${IRONIC_KEY_FILE}" "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/tls/
  cp "${IRONIC_CERT_FILE}" "${REPO_ROOT}"/test/e2e/data/ironic-standalone-operator/ironic/components/tls/
fi

for overlay in "${BMO_OVERLAYS[@]}"; do
  if [[ "${IRONIC_BASIC_AUTH}" == "true" ]]; then
    echo "${IRONIC_USERNAME}" > "${overlay}/ironic-username"
    echo "${IRONIC_PASSWORD}" > "${overlay}/ironic-password"
  fi
done

# Name of the kind management cluster created by the Go test framework.
# Must match managementClusterName in test/e2e/config/e2e_conf.yaml.
export MANAGEMENT_CLUSTER_NAME="${MANAGEMENT_CLUSTER_NAME:-capm3-e2e}"

# Cleanup function.
cleanup() {
  if [[ "${SKIP_CLEANUP:-false}" == "true" ]]; then
     echo "SKIP_CLEANUP=true; preserving the virtual bare metal lab and kind cluster."
     return
  fi
  if [[ -n "${VBMCTL:-}" && -n "${VBMCTL_CONFIG:-}" ]]; then
    echo "Cleaning up virtual bare metal lab..."
    "${VBMCTL}" -c "${VBMCTL_CONFIG}" delete bml || true
  fi
  echo "Deleting kind management cluster '${MANAGEMENT_CLUSTER_NAME}'..."
  kind delete cluster --name "${MANAGEMENT_CLUSTER_NAME}" || true
}
trap cleanup EXIT

## --- Virtual bare metal lab setup via vbmctl ---
## Creates the VMs, networks, BMC emulator and image server used by the tests.
# shellcheck source=./hack/setup-bml.sh disable=SC1091
source "${REPO_ROOT}/hack/setup-bml.sh"

# Delete any leftover kind management cluster from a previous interrupted run so
# the framework can create a fresh one (makes the script idempotent/re-runnable).
echo "Removing any pre-existing kind cluster '${MANAGEMENT_CLUSTER_NAME}'..."
kind delete cluster --name "${MANAGEMENT_CLUSTER_NAME}" || true

# Run e2e tests
# The bootstrap cluster (kind) is created by the Go test framework
# via CAPI's bootstrap package. VMs are already running from vbmctl above.
export E2E_COPY_KUBECONFIG=true
export EXP_MACHINE_TAINT_PROPAGATION=true
# USE_CLUSTERCLASS_TEMPLATES (set per-scenario above) decides the template set;
# CLUSTER_TOPOLOGY only toggles the controller feature gate, so it does not
# select the make target here.
if [[ "${USE_CLUSTERCLASS_TEMPLATES}" == "true" ]]; then
  make e2e-clusterclass-tests
else
  make e2e-tests
fi
