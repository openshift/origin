#!/bin/bash

# This script starts standalone etcd instance and the OpenShift master API
# server with a default configuration with overriden controllerLeaseTTL.
# Controllers need to be started and managed by go test suite.

set -o errexit
set -o nounset
set -o pipefail

CONTROLLER_LEASE_TTL=10

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/common.sh"
os::log::install_errexit

ensure_ginkgo_or_die
ensure_iptables_or_die

os::build::setup_env
go test -c ./test/extended/ha -o ${OS_OUTPUT_BINPATH}/ha.test

function cleanup()
{
	out=$?
	cleanup_openshift
	echo "[INFO] Exiting"
	exit $out
}

echo "[INFO] Starting 'ha' extended tests"

trap "exit" INT TERM
trap "cleanup" EXIT

export TMPDIR="${TMPDIR:-"/tmp"}"
export BASETMPDIR="${TMPDIR}/openshift-extended-tests/ha"
setup_env_vars
export MASTER_CONFIG_PATH="${MASTER_CONFIG_DIR}/master-config.yaml"

# start_os_api_server starts standalone OS's API. Node and Controllers need to be started
# separately. Useful for testing controllers election.
function start_os_api_server {
	echo "[INFO] `openshift version`"
	echo "[INFO] Server logs will be at:    ${LOG_DIR}/openshift.log"
	echo "[INFO] Test artifacts will be in: ${ARTIFACT_DIR}"
	echo "[INFO] Volumes dir is:            ${VOLUME_DIR}"
	echo "[INFO] Config dir is:             ${SERVER_CONFIG_DIR}"
	echo "[INFO] Using images:              ${USE_IMAGES}"
	echo "[INFO] MasterIP is:               ${MASTER_ADDR}"

	mkdir -p ${LOG_DIR}

	echo "[INFO] Scan of OpenShift related processes already up via ps -ef | grep openshift : "
	ps -ef | grep openshift ||:
	echo "[INFO] Starting OpenShift API server"
	sudo env "PATH=${PATH}" OPENSHIFT_PROFILE=web OPENSHIFT_ON_PANIC=crash \
	 openshift start master api \
	 --config=${MASTER_CONFIG_PATH} \
	 --loglevel=4 \
	&> "${LOG_DIR}/openshift-api.log" &
	export OS_API_PID=$!

	echo "[INFO] OpenShift API server start at: "
	echo `date`
}

function start_os_node {
	echo "[INFO] Starting OpenShift node"
	sudo env "PATH=${PATH}" OPENSHIFT_ON_PANIC=crash \
	 openshift start node \
	 --config=${NODE_CONFIG_DIR}/node-config.yaml \
	 --loglevel=4 \
	&> "${LOG_DIR}/openshift-node.log" &
	export OS_NODE_PID=$!

	echo "[INFO] OpenShift node start at:"
	echo `date`

	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/nodes/${KUBELET_HOST}" "apiserver(nodes): " 0.25 80
}

reset_tmp_dir

configure_os_server
sed -i 's/\("\?\<controllerLeaseTTL"\?\s*:\s*\)[0-9]\+/\1'"$CONTROLLER_LEASE_TTL/" \
	$MASTER_CONFIG_PATH

# Start standalone etcd server
export ETCD_CERT_FILE="${MASTER_CONFIG_DIR}/etcd.server.crt"
export ETCD_KEY_FILE="${MASTER_CONFIG_DIR}/etcd.server.key"
export ETCD_TRUSTED_CA_FILE="${MASTER_CONFIG_DIR}/ca.crt"
export ETCD_HOST="${API_HOST}"
start_etcd 

# Controllers will be started by extended tests
start_os_api_server
start_os_node

# Run the tests
pushd ${OS_ROOT}/test/extended >/dev/null
export KUBECONFIG="${ADMIN_KUBECONFIG}"
export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"
# Skip e2e tests. Focus only on ha tests.
TMPDIR=${BASETMPDIR} ginkgo -progress -stream -v -focus="ha:" "$@" ${OS_OUTPUT_BINPATH}/ha.test
popd >/dev/null
