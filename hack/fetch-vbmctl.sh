#!/bin/bash
# Fetches the vbmctl binary from a BMO release. vbmctl has been published as a
# release artifact since BMO v0.14.0, so we download it instead of building from
# source. Sourced by ci-e2e.sh.
#
# Expected env vars (with defaults):
#   REPO_ROOT      - root of the CAPM3 repository (set by ci-e2e.sh)
#   VBMCTL_VERSION - BMO release tag to fetch vbmctl from

set -euxo pipefail

VBMCTL_VERSION="${VBMCTL_VERSION:-v0.14.0}"
VBMCTL="${REPO_ROOT}/_out/bin/vbmctl"

# vbmctl uses CGo libvirt bindings and drives libvirt/qemu to create the VMs, so
# it needs the libvirt daemon, client and qemu at runtime.
if command -v apt-get &>/dev/null; then
  sudo apt-get update -qq
  sudo apt-get install -y -qq libvirt-daemon-system libvirt-clients qemu-kvm libcap2-bin
elif command -v dnf &>/dev/null; then
  sudo dnf install -y libvirt-daemon libvirt-client qemu-kvm libcap
fi

if [[ ! -f "${VBMCTL}" ]]; then
  arch="$(uname -m)"
  case "${arch}" in
    x86_64) arch="amd64" ;;
    aarch64) arch="arm64" ;;
  esac
  url="https://github.com/metal3-io/baremetal-operator/releases/download/${VBMCTL_VERSION}/vbmctl-linux-${arch}"
  echo "Downloading vbmctl ${VBMCTL_VERSION} (linux-${arch}) from ${url}..."
  mkdir -p "${REPO_ROOT}/_out/bin"
  curl -fsSL -o "${VBMCTL}" "${url}"
  chmod +x "${VBMCTL}"
fi

# Grant vbmctl network admin capabilities (needed for veth pair creation).
if command -v setcap &>/dev/null; then
  sudo setcap cap_net_admin+epi "${VBMCTL}" || echo "WARNING: setcap failed; vbmctl may require sudo for network operations"
else
  echo "WARNING: setcap not found; install libcap2-bin/libcap to avoid requiring sudo for vbmctl network operations"
fi
