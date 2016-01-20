#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# The OpenShift Docker registry and router are installed.
# It will run all tests that are imported into test/extended.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/common.sh"
os::log::install_errexit
cd "${OS_ROOT}"

os::build::setup_env

export TMPDIR="${TMPDIR:-"/tmp"}"
export BASETMPDIR="${TMPDIR}/openshift-extended-tests/alternate_launches"
export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"

function cleanup()
{
	out=$?
	cleanup_openshift
	echo "[INFO] Exiting"
	exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT


echo "[INFO] Starting server as distinct processes"
ensure_iptables_or_die
setup_env_vars
reset_tmp_dir
configure_os_server

echo "[INFO] `openshift version`"
echo "[INFO] Server logs will be at:    ${LOG_DIR}/openshift.log"
echo "[INFO] Test artifacts will be in: ${ARTIFACT_DIR}"
echo "[INFO] Volumes dir is:            ${VOLUME_DIR}"
echo "[INFO] Config dir is:             ${SERVER_CONFIG_DIR}"
echo "[INFO] Using images:              ${USE_IMAGES}"
echo "[INFO] MasterIP is:               ${MASTER_ADDR}"

mkdir -p ${LOG_DIR}

echo "[INFO] Scan of OpenShift related processes already up via ps -ef	| grep openshift : "
ps -ef | grep openshift
echo "[INFO] Starting etcdserver"
sudo env "PATH=${PATH}" OPENSHIFT_ON_PANIC=crash openshift start etcd \
 --config=${MASTER_CONFIG_DIR}/master-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-etcdserver.log" &

echo "[INFO] Starting api server"
sudo env "PATH=${PATH}" OPENSHIFT_PROFILE=web OPENSHIFT_ON_PANIC=crash openshift start master api \
 --config=${MASTER_CONFIG_DIR}/master-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-apiserver.log" &

wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
echo "[INFO] OpenShift API server up at: "
date

echo "[INFO] Starting controllers"
sudo env "PATH=${PATH}"  OPENSHIFT_ON_PANIC=crash openshift start master controllers \
 --config=${MASTER_CONFIG_DIR}/master-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-controllers.log" &

echo "[INFO] Starting node"
sudo env "PATH=${PATH}"  OPENSHIFT_ON_PANIC=crash openshift start node \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-node.log" &
export OS_PID=$!

echo "[INFO] OpenShift server start at: "
date

wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "[INFO] kubelet: " 0.5 60
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/nodes/${KUBELET_HOST}" "apiserver(nodes): " 0.25 80
echo "[INFO] OpenShift node health checks done at: "
date

# set our default KUBECONFIG location
export KUBECONFIG="${ADMIN_KUBECONFIG}"

${OS_ROOT}/test/end-to-end/core.sh
