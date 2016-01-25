#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/lib/log.sh"
source "${OS_ROOT}/hack/lib/util/environment.sh"

echo "[INFO] Starting containerized end-to-end test"

unset KUBECONFIG

os::util::environment::setup_all_server_vars "test-end-to-end-docker/"
os::util::environment::use_sudo
reset_tmp_dir

os::log::start_system_logger

out=$(
	set +e
	docker stop origin 2>&1
	docker rm origin 2>&1
	set -e
)

os::start_container


${OS_ROOT}/test/end-to-end/core.sh
