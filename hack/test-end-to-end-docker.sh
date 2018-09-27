#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::log::info "Starting containerized end-to-end test"

# cluster up no longer produces a cluster that run the e2e test.  These use cases are already mostly covered
# in existing e2e suites.  The image-registry related tests stand out as ones that may not have an equivalent.
exit 0

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

# Tag the web console image with the same tag as the other origin images
docker pull openshift/origin-web-console:latest
docker tag openshift/origin-web-console:latest openshift/origin-web-console:${TAG}

# Setup
os::log::info "openshift version: `openshift version`"
os::log::info "oc version:        `oc version`"
os::log::info "Using images:							${USE_IMAGES}"

os::log::info "Starting OpenShift containerized server"
CLUSTERUP_DIR="${BASETMPDIR}"/cluster-up
mkdir "${CLUSTERUP_DIR}"
oc cluster up --server-loglevel=4 --tag="${TAG}" \
        --base-dir="${CLUSTERUP_DIR}" \
        --write-config
        
oc cluster up --server-loglevel=4 --tag="${TAG}" \
        --base-dir="${CLUSTERUP_DIR}"

MASTER_CONFIG_DIR="${CLUSTERUP_DIR}/kube-apiserver"

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
