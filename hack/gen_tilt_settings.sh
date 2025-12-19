#!/usr/bin/env bash
# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

get_latest_release() {
    set +x
    if [ -z "${GITHUB_TOKEN:-}" ]; then
        release="$(curl -sL "$1")" || {
            set -x && exit 1
        }
    else
        release="$(curl -H "Authorization: token ${GITHUB_TOKEN}" -sL "$1")" || {
            set -x && exit 1
        }
    fi
    # This gets the latest release as vx.y.z or vx.y.z-rc.0, including any version with a suffix starting with - , for example -rc.0
    # This relies on the order of the Github release page + select filter
    release_tag="$(echo "${release}" | jq -r "[.[].tag_name | select( startswith(\"${2:-}\"))] | .[0]")"

    if [[ "${release_tag}" == "null" ]]; then
        set -x
        exit 1
    fi
    set -x
    # shellcheck disable=SC2005
    echo "${release_tag}"
}

CAPIRELEASEPATH="${CAPIRELEASEPATH:-https://api.github.com/repos/${CAPI_BASE_URL:-kubernetes-sigs/cluster-api}/releases}"
export CAPIRELEASE="${CAPIRELEASE:-$(get_latest_release "${CAPIRELEASEPATH}" "v1.12.")}"

# ClusterClass enable flag
if [ -n "${CLUSTER_TOPOLOGY:-}" ]; then
cat <<EOF >tilt-settings.json
{
    "capi_version": "${CAPIRELEASE}",
    "cert_manager_version": "v1.12.3",
    "kubernetes_version": "${KUBERNETES_VERSION:-v1.34.0}",
    "extra_args": {
        "feature_gates": {
            "ClusterTopology": "true"
        }
    }
}
EOF
else
cat <<EOF >tilt-settings.json
{
    "capi_version": "${CAPIRELEASE}",
    "cert_manager_version": "v1.12.3",
    "kubernetes_version": "${KUBERNETES_VERSION:-v1.34.0}"
}
EOF
fi
