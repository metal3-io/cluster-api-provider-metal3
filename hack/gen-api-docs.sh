#!/usr/bin/env bash
# Generates the API reference documentation in docs/api-reference.md from the
# CRDs using crdoc. The documented API versions and kinds are selected by
# hack/api-reference-toc.yaml. Run through "make generate-api-docs".
#
# Expected env vars (with defaults):
#   CRDOC    - path to the crdoc binary
#   CRD_ROOT - directory containing the generated CRDs

set -o errexit -o nounset -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CRDOC="${CRDOC:-crdoc}"
CRD_ROOT="${CRD_ROOT:-${REPO_ROOT}/config/crd/bases}"
API_REFERENCE="${REPO_ROOT}/docs/api-reference.md"

# The descriptions of the embedded object metadata come from upstream and link
# to Kubernetes user guide pages that no longer exist, so point them to the
# current pages to keep the generated document free of broken links. The
# metadata link is also pinned to an old Kubernetes API version by crdoc, so
# move it to the Kubernetes version used by this repository.
#
# crdoc puts the descriptions into HTML table cells without escaping them, so
# the placeholders in the providerID format would be rendered as HTML tags and
# vanish. Escape them explicitly.
"${CRDOC}" \
    --resources "${CRD_ROOT}" \
    --toc "${REPO_ROOT}/hack/api-reference-toc.yaml" \
    --output /dev/stdout |
    sed -E \
        -e 's#http://kubernetes.io/docs/user-guide/labels#https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#g' \
        -e 's#http://kubernetes.io/docs/user-guide/annotations#https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/#g' \
        -e 's#/kubernetes-api/v1\.27/#/kubernetes-api/v1.37/#g' \
        -e 's#<(namespace|bmh-name|m3m-name|bmh-uuid)>#\&lt;\1\&gt;#g' \
    > "${API_REFERENCE}"
