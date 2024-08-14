#!/usr/bin/env bash

set -x

DIR_NAME="/tmp/target_cluster_logs"
NAMESPACES="$(kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" get namespace -o jsonpath='{.items[*].metadata.name}')"
mkdir -p "${DIR_NAME}"

set +x

for NAMESPACE in ${NAMESPACES}
do
  mkdir -p "${DIR_NAME}/${NAMESPACE}"
# Fetch logs from target cluster according to the pods name
  PODS="$(kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" get pods -n "${NAMESPACE}" -o jsonpath='{.items[*].metadata.name}')"
  for POD in ${PODS}
  do
    mkdir -p "${DIR_NAME}/${NAMESPACE}/${POD}"
    kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" describe pods -n "${NAMESPACE}" "${POD}" \
      > "${DIR_NAME}/${NAMESPACE}/${POD}/stdout_describe.log" \
      2> "${DIR_NAME}/${NAMESPACE}/${POD}/stderr_describe.log"
    CONTAINERS="$(kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" get pods -n "${NAMESPACE}" "${POD}" -o jsonpath='{.spec.containers[*].name}')"
    for CONTAINER in ${CONTAINERS}
    do
      LOG_DIR="${DIR_NAME}/${NAMESPACE}/${POD}/${CONTAINER}"
      mkdir -p "${LOG_DIR}"
      kubectl --kubeconfig="${KUBECONFIG_WORKLOAD}" logs -n "${NAMESPACE}" "${POD}" "${CONTAINER}" \
        > "${LOG_DIR}/stdout.log" 2> "${LOG_DIR}/stderr.log"
    done
  done
done