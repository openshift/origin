#!/bin/bash
# Copyright 2016 The Kubernetes Authors.
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

set -o nounset
set -o errexit

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
. "${ROOT}/contrib/hack/utilities.sh" || { echo 'Cannot load bash utilities.'; exit 1; }

GO_VERSION='1.9'
HELM_VERSION='v2.7.0'
GLIDE_VERSION='v0.12.3'

function update-golang() {
  # Check version of golang
  local current="$(go version 2>/dev/null || echo "unknown")"

  # go version prints its output in the format:
  #   go version go1.7.3 <os>/<arch>
  # To isolate the version, we include the leading 'go' and trailing
  # space in the comparison.
  if [[ "${current}" == *"go${GO_VERSION} "* ]]; then
    echo "Golang is up-to-date: ${current}"
  else
    echo "Upgrading golang ${current} to ${GO_VERSION}"

    # Install new golang.
    local golang_url='https://storage.googleapis.com/golang'
    rm -rf /usr/local/go \
      && curl -sSL "${golang_url}/go${GO_VERSION}.linux-amd64.tar.gz" \
         | tar -C /usr/local -xz \
      || { echo "Cannot upgrade golang to ${GO_VERSION}"; return 1; }
  fi
}


function update-helm() {
  # Check version of Helm
  local current="$(helm version --client 2>/dev/null || echo "unknown")"

  # helm version prints its output in the format:
  #   Client: &version.Version{SemVer:"v2.0.0", GitCommit:"...", ... }
  # To isolate the version string, we include the surrounding quotes
  # in the comparison.
  if [[ "${current}" == *"\"${HELM_VERSION}\""* ]]; then
    echo "Helm is up-to-date: ${current}"
  else
    echo "Upgrading Helm ${current} to ${HELM_VERSION}"

    # Install new Helm.
    local helm_url='https://storage.googleapis.com/kubernetes-helm'
    curl -sSL "${helm_url}/helm-${HELM_VERSION}-linux-amd64.tar.gz" \
        | tar -C /usr/local/bin -xz --strip-components=1 linux-amd64/helm \
      || { echo "Cannot upgrade helm to ${HELM_VERSION}"; return 1; }
  fi
}


function update-glide() {
  # Check version of glide
  local current="$(glide --version 2>/dev/null || echo "unknown")"

  # glide version prints its output in the format:
  #   glide version v0.12.3
  # To isolate the version string, we include the leading space
  # in the comparison, and ommit the trailing wildcard.
  if [[ "${current}" == *" ${GLIDE_VERSION}" ]]; then
    echo "Glide is up-to-date: ${current}"
  else
    echo "Upgrading Glide ${current} to ${GLIDE_VERSION}"

    # Install new Glide.
    local glide_url='https://github.com/Masterminds/glide/releases/download/'
    glide_url+="${GLIDE_VERSION}/glide-${GLIDE_VERSION}-linux-amd64.tar.gz"

    curl -sSL "${glide_url}" \
        | tar -C /usr/local/bin -xz --strip-components=1 linux-amd64/glide \
      || { echo "Cannot upgrade glide to ${GLIDE_VERSION}"; return 1; }
  fi
}


function main() {
  update-golang || error_exit 'Failed to update golang'
  update-helm   || error_exit 'Failed to update helm'
  update-glide  || error_exit 'Failed to update glide'
}

main
