#!/bin/bash

set -x

# Initial manifest directory
DIR_NAME="/tmp/manifests/bootstrap-before-pivot"
DIR_NAME_AFTER_PIVOT="/tmp/manifests/target-after-pivot"
DIR_NAME_AFTER_REPIVOT="/tmp/manifests/bootstrap-after-repivot"
# Check if manifest directory exists
if [[ -d "${DIR_NAME}" ]] && [[ -d "${DIR_NAME_AFTER_PIVOT}" ]]; then
  DIR_NAME="${DIR_NAME_AFTER_REPIVOT}"
  mkdir -p "${DIR_NAME}"
  # Bootstrap cluster kubeconfig
  kconfig="${KUBECONFIG_BOOTSTRAP}"
elif [[ -d "${DIR_NAME}" ]] && [[ ! -d "${DIR_NAME_AFTER_PIVOT}" ]]; then
  DIR_NAME="${DIR_NAME_AFTER_PIVOT}"
  mkdir -p "${DIR_NAME}"
  # Target cluster kubeconfig
  kconfig="${KUBECONFIG_WORKLOAD}"
else
  mkdir -p "${DIR_NAME}"
  # Bootstrap cluster kubeconfig
  kconfig="${KUBECONFIG_BOOTSTRAP}"
fi

manifests=(
  bmh
  hardwaredata
  cluster
  deployment
  machine
  machinedeployment
  machinehealthchecks
  machinesets
  machinepools
  m3cluster
  m3machine
  metal3machinetemplate
  kubeadmconfig
  kubeadmconfigtemplates
  kubeadmcontrolplane
  replicaset
  ippool
  ipclaim
  ipaddress
  m3data
  m3dataclaim
  m3datatemplate
)

set +x

NAMESPACES="$(kubectl --kubeconfig="${kconfig}" get namespace -o jsonpath='{.items[*].metadata.name}')"
for NAMESPACE in ${NAMESPACES}; do
  for kind in "${manifests[@]}"; do
    mkdir -p "${DIR_NAME}/${NAMESPACE}/${kind}"
    for name in $(kubectl --kubeconfig="${kconfig}" get -n "${NAMESPACE}" -o name "${kind}" || true); do
      kubectl --kubeconfig="${kconfig}" get -n "${NAMESPACE}" -o yaml "${name}" | tee "${DIR_NAME}/${NAMESPACE}/${kind}/$(basename "${name}").yaml" || true
    done
  done
done
