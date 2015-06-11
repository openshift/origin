#!/bin/bash

# Script to create latest swagger spec.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

os::log::install_errexit

function cleanup()
{
    out=$?
    pkill -P $$
    rm -rf "${TEMP_DIR}"

    if [ $out -ne 0 ]; then
        echo "[FAIL] !!!!! Generate Failed !!!!"
        echo
        cat "${TEMP_DIR}/openshift.log"
        echo
        echo -------------------------------------
        echo
    fi
    exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

set -e

ADDR=127.0.0.1:8443
HOST=https://${ADDR}
SWAGGER_ROOT_DIR="${OS_ROOT}/api/swagger-spec"
mkdir -p "${SWAGGER_ROOT_DIR}"
SWAGGER_API_PATH="${HOST}/swaggerapi/"

# Prevent user environment from colliding with the test setup
unset KUBECONFIG

# set path so OpenShift is available
GO_OUT="${OS_ROOT}/_output/local/go/bin"
export PATH="${GO_OUT}:${PATH}"

# create temp dir
TEMP_DIR=${USE_TEMP:-$(mktemp -d /tmp/openshift-cmd.XXXX)}
export CURL_CA_BUNDLE="${TEMP_DIR}/openshift.local.config/master/ca.crt"

# Start openshift
pushd "${TEMP_DIR}" > /dev/null
OPENSHIFT_ON_PANIC=crash openshift start master --listen="https://0.0.0.0:8443" --master="https://127.0.0.1:8443" 1>&2 &
OS_PID=$!
popd > /dev/null

wait_for_url "${HOST}/healthz" "apiserver: " 0.25 80

echo "Updating ${SWAGGER_ROOT_DIR}"
set -x
curl "${SWAGGER_API_PATH}oapi/v1" > "${SWAGGER_ROOT_DIR}/oapi-v1.json"
curl "${SWAGGER_API_PATH}api/v1" > "${SWAGGER_ROOT_DIR}/api-v1.json"

echo "SUCCESS"