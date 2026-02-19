#!/usr/bin/env bash

set -euxo pipefail

REPO_ROOT=$(realpath "$(dirname "$(realpath "${BASH_SOURCE[0]}")")"/..)
cd "${REPO_ROOT}" || exit 1
export CAPM3PATH="${REPO_ROOT}"
export WORKING_DIR=/tmp/metal3-dev-env
FORCE_REPO_UPDATE="${FORCE_REPO_UPDATE:-false}"

export CAPM3RELEASEBRANCH="${CAPM3RELEASEBRANCH:-main}"
export IPAMRELEASEBRANCH="${IPAMRELEASEBRANCH:-main}"

# Verify they are available and have correct versions.
PATH=$PATH:/usr/local/go/bin
PATH=$PATH:$(go env GOPATH)/bin

"${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=./hack/ensure-kind.sh
source "${REPO_ROOT}/hack/ensure-kind.sh"
# shellcheck source=./hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure_kubectl.sh"
# shellcheck source=./hack/e2e/fake-ipa.sh
source "${REPO_ROOT}/hack/e2e/fake-ipa.sh"

"${REPO_ROOT}/hack/ensure_yq.sh"
# Ensure kustomize
make kustomize
sudo install "${REPO_ROOT}/hack/tools/bin/kustomize" /usr/local/bin/

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

# Set USE_IRSO only when IMAGE_OS is not ubuntu and not running scalability tests
if [[ "${IMAGE_OS}" != "ubuntu" && "${GINKGO_FOCUS:-}" != "scalability" ]]; then
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
    echo 'export NODES_PLATFORM="fake"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
    echo 'export SKIP_APPLY_BMH="true"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
    echo 'export USE_IRSO="false"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
    sed -i "s/^export NUM_NODES=.*/export NUM_NODES=30/" "${M3_DEV_ENV_PATH}/config_${USER}.sh"
    echo 'CLUSTER_TOPOLOGY: true' >"${CAPI_CONFIG_FOLDER}/clusterctl.yaml"
    echo 'export BOOTSTRAP_CLUSTER="minikube"' >>"${M3_DEV_ENV_PATH}/config_${USER}.sh"
  ;;
esac

echo 'EXP_MACHINE_TAINT_PROPAGATION: true' >> "${CAPI_CONFIG_FOLDER}/clusterctl.yaml"

if [[ ${GINKGO_FOCUS:-} != "scalability" ]]; then
  # Don't run scalability tests if not asked for.
    export GINKGO_SKIP="${GINKGO_SKIP:-} scalability"
fi

# Clone BMO repo and install vbmctl
if ! command -v vbmctl >/dev/null 2>&1; then
  clone_repo "https://github.com/metal3-io/baremetal-operator.git" "main" "${WORKING_DIR}/baremetal-operator"
  pushd "${WORKING_DIR}/baremetal-operator/test/vbmctl/"
  go build -tags=e2e,integration -o vbmctl ./main.go
  sudo install vbmctl /usr/local/bin/vbmctl
  popd
fi


DNSMASQ_ENV="${REPO_ROOT}/test/e2e/data/dnsmasq.env"
docker run --name dnsmasq --rm -d --net=host --privileged --user 997:994 \
  --env-file "${DNSMASQ_ENV}" --entrypoint /bin/rundnsmasq \
  quay.io/metal3-io/ironic

virsh --connect qemu:///system attach-interface \
  --domain kind \
  --type network \
  --source provisioning \
  --mac="52:54:00:6c:3c:01" \
  --model virtio \
  --config \
  --persistent

IMAGE_DIR="/tmp/metal3"
mkdir -p "${IMAGE_DIR}/images"

IPA_HEADER_FILE="${IMAGE_DIR}/images/ipa-centos9-master.tar.gz.headers"
if [[ ! -f "${IPA_HEADER_FILE}" ]]; then
  curl -g --dump-header "${IPA_HEADER_FILE}" -O https://tarballs.opendev.org/openstack/ironic-python-agent/dib/ipa-centos9-master.tar.gz
fi

docker run --name image-server-e2e -d \
  -p 8080:8080 \
  -v "${IMAGE_DIR}:/usr/share/nginx/html" nginxinc/nginx-unprivileged

# E2E_BMCS_CONF_FILE="${REPO_ROOT}/test/e2e/config/bmcs.yaml"
export E2E_BMCS_CONF_FILE="${REPO_ROOT}/test/e2e/config/bmcs-redfish-virtualmedia.yaml"
vbmctl --yaml-source-file "${E2E_BMCS_CONF_FILE}"

# This IP is defined by the network above, and is used consistently in all of
# our e2e overlays
export IRONIC_PROVISIONING_IP="192.168.111.199"

# Start VBMC
docker run --name vbmc --network host -d \
  -v /var/run/libvirt/libvirt-sock:/var/run/libvirt/libvirt-sock \
  -v /var/run/libvirt/libvirt-sock-ro:/var/run/libvirt/libvirt-sock-ro \
  quay.io/metal3-io/vbmc

# Sushy-tools variables
SUSHY_EMULATOR_FILE="${REPO_ROOT}"/test/e2e/data/sushy-tools/sushy-emulator.conf
# Start sushy-tools
docker run --name sushy-tools -d --network host \
  -v "${SUSHY_EMULATOR_FILE}":/etc/sushy/sushy-emulator.conf:Z \
  -v /var/run/libvirt:/var/run/libvirt:Z \
  -e SUSHY_EMULATOR_CONFIG=/etc/sushy/sushy-emulator.conf \
  quay.io/metal3-io/sushy-tools:latest sushy-emulator

# Add ipmi nodes to vbmc
readarray -t BMCS < <(yq e -o=j -I=0 '.[]' "${E2E_BMCS_CONF_FILE}")
for bmc in "${BMCS[@]}"; do
  address=$(echo "${bmc}" | jq -r '.address')
  if [[ "${address}" != ipmi:* ]]; then
    continue
  fi
  hostName=$(echo "${bmc}" | jq -r '.hostName')
  vbmc_port="${address##*:}"
  docker exec vbmc vbmc add "${hostName}" --port "${vbmc_port}" --libvirt-uri "qemu:///system"
  docker exec vbmc vbmc start "${hostName}"
done

update_kustomize_image() {
  local image_name="${1:-}"      # e.g., quay.io/metal3-io/ironic
  local env_var_name="${2:-}"    # e.g., IRONIC_IMAGE
  local kustomize_dir="${3:-}"   # e.g., ./overlays/dev

  if [[ -z "${image_name}" || -z "${env_var_name}" || -z "${kustomize_dir}" ]]; then
    echo "Usage: update_kustomize_image <image_name> <env_var_name> <kustomize_dir>"
    return 1
  fi

  local full_image="${!env_var_name:-}"  # Resolve the env var value

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
export BARE_METAL_OPERATOR_IMAGE="${BARE_METAL_OPERATOR_IMAGE:-quay.io/metal3-io/baremetal-operator:main}"
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
      IRONIC_USERNAME="$(uuidgen)"
      echo "${IRONIC_USERNAME}" > "${IRONIC_AUTH_DIR}/ironic-username"
    else
      IRONIC_USERNAME="$(cat "${IRONIC_AUTH_DIR}/ironic-username")"
    fi
  fi
  if [[ -z "${IRONIC_PASSWORD:-}" ]]; then
    if [[ ! -f "${IRONIC_AUTH_DIR}/ironic-password" ]]; then
      IRONIC_PASSWORD="$(uuidgen)"
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
done

# run e2e tests
if [[ -n "${CLUSTER_TOPOLOGY:-}" ]]; then
  export CLUSTER_TOPOLOGY=true
  make e2e-clusterclass-tests
else
  export EXP_MACHINE_TAINT_PROPAGATION=true
  make e2e-tests
fi
