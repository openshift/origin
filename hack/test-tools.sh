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
source "${OS_ROOT}/hack/lib/test/junit.sh"
os::log::install_errexit

os::test::junit::declare_suite_start 'tools'

os::cmd::expect_success 'tools/junitreport/test/integration.sh'

echo "test-tools: ok"
os::test::junit::declare_suite_end