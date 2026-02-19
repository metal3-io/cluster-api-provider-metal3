#!/usr/bin/env bash

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}" || exit 1

docker rm -f vbmc
docker rm -f image-server-e2e
docker rm -f sushy-tools
docker rm -f dnsmasq

BMH_NAME_REGEX="^node-"
BMH_NAMESPACE="${BMH_NAMESPACE:-metal3}"
# Get a list of all virtual machines
VM_LIST=$(virsh -c qemu:///system list --all --name | grep "${BMH_NAME_REGEX}")

if [[ -n "${VM_LIST}" ]]; then
    # Loop through the list and delete each virtual machine
    for vm_name in ${VM_LIST}; do
        virsh -c qemu:///system destroy --domain "${vm_name}"
        virsh -c qemu:///system undefine --domain "${vm_name}" --remove-all-storage --nvram
        kubectl -n "${BMH_NAMESPACE}" delete baremetalhost "${vm_name}" --ignore-not-found
    done
else
    echo "No virtual machines found. Skipping..."
fi

rm -rf "${REPO_ROOT}/test/e2e/_artifacts"
rm -rf "${REPO_ROOT}"/artifacts-*
rm -rf "${REPO_ROOT}/test/e2e/images"

# Clear network
network="provisioning"
virsh -c qemu:///system net-destroy "${network}"
virsh -c qemu:///system net-undefine "${network}"

# Clean volume pool directory
rm -rf /tmp/pool_oo/*
rm -rf /tmp/node-*.qcow2

# Clean volume pool
virsh pool-destroy default || true
virsh pool-delete default || true
virsh pool-undefine default || true
