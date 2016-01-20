#!/bin/bash

# This command runs any exposed integration tests for the developer tools

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
cd "${OS_ROOT}"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit


for tool in ${OS_ROOT}/tools/*; do
	test_file=${tool}/test/integration.sh
	if [ -e ${test_file} ]; then
		# if the tool exposes an integration test, run it
		os::cmd::expect_success "${test_file}"
	fi
done

echo "test-tools: ok"