#!/bin/bash
# TODO: Remove this file once vbmctl is distributed as a release artifact by BMO.
#
# This script builds vbmctl from the BMO source repository when a pre-built
# binary is not already present. It is sourced by ci-e2e.sh.
#
# Expected env vars (with defaults):
#   BMO_REPO   - BMO git repository URL
#   BMO_BRANCH - BMO branch to clone
#   REPO_ROOT  - root of the CAPM3 repository (set by ci-e2e.sh)

set -euxo pipefail

BMO_REPO="${BMO_REPO:-https://github.com/metal3-io/baremetal-operator.git}"
BMO_BRANCH="${BMO_BRANCH:-main}"
BMO_PATH="${REPO_ROOT}/_out/bmo"

if [[ ! -f "${REPO_ROOT}/_out/bin/vbmctl" ]]; then
  echo "Building vbmctl from source (${BMO_REPO}@${BMO_BRANCH})..."

  # Install build dependencies and libvirt required by vbmctl's CGo bindings
  if command -v apt-get &>/dev/null; then
    sudo apt-get update -qq
    sudo apt-get install -y -qq build-essential tar libvirt-dev pkg-config \
      libvirt-daemon-system qemu-kvm virt-manager libcap2-bin
  elif command -v dnf &>/dev/null; then
    sudo dnf install -y gcc make tar pkgconf libcap \
      libvirt-daemon qemu-kvm libvirt-client
    # libvirt-devel may not exist on EL10; install from CRB/powertools if needed
    sudo dnf install -y libvirt-devel 2>/dev/null || \
      sudo dnf install -y --enablerepo="*crb*" libvirt-devel 2>/dev/null || \
      sudo dnf install -y --enablerepo="*powertools*" libvirt-devel 2>/dev/null || \
      echo "WARNING: libvirt-devel not available, attempting build anyway"
  fi

  rm -rf "${BMO_PATH}"
  git clone --depth 1 --branch "${BMO_BRANCH}" "${BMO_REPO}" "${BMO_PATH}"
  make -C "${BMO_PATH}" build-vbmctl
  mkdir -p "${REPO_ROOT}/_out/bin"
  cp "${BMO_PATH}/bin/vbmctl" "${REPO_ROOT}/_out/bin/vbmctl"
fi

VBMCTL="${REPO_ROOT}/_out/bin/vbmctl"

# Grant vbmctl network admin capabilities (needed for veth pair creation).
if command -v setcap &>/dev/null; then
  sudo setcap cap_net_admin+epi "${VBMCTL}" || echo "WARNING: setcap failed; vbmctl may require sudo for network operations"
else
  echo "WARNING: setcap not found; install libcap2-bin/libcap to avoid requiring sudo for vbmctl network operations"
fi
