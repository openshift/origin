#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

readonly JQSETPULLPOLICY='(.items[] | select(.kind == "DeploymentConfig") | .spec.template.spec.containers[0].imagePullPolicy) |= "IfNotPresent"'

if [[ "${TEST_END_TO_END:-}" != "direct" ]]; then
	if docker version >/dev/null 2>&1; then
		echo "++ Docker is installed, running hack/test-end-to-end-docker.sh instead."
		"${OS_ROOT}/hack/test-end-to-end-docker.sh"
		exit $?
	fi
	echo "++ Docker is not installed, running end-to-end against local binaries"
fi

ensure_iptables_or_die

echo "[INFO] Starting end-to-end test"

function cleanup()
{
	out=$?
	echo
	if [ $out -ne 0 ]; then
		echo "[FAIL] !!!!! Test Failed !!!!"
	else
		echo "[INFO] Test Succeeded"
	fi
	echo

	cleanup_openshift
	echo "[INFO] Exiting"
	ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"
	exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT


# Start All-in-one server and wait for health
os::util::environment::setup_all_server_vars "test-end-to-end/"
os::util::environment::use_sudo
reset_tmp_dir

os::log::start_system_logger

configure_os_server
start_os_server

# set our default KUBECONFIG location
export KUBECONFIG="${ADMIN_KUBECONFIG}"

os::test::junit::declare_suite_start "end-to-end/startup"
if [[ -n "${USE_IMAGES:-}" ]]; then
    os::cmd::expect_success "oadm registry --dry-run -o json --images='$USE_IMAGES' | jq '$JQSETPULLPOLICY' | oc create -f -"
else
    os::cmd::expect_success "oadm registry"
fi
os::cmd::expect_success 'oadm policy add-scc-to-user hostnetwork -z router'
os::cmd::expect_success 'oadm router'
os::test::junit::declare_suite_end

${OS_ROOT}/test/end-to-end/core.sh
