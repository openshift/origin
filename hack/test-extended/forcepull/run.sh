#!/bin/bash
#
# This scripts starts the OpenShift server where
# the OpenShift Docker registry and router are installed,
# and then the forcepull tests are launched.
# We intentionally do not run the force pull tests in parallel
# given the tagging based image corruption that occurs - do not
# want 2 tests corrupting an image differently at the same time.

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

echo "[INFO] Starting 'forcepull' extended tests"

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

wait_for_command '[[ "$(oc get endpoints docker-registry --output-version=v1 -t "{{ if .subsets }}{{ len .subsets }}{{ else }}0{{ end }}" --config=/tmp/openshift-extended-tests/openshift.local.config/master/admin.kubeconfig || echo "0")" != "0" ]]' $((5*TIME_MIN))

create_image_streams_extended

echo "[INFO] MASTER IP - ${MASTER_ADDR}"
echo "[INFO] SERVER CONFIG PATH - ${SERVER_CONFIG_DIR}"
echo "[INFO] Starting extended tests for forcepull ..."

# Run the tests
pushd ${OS_ROOT}/test/extended >/dev/null
export KUBECONFIG="${ADMIN_KUBECONFIG}"
export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"
# we don't run in parallel with this suite - do not want different tests tagging the same image in different was at the same time
ginkgo -progress -stream -v -focus="forcepull:" ${OS_OUTPUT_BINPATH}/extended.test
popd >/dev/null
