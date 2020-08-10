#!/bin/bash
# Copyright 2019 The Kubernetes Authors.
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

# Directories.
SOURCE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
OUTPUT_DIR=${OUTPUT_DIR:-${SOURCE_DIR}/_out}
KUSTOMIZE="${SOURCE_DIR}/../hack/tools/bin/kustomize"

# Binaries
ENVSUBST=${ENVSUBST:-envsubst}
command -v "${ENVSUBST}" >/dev/null 2>&1 || echo -v "Cannot find ${ENVSUBST} in path."

# Cluster.
export CLUSTER_NAME="${CLUSTER_NAME:-test1}"
export KUBERNETES_VERSION="${KUBERNETES_VERSION:-v1.16.0}"

# Machine settings.
export CONTROL_PLANE_MACHINE_TYPE="${CONTROL_PLANE_MACHINE_TYPE:-t2.medium}"
export NODE_MACHINE_TYPE="${CONTROL_PLANE_MACHINE_TYPE:-t2.medium}"
export SSH_KEY_NAME="${SSH_KEY_NAME:-default}"

# BMO settings
export DEPLOY_KERNEL_URL="${DEPLOY_KERNEL_URL:-http://127.0.0.1:6180/images/ironic-python-agent.kernel}"
export DEPLOY_RAMDISK_URL="${DEPLOY_RAMDISK_URL:-http://127.0.0.1:6180/images/ironic-python-agent.initramfs}"
export IRONIC_URL="${IRONIC_URL:-http://127.0.0.1:6385/v1/}"
export IRONIC_INSPECTOR_URL="${IRONIC_INSPECTOR_URL:-http://127.0.0.1:5050/v1/}"

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

# Generate cluster resources.
"$KUSTOMIZE" build "${SOURCE_DIR}/cluster" | envsubst > "${CLUSTER_GENERATED_FILE}"
echo "Generated ${CLUSTER_GENERATED_FILE}"

# Generate controlplane resources.
"$KUSTOMIZE" build "${SOURCE_DIR}/controlplane" | envsubst > "${CONTROLPLANE_GENERATED_FILE}"
echo "Generated ${CONTROLPLANE_GENERATED_FILE}"

# Generate metal3crds resources.
"$KUSTOMIZE" build "${SOURCE_DIR}/metal3crds" | envsubst > "${METAL3CRDS_GENERATED_FILE}"
echo "Generated ${METAL3CRDS_GENERATED_FILE}"

# Generate metal3plane resources.
"$KUSTOMIZE" build "${SOURCE_DIR}/metal3plane" | envsubst > "${METAL3PLANE_GENERATED_FILE}"
echo "Generated ${METAL3PLANE_GENERATED_FILE}"

# Generate machinedeployment resources.
"$KUSTOMIZE" build "${SOURCE_DIR}/machinedeployment" | envsubst >> "${MACHINEDEPLOYMENT_GENERATED_FILE}"
echo "Generated ${MACHINEDEPLOYMENT_GENERATED_FILE}"

# Get Cert-manager provider components file
curl -L -o "${COMPONENTS_CERT_MANAGER_GENERATED_FILE}" https://github.com/jetstack/cert-manager/releases/download/v0.13.0/cert-manager.yaml

# Get enhanced envsubst version to evaluate expressions like ${VAR:=default}
curl -L -o "${SOURCE_DIR}"/envsubst-go https://github.com/a8m/envsubst/releases/download/v1.2.0/envsubst-"$(uname -s)"-"$(uname -m)"
chmod +x "${SOURCE_DIR}"/envsubst-go

# Generate Cluster API provider components file.
"$KUSTOMIZE" build "github.com/kubernetes-sigs/cluster-api/config/?ref=master" | "${SOURCE_DIR}"/envsubst-go > "${COMPONENTS_CLUSTER_API_GENERATED_FILE}"
echo "Generated ${COMPONENTS_CLUSTER_API_GENERATED_FILE}"

# Generate Kubeadm Bootstrap Provider components file.
"$KUSTOMIZE" build "github.com/kubernetes-sigs/cluster-api/bootstrap/kubeadm/config/?ref=master" | "${SOURCE_DIR}"/envsubst-go > "${COMPONENTS_KUBEADM_GENERATED_FILE}"
echo "Generated ${COMPONENTS_KUBEADM_GENERATED_FILE}"

# Generate Kubeadm Controlplane components file.
"$KUSTOMIZE" build "github.com/kubernetes-sigs/cluster-api/controlplane/kubeadm/config/?ref=master" | "${SOURCE_DIR}"/envsubst-go > "${COMPONENTS_CTRLPLANE_GENERATED_FILE}"
echo "Generated ${COMPONENTS_CTRLPLANE_GENERATED_FILE}"

# Generate METAL3 Infrastructure Provider components file.
"$KUSTOMIZE" build "${SOURCE_DIR}/../config" | envsubst > "${COMPONENTS_METAL3_GENERATED_FILE}"
echo "Generated ${COMPONENTS_METAL3_GENERATED_FILE}"

# Generate a single provider components file.
"$KUSTOMIZE" build "${SOURCE_DIR}/provider-components" | envsubst > "${PROVIDER_COMPONENTS_GENERATED_FILE}"
echo "Generated ${PROVIDER_COMPONENTS_GENERATED_FILE}"
