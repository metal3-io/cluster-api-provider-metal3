#!/usr/bin/env bash

# Sets up the virtual bare metal lab (VMs, networks, BMC emulator and image
# server) used by the e2e tests via the vbmctl CLI.
#
# This script is meant to be *sourced* from scripts/ci-e2e.sh so that it shares
# the same shell environment.
#
# Expected env vars (set by ci-e2e.sh):
#   REPO_ROOT       - root of the CAPM3 repository
#   NUM_NODES       - number of bare metal nodes to create (set by environment.sh)
#   IRONIC_DATA_DIR - Ironic data directory
#
# Exported for the caller and the Go tests:
#   VBMCTL          - path to the vbmctl binary (set by build-vbmctl.sh)
#   VBMCTL_CONFIG   - generated vbmctl lab config (used by the cleanup trap)
#   E2E_BMCS_CONFIG - generated BMC config consumed by the Go tests

set -euxo pipefail

export PROVISIONING_IP="${PROVISIONING_IP:-172.22.0.1}"
export EXTERNAL_SUBNET_V4_HOST="${EXTERNAL_SUBNET_V4_HOST:-192.168.111.1}"
export CLUSTER_EXTERNAL_IP="${CLUSTER_EXTERNAL_IP:-192.168.111.2}"
export PROVISIONING_BRIDGE="${PROVISIONING_BRIDGE:-provisioning}"
export EXTERNAL_BRIDGE="${EXTERNAL_BRIDGE:-external}"
export PROVISIONING_NETWORK_NAME="${PROVISIONING_NETWORK_NAME:-provisioning-e2e}"
export EXTERNAL_NETWORK_NAME="${EXTERNAL_NETWORK_NAME:-external-e2e}"
export BMC_EMULATOR_PORT="${BMC_EMULATOR_PORT:-8000}"
export BMC_EMULATOR_IMAGE="${BMC_EMULATOR_IMAGE:-quay.io/metal3-io/sushy-tools:latest}"

# TODO: Remove once vbmctl ships as a BMO release artifact; replace with a
# direct download. See hack/hack-build-vbmctl.sh.
# shellcheck source=./hack/build-vbmctl.sh
source "${REPO_ROOT}/hack/build-vbmctl.sh"

# Number of bare metal nodes to create. Normally set by scripts/environment.sh
# based on GINKGO_FOCUS; default here as a fallback so the script is safe to run
# on its own (e.g. running ci-e2e.sh directly with no focus).
export NUM_NODES="${NUM_NODES:-4}"
if ! [[ "${NUM_NODES}" =~ ^[0-9]+$ ]] || [[ "${NUM_NODES}" -lt 1 ]]; then
  echo "NUM_NODES must be a positive integer, got: ${NUM_NODES}" >&2
  exit 1
fi

generate_vbmctl_vms() {
  local vm_entries=""
  local i

  for ((i=0; i<NUM_NODES; i++)); do
    local suffix
    suffix=$(printf "%02d" "$((i + 1))")
    vm_entries+="  - name: node-${i}
    memory: 4096
    vcpus: 2
    volumes:
    - name: \"1\"
      size: 20
    networkAttachments:
    - network: ${PROVISIONING_NETWORK_NAME}
      macAddress: \"00:60:2f:31:81:${suffix}\"
    - network: ${EXTERNAL_NETWORK_NAME}
      macAddress: \"00:60:2f:32:81:${suffix}\"
"
  done

  export VBMCTL_VMS="${vm_entries%$'\n'}"
}

# Generate bmcs config dynamically based on NUM_NODES
generate_bmcs_config() {
  local bmcs_entries=""
  local i

  for ((i=0; i<NUM_NODES; i++)); do
    local suffix
    suffix=$(printf "%02d" "$((i + 1))")
    local ip_last_octet=$((20 + i))
    bmcs_entries+="- name: \"node-${i}\"
  address: \"redfish-virtualmedia+http://${PROVISIONING_IP}:${BMC_EMULATOR_PORT}/redfish/v1/Systems/node-${i}\"
  bootMacAddress: \"00:60:2f:31:81:${suffix}\"
  ipAddress: \"192.168.111.${ip_last_octet}\"
  user: admin
  password: password
  rootDeviceHints:
    deviceName: \"/dev/vda\"
"
  done

  echo -n "${bmcs_entries}" > "${E2E_BMCS_CONFIG}"
}

# Generate vbmctl config from template
export VBMCTL_CONFIG="${REPO_ROOT}/_out/vbmctl.yaml"
generate_vbmctl_vms
envsubst < "${REPO_ROOT}/test/e2e/config/vbmctl.yaml.tmpl" > "${VBMCTL_CONFIG}"

# Generate bmcs config dynamically (consumed by Go tests to create BMH objects)
export E2E_BMCS_CONFIG="${REPO_ROOT}/_out/bmcs.yaml"
generate_bmcs_config


# Ensure Ironic data directories exist before vbmctl creates the lab
mkdir -p "${IRONIC_DATA_DIR}/html/images"

# vbmctl connects to libvirt directly to create the VMs. Verify we can reach it
# before creating containers, otherwise the run fails part way through with a
# cryptic libvirt error. Modern libvirt uses modular daemons (virtqemud) with
# systemd socket activation, so the socket path varies and may not exist until
# first use; actually connecting is the only reliable check. If the connection
# fails on permissions, add the user to the libvirt group (group membership
# changes do not apply to the current shell, so we stop with clear guidance).
ensure_libvirt_access() {
  local uri="${LIBVIRT_DEFAULT_URI:-qemu:///system}"

  # If virsh is unavailable we cannot preflight; let vbmctl surface any error.
  command -v virsh &>/dev/null || return 0

  # Already able to connect (running as root or user already in the group).
  if virsh -c "${uri}" uri &>/dev/null; then
    return 0
  fi

  # Prefer the "libvirt" group, fall back to the legacy "libvirtd" group.
  local libvirt_group="libvirt"
  getent group libvirt &>/dev/null || libvirt_group="libvirtd"

  echo "Adding ${USER} to the '${libvirt_group}' group for libvirt access..."
  sudo usermod -aG "${libvirt_group}" "${USER}" || true
  cat >&2 <<EOF
ERROR: cannot connect to libvirt at ${uri}.
If libvirtd is not running, start it with:

    sudo systemctl enable --now libvirtd

Otherwise this is most likely a permissions issue. ${USER} has now been added to
the '${libvirt_group}' group, but group membership changes do not apply to the
current shell session. Start a new login session (log out and back in) or run:

    newgrp ${libvirt_group}

and then re-run the tests.
EOF
  exit 1
}
ensure_libvirt_access

# Create virtual bare metal lab (VMs, networks, BMC emulator, image server)
echo "Creating virtual bare metal lab with vbmctl..."
"${VBMCTL}" -c "${VBMCTL_CONFIG}" create bml

# Wait for the sushy-tools BMC emulator to become reachable on the provisioning
# IP. sushy-tools may start before the provisioning bridge IP is assigned,
# failing to bind ("Cannot assign requested address"). If Ironic later tries to
# register a BMH before the emulator is listening it fails with
# "Connection refused", which surfaces much later as an opaque BMH registration
# error. Poll the Redfish endpoint, restarting the container if needed, and fail
# loudly if it never comes up.
wait_for_sushy_tools() {
  local redfish_url="http://${PROVISIONING_IP}:${BMC_EMULATOR_PORT}/redfish/v1/"
  local container attempts=0 max_attempts=30

  # Detect the sushy-tools container name (vbmctl may name it with or without an
  # "-e2e" suffix depending on version), so the restart is not silently skipped.
  for name in vbmctl-sushy-tools vbmctl-sushy-tools-e2e; do
    if docker inspect "${name}" &>/dev/null; then
      container="${name}"
      break
    fi
  done
  if [[ -z "${container:-}" ]]; then
    echo "ERROR: sushy-tools BMC emulator container not found" >&2
    docker ps -a --format '{{.Names}}' | grep -i sushy >&2 || true
    return 1
  fi

  echo "Waiting for sushy-tools (${container}) to serve ${redfish_url}..."
  while (( attempts < max_attempts )); do
    # Bound each probe so a black-holed IP cannot hang the whole loop.
    if curl -sSf --connect-timeout 3 --max-time 5 "${redfish_url}" &>/dev/null; then
      echo "sushy-tools is reachable."
      return 0
    fi
    # After every 5 attempts, restart to recover from the bind race.
    attempts=$((attempts + 1))
    if (( attempts < max_attempts && attempts % 5 == 0 )); then
      echo "sushy-tools not reachable yet; restarting ${container} to pick up the bridge IP..."
      docker restart "${container}" || true
    fi
    sleep 2
  done

  echo "ERROR: sushy-tools did not become reachable at ${redfish_url}" >&2
  docker logs --tail 50 "${container}" >&2 || true
  return 1
}
wait_for_sushy_tools

# Docker's nftables FORWARD chain uses "policy drop" and only accepts traffic
# on its own bridges (docker0, kind-bridge). Libvirt sets up NAT and forwarding
# rules in a separate table (libvirt_network) but packets must also pass
# Docker's filter chain. Add accept rules to Docker's DOCKER-USER chain
# (the standard hook for user-defined overrides) for the libvirt bridges.
echo "Adding nftables rules for VM internet access..."
add_nft_rule() {
  # Only add the rule if it doesn't already exist (makes the script idempotent).
  if ! sudo nft list chain ip filter DOCKER-USER 2>/dev/null | grep -Fq -- "$*"; then
    sudo nft add rule ip filter DOCKER-USER "$@"
  fi
}
add_nft_rule iifname "${EXTERNAL_BRIDGE}" accept
add_nft_rule oifname "${EXTERNAL_BRIDGE}" accept
add_nft_rule iifname "${PROVISIONING_BRIDGE}" accept
add_nft_rule oifname "${PROVISIONING_BRIDGE}" accept
