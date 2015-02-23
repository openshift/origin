#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

os::build::setup_env

function cleanup()
{
    set +e
    kill ${ETCD_PID} 1>&2 2>/dev/null
    rm -rf ${ETCD_DIR} 1>&2 2>/dev/null
    echo
    echo "Complete"
}

package="${OS_TEST_PACKAGE:-test/integration}"
tags="${OS_TEST_TAGS:-integration no-docker}"

echo
echo "Test ${package} -tags='${tags}' ..."
echo

# setup the test dirs
testdir="${OS_ROOT}/_output/testbin/${package}"
name="$(basename ${testdir})"
testexec="${testdir}/${name}.test"
mkdir -p "${testdir}"

# build the test executable (cgo must be disabled to have the symbol table available)
pushd "${testdir}" 2>&1 >/dev/null
CGO_ENABLED=0 go test -c -tags="${tags}" "${OS_GO_PACKAGE}/${package}"
popd 2>&1 >/dev/null

start_etcd
trap cleanup EXIT SIGINT

# run each test as its own process
pushd "./${package}" 2>&1 >/dev/null
time go run "${OS_ROOT}/hack/listtests.go" -prefix="${OS_GO_PACKAGE}/${package}.Test" "${testdir}" | xargs -I {} -n 1 "${testexec}" -test.run="^{}$" "${@:1}"
popd 2>&1 >/dev/null
