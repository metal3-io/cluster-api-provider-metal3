#!/usr/bin/env bash

set -x

DIR_NAME="/tmp/target_cluster_logs"
NAMESPACES="$(kubectl --kubeconfig="${KUBE_CONFIG}" get namespace -o json | jq -r '.items[].metadata.name')"
mkdir -p "${DIR_NAME}"
for NAMESPACE in ${NAMESPACES}
do
  mkdir -p "${DIR_NAME}/${NAMESPACE}"
# Fetch logs from target cluster according to the pods name
  PODS="$(kubectl --kubeconfig="${KUBE_CONFIG}" get pods -n "${NAMESPACE}" -o json | jq -r '.items[].metadata.name')"
  for POD in ${PODS}
  do
    mkdir -p "${DIR_NAME}/${NAMESPACE}/${POD}"
    CONTAINERS="$(kubectl --kubeconfig="${KUBE_CONFIG}" get pods -n "${NAMESPACE}" "${POD}" -o json | jq -r '.spec.containers[].name')"
    for CONTAINER in ${CONTAINERS}
    do
      LOG_DIR="${DIR_NAME}/${NAMESPACE}/${POD}/${CONTAINER}"
      mkdir -p "${LOG_DIR}"
      kubectl --kubeconfig="${KUBE_CONFIG}" logs -n "${NAMESPACE}" "${POD}" "${CONTAINER}" \
        > "${LOG_DIR}/stdout.log" 2> "${LOG_DIR}/stderr.log"
    done
  done
done