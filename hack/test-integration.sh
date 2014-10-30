#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

function cleanup()
{
    set +e
    kill ${ETCD_PID} 1>&2 2>/dev/null
    rm -rf ${ETCD_DIR} 1>&2 2>/dev/null
    echo
    echo "Complete"
}

start_etcd
trap cleanup EXIT SIGINT

echo
echo Integration test cases ...
echo
"${OS_ROOT}/hack/test-go.sh" test/integration -tags 'integration no-docker' "${@:1}"
