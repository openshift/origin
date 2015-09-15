#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# The OpenShift Docker registry and router are installed.
# It will run all tests that are imported into test/extended.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/common.sh"
os::log::install_errexit

ensure_ginkgo_or_die
ensure_iptables_or_die

os::build::setup_env
go test -c ./test/extended -o ${OS_OUTPUT_BINPATH}/extended.test

function cleanup()
{
  out=$?
  cleanup_openshift
  echo "[INFO] Exiting"
  exit $out
}

echo "[INFO] Starting 'images' extended tests"

trap "exit" INT TERM
trap "cleanup" EXIT

export TMPDIR="${TMPDIR:-"/tmp"}"
export BASETMPDIR="${TMPDIR}/openshift-extended-tests/images"
setup_env_vars
reset_tmp_dir
configure_os_server
start_os_server

install_registry
wait_for_registry

echo "[INFO] Creating image streams"
oc create -n openshift -f examples/image-streams/image-streams-centos7.json --config="${ADMIN_KUBECONFIG}"

# Run the tests
pushd ${OS_ROOT}/test/extended >/dev/null
export KUBECONFIG="${ADMIN_KUBECONFIG}"
export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"
TMPDIR=${BASETMPDIR} ginkgo -progress -stream -v -p "$@" ${OS_OUTPUT_BINPATH}/extended.test
popd >/dev/null


