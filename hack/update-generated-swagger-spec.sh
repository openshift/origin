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
SWAGGER_SPEC_REL_DIR=${1:-""}
SWAGGER_SPEC_OUT_DIR="${OS_ROOT}/${SWAGGER_SPEC_REL_DIR}/api/swagger-spec"
mkdir -p "${SWAGGER_SPEC_OUT_DIR}" || echo $? > /dev/null
SWAGGER_API_PATH="${HOST}/swaggerapi/"

# Prevent user environment from colliding with the test setup
unset KUBECONFIG

# set path so OpenShift is available
GO_OUT="${OS_ROOT}/_output/local/go/bin"
export PATH="${GO_OUT}:${PATH}"

# create temp dir
TEMP_DIR=${USE_TEMP:-$(mktemp -d /tmp/openshift-cmd.XXXX)}
export CURL_CA_BUNDLE="${TEMP_DIR}/origin.local.config/master/ca.crt"

# Start openshift
echo "Starting OpenShift..."
pushd "${TEMP_DIR}" > /dev/null
OPENSHIFT_ON_PANIC=crash openshift start master --listen="https://0.0.0.0:8443" --master="https://127.0.0.1:8443" &> /dev/null &
OS_PID=$!
popd > /dev/null

wait_for_url "${HOST}/healthz" "apiserver: " 0.25 80

echo "Updating ${SWAGGER_SPEC_OUT_DIR}:"

ENDPOINT_TYPES="oapi api"
for type in $ENDPOINT_TYPES
do
    ENDPOINTS=$(curl "${HOST}" | grep -Po "(?<=\/${type}\/)[a-z0-9]+" | sed '/v1beta3/d')
    for endpoint in $ENDPOINTS
    do
        echo "Updating ${SWAGGER_SPEC_OUT_DIR}/${type}-${endpoint}.json from ${SWAGGER_API_PATH}${type}/${endpoint}..."
        curl "${SWAGGER_API_PATH}${type}/${endpoint}" > "${SWAGGER_SPEC_OUT_DIR}/${type}-${endpoint}.json"    
    done
done
echo "SUCCESS"
