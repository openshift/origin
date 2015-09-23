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

openshift=$(cd "${OS_ROOT}"; echo "$(pwd)/_output/local/bin/$(os::util::host_platform)/openshift")

if [[ ! -e "${openshift}" ]]; then
  {
    echo "It looks as if you don't have a compiled openshift binary"
    echo
    echo "If you are running from a clone of the git repo, please run"
    echo "'./hack/build-go.sh'."
  } >&2
  exit 1
fi

# create temp dir
TEMP_DIR=${USE_TEMP:-$(mktemp -d /tmp/openshift-cmd.XXXX)}
export CURL_CA_BUNDLE="${TEMP_DIR}/openshift.local.config/master/ca.crt"

# Start openshift
echo "Starting OpenShift..."
pushd "${TEMP_DIR}" > /dev/null
OPENSHIFT_ON_PANIC=crash "${openshift}" start master --master="https://127.0.0.1:8443" >/dev/null 2>&1  &
OS_PID=$!
popd > /dev/null

wait_for_url "${HOST}/healthz" "apiserver: " 0.25 80

echo "Updating ${SWAGGER_SPEC_OUT_DIR}:"

ENDPOINT_TYPES="oapi api"
for type in $ENDPOINT_TYPES
do
    ENDPOINTS=(v1)
    for endpoint in $ENDPOINTS
    do
        echo "Updating ${SWAGGER_SPEC_OUT_DIR}/${type}-${endpoint}.json from ${SWAGGER_API_PATH}${type}/${endpoint}..."
        curl -w "\n" "${SWAGGER_API_PATH}${type}/${endpoint}" > "${SWAGGER_SPEC_OUT_DIR}/${type}-${endpoint}.json"
    done
done
echo "SUCCESS"
