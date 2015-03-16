#!/bin/bash

set -e

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

mkdir -p "${OS_ROOT}/_output"
cd "${OS_ROOT}/_output"

if [ -d etcd ]; then
    pushd etcd >/dev/null
    ./build
    popd >/dev/null
    exit
fi

etcd_version=$(go run ${OS_ROOT}/hack/version.go ${OS_ROOT}/Godeps/Godeps.json \
  github.com/coreos/etcd/server)

mkdir -p etcd && cd etcd

curl -s -L https://github.com/coreos/etcd/tarball/${etcd_version} | \
  tar xz --strip-components 1 2>/dev/null

if [ "$?" != "0" ]; then
  echo "Failed to download coreos/etcd." && exit 1
fi

./build
