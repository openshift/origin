#!/bin/bash

# Copyright 2015 The Kubernetes Authors All rights reserved.
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

KUBE_ROOT=$(dirname "${BASH_SOURCE}")/../..
PATCH_ROOT=$(dirname "${BASH_SOURCE}")/patches

pin-godep() {
  pushd "${GOPATH}/src/github.com/tools/godep" > /dev/null
    git checkout "$1"
    "${GODEP}" go install
  popd > /dev/null
}

commit=false
while getopts ":c" opt; do
  case $opt in
    c) 
      commit=true
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      ;;
  esac
done

# build the godep tool
go get -u github.com/tools/godep 2>/dev/null
GODEP="${GOPATH}/bin/godep"

# Use to following if we ever need to pin godep to a specific version again
#pin-godep 'v53'

rm -rf Godeps
"${GODEP}" save ./...
if [ "$commit" = true ]; then
  git add --all
  git commit -a -m "Update Godeps to be clean comared to Godeps.json"
fi
for patch in $(ls ${PATCH_ROOT}); do
  echo "Applying patch: ${PATCH_ROOT}/${patch}"
  if [ "$commit" = true ]; then
    git am --whitespace=nowarn "${PATCH_ROOT}/${patch}"
  else
    git apply --whitespace=nowarn "${PATCH_ROOT}/${patch}"
  fi
done

# ex: ts=2 sw=2 et filetype=sh
