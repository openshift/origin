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
source "${OS_ROOT}/hack/lib/os.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"
source "${OS_ROOT}/hack/lib/util/environment.sh"
os::log::install_errexit
os::util::trap::init

ensure_iptables_or_die

echo "[INFO] Starting end-to-end test"

# Start All-in-one server and wait for health
os::util::environment::setup_all_server_vars "test-end-to-end/"
os::util::environment::use_sudo
reset_tmp_dir

os::log::start_system_logger

os::configure_server
os::start_server

# set our default KUBECONFIG location
export KUBECONFIG="${ADMIN_KUBECONFIG}"

${OS_ROOT}/test/end-to-end/core.sh