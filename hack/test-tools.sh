#!/bin/bash

# This command runs any exposed integration tests for the developer tools

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
cd "${OS_ROOT}"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/lib/cmd.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"

os::util::trap::init_err
os::log::stacktrace::install

os::cmd::expect_success 'tools/junitreport/test/integration.sh'

echo "test-tools: ok"