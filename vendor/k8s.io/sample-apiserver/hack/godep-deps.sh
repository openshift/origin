#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
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


# overall flow
# 1. make a clean gopath
# 2. get client-go
# 3. flatten client-go's vendored deps
# 4. go get extra deps we need
# 5. remove old vendoring data
# 6. vendor packages we need
# 7. copy new vendored packages and save them

set -o errexit
set -o nounset
set -o pipefail

goPath=$(mktemp -d "${TMPDIR:-/tmp/}$(basename 0).XXXXXXXXXXXX")
echo ${goPath}

export GOPATH=${goPath}

mkdir -p ${goPath}/src/k8s.io/sample-apiserver
cp -R . ${goPath}/src/k8s.io/sample-apiserver

pushd ${goPath}/src/k8s.io/sample-apiserver
rm -rf vendor || true

# restore what we have in our new manifest that we've sync
godep restore

# the manifest doesn't include any levels of k8s.io dependencies so load them using the go get
# assume you sync all the repos at the same time, the leves you get will be correct
go get -d ./... || true

# save the new levels of dependencies
rm -rf vendor || true
rm -rf Godeps || true
godep save ./...
popd

# remove the vendor dir we have and move the one we just made
rm -rf vendor || true
rm -rf Godeps || true
git rm -rf vendor || true
git rm -rf Godeps || true
mv ${goPath}/src/k8s.io/sample-apiserver/vendor .
mv ${goPath}/src/k8s.io/sample-apiserver/Godeps .
git add vendor
git add Godeps
git commit -m "sync: resync vendor folder"

