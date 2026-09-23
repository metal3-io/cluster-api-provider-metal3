#!/bin/sh

# Ignore the rule that says we should always quote variables, because
# in this script we *do* want globbing.
# shellcheck disable=SC2086,SC2292

set -eux

IS_CONTAINER="${IS_CONTAINER:-false}"
ARTIFACTS="${ARTIFACTS:-/tmp}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
    export XDG_CACHE_HOME="/tmp/.cache"

    # Copy source to a writable temp directory so make generate can write
    # generated files without requiring a read-write host mount.
    # This script is a verification check only — generated output is not
    # written back to the host working tree.
    CODEGEN_DIR="$(mktemp -d -t codegen.XXXXXX)"

    # This part could be run outside of a container and generated files
    # should be cleaned up just in case this is run locally and not in test
    # environment.
    trap 'rm -rf "${CODEGEN_DIR}"' EXIT
    cp -a . "${CODEGEN_DIR}"
    cd "${CODEGEN_DIR}"

    INPUT_FILES="$(git ls-files config) $(git ls-files | grep zz_generated)"
    cksum ${INPUT_FILES} > "${ARTIFACTS}/lint.cksums.before"
    export VERBOSE="--verbose"
    make generate
    cksum ${INPUT_FILES} > "${ARTIFACTS}/lint.cksums.after"
    diff "${ARTIFACTS}/lint.cksums.before" "${ARTIFACTS}/lint.cksums.after"

else
    "${CONTAINER_RUNTIME}" run --rm \
        --env IS_CONTAINER=TRUE \
        --volume "${PWD}:/workdir:ro,z" \
        --entrypoint sh \
        --workdir /workdir \
        docker.io/golang:1.26 \
        /workdir/hack/codegen.sh "$@"
fi
