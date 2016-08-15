#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# The OpenShift Docker registry and router are installed.
# It will run all tests that are imported into test/extended.
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"

os::util::environment::setup_all_server_vars "test-extended-alternate-launches/"
reset_tmp_dir

export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"

function cleanup()
{
	out=$?
  pgrep -f "openshift" | xargs -r sudo kill
	cleanup_openshift
	echo "[INFO] Exiting"
	exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT


echo "[INFO] Starting server as distinct processes"
ensure_iptables_or_die
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

# test alternate node level launches
echo "[INFO] Testing alternate node configurations"

# proxy only
sudo env "PATH=${PATH}" TEST_CALL=1 OPENSHIFT_ON_PANIC=crash openshift start network --enable=proxy \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-network-1.log" &
os::cmd::try_until_text 'cat ${LOG_DIR}/os-network-1.log' 'syncProxyRules took'
pgrep -f "TEST_CALL=1" | xargs -r sudo kill
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-network-1.log' 'Starting node networking'
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-network-1.log' 'Started Kubernetes Proxy on'

# proxy only
sudo env "PATH=${PATH}" TEST_CALL=1 OPENSHIFT_ON_PANIC=crash openshift start node --enable=proxy \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-node-1.log" &
os::cmd::try_until_text 'cat ${LOG_DIR}/os-node-1.log' 'syncProxyRules took'
pgrep -f "TEST_CALL=1" | xargs -r sudo kill
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-node-1.log' 'Starting node networking'
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-node-1.log' 'Started Kubernetes Proxy on'

# plugins only
sudo env "PATH=${PATH}" TEST_CALL=1 OPENSHIFT_ON_PANIC=crash openshift start network --enable=plugins \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-network-2.log" &
os::cmd::try_until_text 'cat ${LOG_DIR}/os-network-2.log' 'Connecting to API server'
pgrep -f "TEST_CALL=1" | xargs -r sudo kill
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-network-2.log' 'Starting node networking'
os::cmd::expect_success_and_not_text 'cat ${LOG_DIR}/os-network-2.log' 'Started Kubernetes Proxy on'

# plugins only
sudo env "PATH=${PATH}" TEST_CALL=1 OPENSHIFT_ON_PANIC=crash openshift start node --enable=plugins \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-node-2.log" &
os::cmd::try_until_text 'cat ${LOG_DIR}/os-node-2.log' 'Connecting to API server'
pgrep -f "TEST_CALL=1" | xargs -r sudo kill
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-node-2.log' 'Starting node networking'
os::cmd::expect_success_and_not_text 'cat ${LOG_DIR}/os-node-2.log' 'Started Kubernetes Proxy on'

# kubelet only
sudo env "PATH=${PATH}" TEST_CALL=1 OPENSHIFT_ON_PANIC=crash openshift start node --enable=kubelet \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-node-3.log" &
os::cmd::try_until_text 'cat ${LOG_DIR}/os-node-3.log' 'Started kubelet'
pgrep -f "TEST_CALL=1" | xargs -r sudo kill
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-node-3.log' 'Starting node'
os::cmd::expect_success_and_not_text 'cat ${LOG_DIR}/os-node-3.log' 'Starting node networking'
os::cmd::expect_success_and_not_text 'cat ${LOG_DIR}/os-node-3.log' 'Started Kubernetes Proxy on'


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
