#!/bin/bash

set -e

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

etcd_version=$(go run ${OS_ROOT}/hack/version.go ${OS_ROOT}/Godeps/Godeps.json github.com/coreos/etcd/etcdserver)

mkdir -p "${OS_ROOT}/_tools"
cd "${OS_ROOT}/_tools"

if [ ! -d etcd ]; then
  mkdir -p etcd
  pushd etcd >/dev/null

  curl -s -L https://github.com/coreos/etcd/tarball/${etcd_version} | \
    tar xz --strip-components 1 2>/dev/null

  if [ "$?" != "0" ]; then
    echo "Failed to download coreos/etcd."
    exit 1
  fi
else
  pushd etcd >/dev/null
fi

./build

echo
echo Installed coreos/etcd ${etcd_version} into:
echo export PATH=$(pwd):\$PATH

popd >/dev/null
exit 0