#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::log::info "Starting containerized end-to-end test"

unset KUBECONFIG

os::util::environment::use_sudo
os::cleanup::tmpdir
os::util::environment::setup_all_server_vars
export HOME="${FAKE_HOME_DIR}"

# Allow setting $JUNIT_REPORT to toggle output behavior
if [[ -n "${JUNIT_REPORT:-}" ]]; then
	export JUNIT_REPORT_OUTPUT="${LOG_DIR}/raw_test_output.log"
fi

function cleanup()
{
	out=$?
	echo
	if [ $out -ne 0 ]; then
		echo "[FAIL] !!!!! Test Failed !!!!"
	else
		os::log::info "Test Succeeded"
	fi
	echo

	set +e
	os::cleanup::dump_container_logs

	# pull information out of the server log so that we can get failure management in jenkins to highlight it and
	# really have it smack people in their logs.  This is a severe correctness problem
    grep -ra5 "CACHE.*ALTERED" ${LOG_DIR}/containers

	os::cleanup::dump_etcd

	os::cleanup::dump_events

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		os::cleanup::containers
	fi

	truncate_large_logs
	os::test::junit::generate_oscmd_report
	set -e

	os::log::info "Exiting"
	ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"
	exit $out
}

trap "cleanup" EXIT INT TERM

os::log::system::start

# Setup
os::log::info "openshift version: `openshift version`"
os::log::info "oc version:        `oc version`"
os::log::info "Using images:							${USE_IMAGES}"

os::log::info "Starting OpenShift containerized server"
oc cluster up --server-loglevel=4 --version="${TAG}" \
        --host-data-dir="${VOLUME_DIR}/etcd" \
        --host-volumes-dir="${VOLUME_DIR}"

oc cluster status

IMAGE_WORKING_DIR=/var/lib/origin
docker cp origin:${IMAGE_WORKING_DIR}/openshift.local.config ${BASETMPDIR}

export ADMIN_KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
export CLUSTER_ADMIN_CONTEXT=$(oc config view --config=${ADMIN_KUBECONFIG} --flatten -o template --template='{{index . "current-context"}}')
sudo chmod -R a+rwX "${ADMIN_KUBECONFIG}"
export KUBECONFIG="${ADMIN_KUBECONFIG}"
os::log::info "To debug: export KUBECONFIG=$ADMIN_KUBECONFIG"


${OS_ROOT}/test/end-to-end/core.sh
