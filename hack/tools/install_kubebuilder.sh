#!/usr/bin/env bash

set -eux

[[ -f bin/kubebuilder ]] && exit 0

if ! command -v sha256sum &>/dev/null; then
    echo "ERROR: sha256sum not found. On macOS, install coreutils: brew install coreutils" >&2
    exit 1
fi

version=4.14.0
arch=$(go env GOARCH)
os=$(go env GOOS)

mkdir -p ./bin

CHECKSUMS_URL="https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${version}/checksums.txt"
CHECKSUMS_FILE="checksums.txt"
BINARY_FILE="kubebuilder_${os}_${arch}"
URL="https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${version}/${BINARY_FILE}"

# Download checksums file
curl --proto '=https' --tlsv1.3 -sSfL \
    --retry 3 --retry-delay 5 --max-time 120 \
    -o "${CHECKSUMS_FILE}" "${CHECKSUMS_URL}"

# Extract the checksum for this platform
# Kubebuilder publishes bare binaries, so we match exactly on the filename
KUBEBUILDER_SHA256="$(grep "kubebuilder_${os}_${arch}$" "${CHECKSUMS_FILE}" | awk '{print $1;}')"
if [[ -z "${KUBEBUILDER_SHA256}" ]]; then
    echo >&2 "fatal: could not find checksum for ${BINARY_FILE} in ${CHECKSUMS_URL}"
    rm -f "${CHECKSUMS_FILE}"
    exit 1
fi

# Download binary with security flags
curl --proto '=https' --tlsv1.3 -sSfL \
    --retry 3 --retry-delay 5 --max-time 120 \
    -o "${BINARY_FILE}" "${URL}"

# Verify checksum before using
checksum="$(sha256sum "${BINARY_FILE}" | awk '{print $1;}')"
if [[ "${checksum}" != "${KUBEBUILDER_SHA256}" ]]; then
    echo >&2 "fatal: ${URL} checksum '${checksum}' differs from expected '${KUBEBUILDER_SHA256}'"
    rm -f "${BINARY_FILE}" "${CHECKSUMS_FILE}"
    exit 1
fi

# Install binary
mv "${BINARY_FILE}" ./bin/kubebuilder
chmod 0755 ./bin/kubebuilder

rm -f "${CHECKSUMS_FILE}"
