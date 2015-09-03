#!/bin/bash

# This script starts standalone etcd instance and the OpenShift master API
# server with a default configuration with overriden controllerLeaseTTL.
# Controllers need to be started and managed by go test suite.

set -o errexit
set -o nounset
set -o pipefail

CONTROLLER_LEASE_TTL=10

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source ${OS_ROOT}/hack/util.sh
source ${OS_ROOT}/hack/common.sh
os::log::install_errexit

ensure_ginkgo_or_die
ensure_iptables_or_die

os::build::setup_env
go test -c ./test/extended -o ${OS_OUTPUT_BINPATH}/ha.test

function cleanup()
{
	out=$?
	cleanup_openshift
	echo "[INFO] Exiting"
	exit $out
}

echo "[INFO] Starting 'ha' extended tests"

trap "exit" INT TERM
trap "cleanup" EXIT

export TMPDIR="${TMPDIR:-"/tmp"}"
export BASETMPDIR="${TMPDIR}/openshift-extended-tests/ha"
setup_env_vars
export MASTER_CONFIG_PATH="${MASTER_CONFIG_DIR}/master-config.yaml"
reset_tmp_dir

configure_os_server
sed -i "s/\(\<controllerLeaseTTL\s*:\s*\)[0-9]\+/\1$CONTROLLER_LEASE_TTL/" \
	$MASTER_CONFIG_PATH

# Start standalone etcd server
export ETCD_CERT_FILE="${MASTER_CONFIG_DIR}/etcd.server.crt"
export ETCD_KEY_FILE="${MASTER_CONFIG_DIR}/etcd.server.key"
export ETCD_TRUSTED_CA_FILE="${MASTER_CONFIG_DIR}/ca.crt"
start_etcd_extended ${API_HOST}

# Controllers will be started by extended tests
start_os_api_server
start_os_node

# Run the tests
pushd ${OS_ROOT}/test/extended >/dev/null
export KUBECONFIG="${ADMIN_KUBECONFIG}"
export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"
TMPDIR=${BASETMPDIR} ginkgo -progress -stream -v -focus="ha:" -p=false ${OS_OUTPUT_BINPATH}/ha.test
popd >/dev/null
