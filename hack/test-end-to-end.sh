#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"
os::log::install_errexit

ensure_iptables_or_die

echo "[INFO] Starting end-to-end test"

function cleanup()
{
	out=$1
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
}

os::util::trap::add "cleanup" EXIT INT TERM


# Start All-in-one server and wait for health
TMPDIR="${TMPDIR:-"/tmp"}"
BASETMPDIR="${BASETMPDIR:-${TMPDIR}/openshift-e2e}"
setup_env_vars
reset_tmp_dir
configure_os_server
start_os_server

# set our default KUBECONFIG location
export KUBECONFIG="${ADMIN_KUBECONFIG}"

${OS_ROOT}/test/end-to-end/core.sh