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

# Enable tracing in this script off by setting the TRACE variable in your
# environment to any value:
#
# $ TRACE=1 test.sh
TRACE=${TRACE:-""}
if [[ -n "${TRACE}" ]]; then
  set -x
fi

k8s_version=1.23.3
arch=amd64
os="unknown"

if [[ "${OSTYPE}" == "linux"* ]]; then
  os="linux"
elif [[ "${OSTYPE}" == "darwin"* ]]; then
  os="darwin"
fi

if [[ "$os" == "unknown" ]]; then
  echo "OS '$OSTYPE' not supported. Aborting." >&2
  exit 1
fi

# Turn colors in this script off by setting the NO_COLOR variable in your
# environment to any value:
#
# $ NO_COLOR=1 test.sh
NO_COLOR=${NO_COLOR:-""}
if [[ -z "${NO_COLOR}" ]]; then
  header=$'\e[1;33m'
  reset=$'\e[0m'
else
  header=''
  reset=''
fi

function header_text {
  echo "$header$*$reset"
}

kb_root_dir="/tmp/kubebuilder"

# Skip fetching and untaring the tools by setting the SKIP_FETCH_TOOLS variable
# in your environment to any value:
#
# $ SKIP_FETCH_TOOLS=1 ./fetch_ext_bins.sh
#
# If you skip fetching tools, this script will use the tools already on your
# machine, but rebuild the kubebuilder and kubebuilder-bin binaries.
SKIP_FETCH_TOOLS=${SKIP_FETCH_TOOLS:-""}

# Download the tarball containing  etcd, k8s API server and
# kubelet binaries and store them under kb_root_dir/bin.
function fetch_tools {
  if [[ -n "$SKIP_FETCH_TOOLS" ]]; then
    return 0
  fi

  mkdir -p "${kb_root_dir}"
  header_text "fetching binaries"
  kb_tools_archive_name="envtest-bins.tar.gz"
  kb_tools_download_url=https://go.kubebuilder.io/test-tools/"${k8s_version}"/"${os}"/"${arch}"
  kb_tools_archive_path="/tmp/${kb_tools_archive_name}"

  if [[ ! -f ${kb_tools_archive_path} ]]; then
    curl -sSLo "${kb_tools_archive_path}" "${kb_tools_download_url}"
  fi
  tar -C "${kb_root_dir}/" --strip-components=1 -zvxf "${kb_tools_archive_path}"
  rm "${kb_tools_archive_path}"
}

function setup_envs {
  header_text "setting up env vars"
  # Export binaries path"
  export PATH="${kb_root_dir}/bin:$PATH"
  export SKIP_FETCH_TOOLS=1
  export KUBEBUILDER_ASSETS="${kb_root_dir}/bin/"
}
