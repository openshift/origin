#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# The OpenShift Docker registry and router are installed.
# It will run all tests that are imported into test/extended.
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"

os::util::environment::use_sudo
os::cleanup::tmpdir
os::util::environment::setup_all_server_vars

function cleanup() {
	return_code=$?
	os::test::junit::generate_report
	os::cleanup::all
	os::util::describe_return_code "${return_code}"
	exit "${return_code}"
}
trap "cleanup" EXIT

os::log::info "Starting server as distinct processes"
os::util::ensure::iptables_privileges_exist
os::start::configure_server

os::log::info "`openshift version`"
os::log::info "Server logs will be at:    ${LOG_DIR}/openshift.log"
os::log::info "Test artifacts will be in: ${ARTIFACT_DIR}"
os::log::info "Volumes dir is:            ${VOLUME_DIR}"
os::log::info "Config dir is:             ${SERVER_CONFIG_DIR}"
os::log::info "Using images:              ${USE_IMAGES}"
os::log::info "MasterIP is:               ${MASTER_ADDR}"

mkdir -p ${LOG_DIR}

os::log::info "Scan of OpenShift related processes already up via ps -ef	| grep openshift : "
ps -ef | grep openshift

os::test::junit::declare_suite_start "extended/alternate_launches"

os::log::info "Starting etcdserver"
sudo env "PATH=${PATH}" OPENSHIFT_ON_PANIC=crash openshift start etcd \
 --config=${MASTER_CONFIG_DIR}/master-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-etcdserver.log" &

os::log::info "Starting api server"
sudo env "PATH=${PATH}" OPENSHIFT_PROFILE=web OPENSHIFT_ON_PANIC=crash openshift start master api \
 --config=${MASTER_CONFIG_DIR}/master-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-apiserver.log" &

os::cmd::try_until_text "oc get --raw /healthz --as system:unauthenticated --config='${MASTER_CONFIG_DIR}/admin.kubeconfig'" 'ok' $(( 80 * second )) 0.25
os::cmd::try_until_text "oc get --raw /healthz/ready --as system:unauthenticated --config='${MASTER_CONFIG_DIR}/admin.kubeconfig'" 'ok' $(( 80 * second )) 0.25
os::log::info "OpenShift API server up at: "
date

# test alternate node level launches
os::log::info "Testing alternate node configurations"

# proxy only
sudo env "PATH=${PATH}" TEST_CALL=1 OPENSHIFT_ON_PANIC=crash openshift start network --enable=proxy \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-network-1.log" &
OS_PID=$!
os::cmd::try_until_text 'cat ${LOG_DIR}/os-network-1.log' 'syncProxyRules took'
pgrep -P "${OS_PID}" | xargs -r sudo kill
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-network-1.log' 'Starting node networking'
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-network-1.log' 'Started Kubernetes Proxy on'

# proxy only
sudo env "PATH=${PATH}" TEST_CALL=1 OPENSHIFT_ON_PANIC=crash openshift start node --enable=proxy \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-node-1.log" &
OS_PID=$!
os::cmd::try_until_text 'cat ${LOG_DIR}/os-node-1.log' 'syncProxyRules took'
pgrep -P "${OS_PID}" | xargs -r sudo kill
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-node-1.log' 'Starting node networking'
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-node-1.log' 'Started Kubernetes Proxy on'

# plugins only
sudo env "PATH=${PATH}" TEST_CALL=1 OPENSHIFT_ON_PANIC=crash openshift start network --enable=plugins \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-network-2.log" &
OS_PID=$!
os::cmd::try_until_text 'cat ${LOG_DIR}/os-network-2.log' 'Connecting to API server'
pgrep -P "${OS_PID}" | xargs -r sudo kill
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-network-2.log' 'Starting node networking'
os::cmd::expect_success_and_not_text 'cat ${LOG_DIR}/os-network-2.log' 'Started Kubernetes Proxy on'

# plugins only
sudo env "PATH=${PATH}" TEST_CALL=1 OPENSHIFT_ON_PANIC=crash openshift start node --enable=plugins \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-node-2.log" &
OS_PID=$!
os::cmd::try_until_text 'cat ${LOG_DIR}/os-node-2.log' 'Connecting to API server'
pgrep -P "${OS_PID}" | xargs -r sudo kill
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-node-2.log' 'Starting node networking'
os::cmd::expect_success_and_not_text 'cat ${LOG_DIR}/os-node-2.log' 'Started Kubernetes Proxy on'

# kubelet only
sudo env "PATH=${PATH}" TEST_CALL=1 OPENSHIFT_ON_PANIC=crash openshift start node --enable=kubelet \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-node-3.log" &
OS_PID=$!
os::cmd::try_until_text 'cat ${LOG_DIR}/os-node-3.log' 'Started kubelet'
pgrep -P "${OS_PID}" | xargs -r sudo kill
os::cmd::expect_success_and_text 'cat ${LOG_DIR}/os-node-3.log' 'Starting node'
os::cmd::expect_success_and_not_text 'cat ${LOG_DIR}/os-node-3.log' 'Starting node networking'
os::cmd::expect_success_and_not_text 'cat ${LOG_DIR}/os-node-3.log' 'Started Kubernetes Proxy on'


os::log::info "Starting controllers"
sudo env "PATH=${PATH}"  OPENSHIFT_ON_PANIC=crash openshift start master controllers \
 --config=${MASTER_CONFIG_DIR}/master-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-controllers.log" &

os::log::info "Starting node"
sudo env "PATH=${PATH}"  OPENSHIFT_ON_PANIC=crash openshift start node \
 --enable=kubelet,plugins,proxy,dns \
 --config=${NODE_CONFIG_DIR}/node-config.yaml \
 --loglevel=4 \
&>"${LOG_DIR}/os-node.log" &
export OS_PID=$!

os::log::info "OpenShift server start at: "
date

os::cmd::try_until_text "oc get --raw ${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz --as system:unauthenticated --config='${MASTER_CONFIG_DIR}/admin.kubeconfig'" 'ok' minute 0.5
os::cmd::try_until_success "oc get --raw /api/v1/nodes/${KUBELET_HOST} --config='${MASTER_CONFIG_DIR}/admin.kubeconfig'" $(( 80 * second )) 0.25
os::log::info "OpenShift node health checks done at: "
date

# set our default KUBECONFIG location
export KUBECONFIG="${ADMIN_KUBECONFIG}"

# TODO this is copy/paste from hack/test-end-to-end.sh. We need to DRY
if [[ -n "${USE_IMAGES:-}" ]]; then
  readonly JQSETPULLPOLICY='(.items[] | select(.kind == "DeploymentConfig") | .spec.template.spec.containers[0].imagePullPolicy) |= "IfNotPresent"'
  os::cmd::expect_success "oc adm registry --dry-run -o json --images='$USE_IMAGES' | jq '$JQSETPULLPOLICY' | oc create -f -"
else
  os::cmd::expect_success "oc adm registry"
fi
os::cmd::expect_success 'oc adm policy add-scc-to-user hostnetwork -z router'
os::cmd::expect_success 'oc adm router'

os::test::junit::declare_suite_end

${OS_ROOT}/test/end-to-end/core.sh
