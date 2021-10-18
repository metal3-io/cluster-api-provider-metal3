#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

# --strict: Disallow additional properties not in schema.
# --ignore-missing-schemas: Skip validation for resource 
# definitions without a schema. This will skip the checks
# for the Custom Resource Definitions(CRDs).
# -d, --directories strings: A comma-separated list of
# directories to recursively search for YAML documents.
# -i, --ignored-filename-patterns strings: A comma-separated
# list of regular expressions specifying filenames to ignore.
# -o, --output string: The format of the output of this script.
# Options are: [stdout json tap].

# We are skipping validation for the files that
# matches our regexp pattern (i.e. kustom, patch, clusterctl). 

if [ "${IS_CONTAINER}" != "false" ]; then
    kubeval --strict --ignore-missing-schemas \
    -d config,examples  -i kustom,patch,clusterctl -o tap
else
  "${CONTAINER_RUNTIME}" run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/capm3:ro,z" \
    --entrypoint sh \
    --workdir /capm3 \
    garethr/kubeval:latest \
    /capm3/hack/manifestlint.sh "${@}"
fi;