#!/bin/bash

set -e

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

mkdir -p "${OS_ROOT}/_output"
cd "${OS_ROOT}/_output"

if [ -d etcd ]; then
    cd etcd
    git fetch origin
else
    git clone https://github.com/coreos/etcd.git
    cd etcd
fi

git checkout $(go run ${OS_ROOT}/hack/version.go ${OS_ROOT}/Godeps/Godeps.json github.com/coreos/etcd/server)
./build

