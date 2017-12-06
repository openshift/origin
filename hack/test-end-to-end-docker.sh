#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::log::info "Starting containerized end-to-end test"

unset KUBECONFIG

os::util::environment::use_sudo
os::cleanup::tmpdir
os::util::environment::setup_all_server_vars
os::util::environment::setup_time_vars
export HOME="${FAKE_HOME_DIR}"

function cleanup() {
	return_code=$?

	os::test::junit::generate_report
	os::cleanup::all

	os::util::describe_return_code "${return_code}"
	exit "${return_code}"
}
trap "cleanup" EXIT

os::log::system::start

# Setup
os::log::info "openshift version: `openshift version`"
os::log::info "oc version:        `oc version`"
os::log::info "Using images:							${USE_IMAGES}"

os::log::info "Starting OpenShift containerized server"
oc cluster up --server-loglevel=4 --version="${TAG}" \
        --host-data-dir="${VOLUME_DIR}/etcd" \
        --host-volumes-dir="${VOLUME_DIR}"

os::test::junit::declare_suite_start "setup/start-oc_cluster_up"
os::cmd::try_until_success "oc cluster status" "$((5*TIME_MIN))" "10"
os::test::junit::declare_suite_end

IMAGE_WORKING_DIR=/var/lib/origin
docker cp origin:${IMAGE_WORKING_DIR}/openshift.local.config ${BASETMPDIR}

export ADMIN_KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
export CLUSTER_ADMIN_CONTEXT=$(oc config view --config=${ADMIN_KUBECONFIG} --flatten -o template --template='{{index . "current-context"}}')
sudo chmod -R a+rwX "${ADMIN_KUBECONFIG}"
export KUBECONFIG="${ADMIN_KUBECONFIG}"
os::log::info "To debug: export KUBECONFIG=$ADMIN_KUBECONFIG"


${OS_ROOT}/test/end-to-end/core.sh
