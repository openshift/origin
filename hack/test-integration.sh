#!/bin/bash

set -e

source $(dirname $0)/util.sh

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
$(dirname $0)/../hack/test-go.sh test/integration -tags 'integration no-docker' "${@:1}"
