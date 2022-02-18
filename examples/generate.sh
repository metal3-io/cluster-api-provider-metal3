#!/bin/bash
# Copyright 2021 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

# Directories.
SOURCE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
OUTPUT_DIR=${OUTPUT_DIR:-${SOURCE_DIR}/_out}
KUSTOMIZE="${SOURCE_DIR}/../hack/tools/bin/kustomize"

# Cluster.
export CLUSTER_NAME="${CLUSTER_NAME:-test1}"
export KUBERNETES_VERSION="${KUBERNETES_VERSION:-v1.23.3}"
export CLUSTER_APIENDPOINT_HOST="${CLUSTER_APIENDPOINT_HOST:-192.168.111.249}"
export CLUSTER_APIENDPOINT_PORT="${CLUSTER_APIENDPOINT_PORT:-6443}"
export IMAGE_URL="${IMAGE_URL:-http://172.22.0.1/images/UBUNTU_20.04_NODE_IMAGE_K8S_v1.23.3-raw.img}"
export IMAGE_CHECKSUM="${IMAGE_CHECKSUM:-http://172.22.0.1/images/UBUNTU_20.04_NODE_IMAGE_K8S_v1.23.3-raw.img.md5sum}"
export IMAGE_CHECKSUM_TYPE="${IMAGE_CHECKSUM_TYPE:-md5}"
export IMAGE_FORMAT="${IMAGE_FORMAT:-raw}"

# Machine settings.
export CONTROL_PLANE_MACHINE_TYPE="${CONTROL_PLANE_MACHINE_TYPE:-t2.medium}"
export NODE_MACHINE_TYPE="${CONTROL_PLANE_MACHINE_TYPE:-t2.medium}"
export SSH_KEY_NAME="${SSH_KEY_NAME:-default}"
export NODE_DRAIN_TIMEOUT="${NODE_DRAIN_TIMEOUT:-"0s"}"

# Outputs.
COMPONENTS_CERT_MANAGER_GENERATED_FILE=${OUTPUT_DIR}/cert-manager.yaml
COMPONENTS_CLUSTER_API_GENERATED_FILE=${SOURCE_DIR}/provider-components/core-components.yaml
COMPONENTS_KUBEADM_GENERATED_FILE=${SOURCE_DIR}/provider-components/bootstrap-components.yaml
COMPONENTS_CTRLPLANE_GENERATED_FILE=${SOURCE_DIR}/provider-components/ctlplane-components.yaml
COMPONENTS_METAL3_GENERATED_FILE=${SOURCE_DIR}/provider-components/infrastructure-components.yaml

PROVIDER_COMPONENTS_GENERATED_FILE=${OUTPUT_DIR}/provider-components.yaml
CLUSTER_GENERATED_FILE=${OUTPUT_DIR}/cluster.yaml
CONTROLPLANE_GENERATED_FILE=${OUTPUT_DIR}/controlplane.yaml
MACHINEDEPLOYMENT_GENERATED_FILE=${OUTPUT_DIR}/machinedeployment.yaml
METAL3PLANE_GENERATED_FILE=${OUTPUT_DIR}/metal3plane.yaml
METAL3CRDS_GENERATED_FILE=${OUTPUT_DIR}/metal3crds.yaml

# Overwrite flag.
OVERWRITE=0

SCRIPT=$(basename "$0")
while test $# -gt 0; do
        case "$1" in
          -h|--help)
            echo "$SCRIPT - generates input yaml files for Cluster API on metal3"
            echo " "
            echo "$SCRIPT [options]"
            echo " "
            echo "options:"
            echo "-h, --help                show brief help"
            echo "-f, --force-overwrite     if file to be generated already exists, force script to overwrite it"
            exit 0
            ;;
          -f)
            OVERWRITE=1
            shift
            ;;
          --force-overwrite)
            OVERWRITE=1
            shift
            ;;
          *)
            break
            ;;
        esac
done

if [ $OVERWRITE -ne 1 ] && [ -d "$OUTPUT_DIR" ]; then
  echo "ERR: Folder ${OUTPUT_DIR} already exists. Delete it manually before running this script."
  exit 1
fi

mkdir -p "${OUTPUT_DIR}"


# Get enhanced envsubst version to evaluate expressions like ${VAR:=default}
ENVSUBST="${SOURCE_DIR}/envsubst-go"
curl --fail -Ss -L -o "${ENVSUBST}" https://github.com/a8m/envsubst/releases/download/v1.2.0/envsubst-"$(uname -s)"-"$(uname -m)"
chmod +x "$ENVSUBST"


# Generate cluster resources.
"$KUSTOMIZE" build "${SOURCE_DIR}/cluster" | "$ENVSUBST" > "${CLUSTER_GENERATED_FILE}"
echo "Generated ${CLUSTER_GENERATED_FILE}"

# Generate controlplane resources.
"$KUSTOMIZE" build "${SOURCE_DIR}/controlplane" | "$ENVSUBST" > "${CONTROLPLANE_GENERATED_FILE}"
echo "Generated ${CONTROLPLANE_GENERATED_FILE}"

# Generate metal3crds resources.
"$KUSTOMIZE" build "${SOURCE_DIR}/metal3crds" | "$ENVSUBST" > "${METAL3CRDS_GENERATED_FILE}"
echo "Generated ${METAL3CRDS_GENERATED_FILE}"

# Generate metal3plane resources.
"$KUSTOMIZE" build "${SOURCE_DIR}/metal3plane" | "$ENVSUBST" > "${METAL3PLANE_GENERATED_FILE}"
echo "Generated ${METAL3PLANE_GENERATED_FILE}"

# Generate machinedeployment resources.
"$KUSTOMIZE" build "${SOURCE_DIR}/machinedeployment" | "$ENVSUBST" >> "${MACHINEDEPLOYMENT_GENERATED_FILE}"
echo "Generated ${MACHINEDEPLOYMENT_GENERATED_FILE}"

# Get Cert-manager provider components file
curl --fail -Ss -L -o "${COMPONENTS_CERT_MANAGER_GENERATED_FILE}" https://github.com/cert-manager/cert-manager/releases/download/v1.5.3/cert-manager.yaml
echo "Downloaded ${COMPONENTS_CERT_MANAGER_GENERATED_FILE}"

# Generate Cluster API provider components file.
"$KUSTOMIZE" build "github.com/kubernetes-sigs/cluster-api/config/default/?ref=main" | "$ENVSUBST" > "${COMPONENTS_CLUSTER_API_GENERATED_FILE}"
echo "Generated ${COMPONENTS_CLUSTER_API_GENERATED_FILE}"

# Generate Kubeadm Bootstrap Provider components file.
"$KUSTOMIZE" build "github.com/kubernetes-sigs/cluster-api/bootstrap/kubeadm/config/default/?ref=main" | "$ENVSUBST" > "${COMPONENTS_KUBEADM_GENERATED_FILE}"
echo "Generated ${COMPONENTS_KUBEADM_GENERATED_FILE}"

# Generate Kubeadm Controlplane components file.
"$KUSTOMIZE" build "github.com/kubernetes-sigs/cluster-api/controlplane/kubeadm/config/default/?ref=main" | "$ENVSUBST" > "${COMPONENTS_CTRLPLANE_GENERATED_FILE}"
echo "Generated ${COMPONENTS_CTRLPLANE_GENERATED_FILE}"

# Generate METAL3 Infrastructure Provider components file.
"$KUSTOMIZE" build "${SOURCE_DIR}/../config/default" | "$ENVSUBST" > "${COMPONENTS_METAL3_GENERATED_FILE}"
echo "Generated ${COMPONENTS_METAL3_GENERATED_FILE}"

# Generate a single provider components file.
"$KUSTOMIZE" build "${SOURCE_DIR}/provider-components" | "$ENVSUBST" > "${PROVIDER_COMPONENTS_GENERATED_FILE}"
echo "Generated ${PROVIDER_COMPONENTS_GENERATED_FILE}"
