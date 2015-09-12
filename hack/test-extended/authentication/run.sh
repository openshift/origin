#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# The OpenShift Docker registry and router are installed.
# It will start all 'default_*_test.go' test cases.

set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
cd "${OS_ROOT}"

source ${OS_ROOT}/hack/util.sh
source ${OS_ROOT}/hack/common.sh


set -e
ensure_ginkgo_or_die
set +e

os::build::extended

ensure_iptables_or_die

function cleanup()
{
	out=$?
	cleanup_openshift
	echo "[INFO] Exiting"
	exit $out
}

echo "[INFO] Starting 'authentication' extended tests"

trap "exit" INT TERM
trap "cleanup" EXIT

export TMPDIR="${TMPDIR:-"/tmp"}"
export BASETMPDIR="${TMPDIR}/openshift-extended-tests/authentication"
setup_env_vars
reset_tmp_dir 
configure_os_server
start_os_server

install_registry
wait_for_registry

# Run the tests
pushd ${OS_ROOT}/test/extended >/dev/null
export KUBECONFIG="${ADMIN_KUBECONFIG}"
export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"
TMPDIR=${BASETMPDIR} ginkgo -progress -stream -v -focus="authentication:" -p ${OS_OUTPUT_BINPATH}/extended.test
popd >/dev/null

