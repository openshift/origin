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
ginkgo_check_extended
set +e

os::build::extended

test_privileges

echo "[INFO] Starting 'default' extended tests"

dirs_image_env_setup_extended

setup_env_vars

trap "exit" INT TERM
trap "cleanup_extended" EXIT

info_msgs_ip_host_setup_extended

configure_os_server

auth_setup_extended

start_os_server

install_router_extended

install_registry_extended

create_image_streams_extended

echo "[INFO] MASTER IP - ${MASTER_ADDR}"
echo "[INFO] SERVER CONFIG PATH - ${SERVER_CONFIG_DIR}"
echo "[INFO] Starting extended tests ..."

# Run the tests
pushd ${OS_ROOT}/test/extended >/dev/null
export KUBECONFIG="${ADMIN_KUBECONFIG}"
export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"
ginkgo -progress -stream -v -focus="default:" -p ${OS_OUTPUT_BINPATH}/extended.test
popd >/dev/null
