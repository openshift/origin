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
    cleanup_openshift

    if [ $out -ne 0 ]; then
        echo "[FAIL] !!!!! Generate Failed !!!!"
        echo
        tail -100 "${LOG_DIR}/openshift.log"
        echo
        echo -------------------------------------
        echo
    fi
    exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

set -e


TMPDIR="${TMPDIR:-"/tmp"}"
BASETMPDIR="${TMPDIR}/openshift-swagger"
export ALL_IP_ADDRESSES=127.0.0.1
export SERVER_HOSTNAME_LIST=127.0.0.1
export API_BIND_HOST=127.0.0.1
export API_PORT=38443
export ETCD_PORT=34001
export ETCD_PEER_PORT=37001
export SUDO=''
setup_env_vars
reset_tmp_dir
configure_os_server


SWAGGER_SPEC_REL_DIR=${1:-""}
SWAGGER_SPEC_OUT_DIR="${OS_ROOT}/${SWAGGER_SPEC_REL_DIR}/api/swagger-spec"
mkdir -p "${SWAGGER_SPEC_OUT_DIR}" || true
SWAGGER_API_PATH="${MASTER_ADDR}/swaggerapi/"

# Start openshift
start_os_master

echo "Updating ${SWAGGER_SPEC_OUT_DIR}:"

ENDPOINT_TYPES="oapi api"
for type in $ENDPOINT_TYPES
do
    ENDPOINTS=(v1)
    for endpoint in $ENDPOINTS
    do
        echo "Updating ${SWAGGER_SPEC_OUT_DIR}/${type}-${endpoint}.json from ${SWAGGER_API_PATH}${type}/${endpoint}..."
        curl -w "\n" "${SWAGGER_API_PATH}${type}/${endpoint}" > "${SWAGGER_SPEC_OUT_DIR}/${type}-${endpoint}.json"

        os::util::sed 's|https://127.0.0.1:38443|https://127.0.0.1:8443|g' "${SWAGGER_SPEC_OUT_DIR}/${type}-${endpoint}.json"
    done
done
echo "SUCCESS"
