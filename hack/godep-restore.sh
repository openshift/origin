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

#### HACK ####
# Sometimes godep just can't handle things. This lets use manually put
# some deps in place first, so godep won't fall over.
preload-dep() {
  org="$1"
  project="$2"
  sha="$3"

  # Go get stinks, which is why we have the || true...
  go get -d "${org}/${project}" >/dev/null 2>&1 || true
  repo_dir="${GOPATH}/src/${org}/${project}"
  pushd "${repo_dir}" > /dev/null
    git remote update > /dev/null
    git checkout "${sha}" > /dev/null
  popd > /dev/null
}

# Sometimes godep needs 'other' remotes. So add those remotes
preload-remote() {
  local orig_org="$1"
  local orig_project="$2"
  local alt_org="$3"
  local alt_project="$4"

  # Go get stinks, which is why we have the || true...
  go get -d "${orig_org}/${orig_project}" &>/dev/null || true

  repo_dir="${GOPATH}/src/${orig_org}/${orig_project}"
  pushd "${repo_dir}" > /dev/null
    git remote add "${alt_org}-remote" "https://${alt_org}/${alt_project}.git" > /dev/null || true
    git remote update > /dev/null
  popd > /dev/null
}

pin-godep() {
  pushd "${GOPATH}/src/github.com/tools/godep" > /dev/null
    git checkout "$1"
    "${GODEP}" go install
  popd > /dev/null
}

ORIGIN_ROOT=$(dirname "${BASH_SOURCE}")/..

# build the godep tool
# Again go get stinks, hence || true
go get -u github.com/tools/godep 2>/dev/null || true
GODEP="${GOPATH}/bin/godep"

# Use to following if we ever need to pin godep to a specific version again
pin-godep 'v63'

# preload any odd-ball remotes
preload-remote "github.com/openshift" "origin" "github.com/openshift" "origin" # this looks goofy, but if you are not in GOPATH you need to pull origin explicitly
preload-remote "k8s.io" "kubernetes" "github.com/openshift" "kubernetes"

# preload any odd-ball commits
# kube e2e test dep
preload-dep "github.com/elazarl" "goproxy" "07b16b6e30fcac0ad8c0435548e743bcf2ca7e92"
# rkt test dep
preload-dep "github.com/golang/mock" "gomock" "bd3c8e81be01eef76d4b503f5e687d2d1354d2d9"

# HACK. PLEASE REMOVE WHEN THIS DEPENDENCY IS FIXED.
OLDINFPATH=${GOPATH}/src/speter.net/go/exp/math/dec
mkdir -p "${OLDINFPATH}"
pushd "${OLDINFPATH}" > /dev/null
  git clone https://github.com/go-inf/inf.git || true
  pushd inf > /dev/null
    git checkout 42ca6cd68aa922bc3f32f1e056e61b65945d9ad7
  popd > /dev/null
popd > /dev/null

# fill out that nice clean place with the origin godeps
echo "Starting to download all godeps. This takes a while"

pushd "${ORIGIN_ROOT}" > /dev/null
"${GODEP}" restore
popd > /dev/null

echo "Download finished"
echo "######## REMEMBER ########"
echo "You cannot just godep save ./..."
echo "You should use hack/godep-save.sh"
echo "hack/godep-save.sh forces the inclusion of packages that godep alone will not include"
echo "Watch out for UPSTREAM patches when updating deps"
