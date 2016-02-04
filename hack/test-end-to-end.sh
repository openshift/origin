#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..

if [[ "${TEST_END_TO_END:-}" != "direct" ]]; then
	if docker version >/dev/null 2>&1; then
		echo "++ Docker is installed, running hack/test-end-to-end-docker.sh instead."
		"${OS_ROOT}/hack/test-end-to-end-docker.sh"
		exit $?
	fi
	echo "++ Docker is not installed, running end-to-end against local binaries"
fi

source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/lib/log.sh"
source "${OS_ROOT}/hack/lib/util/environment.sh"
os::log::install_errexit

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

${OS_ROOT}/test/end-to-end/core.sh