#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

cd "${OS_ROOT}"
source hack/lib/init.sh

export TEST_NAME="test-extended/gssapiproxy-tests/$(uname -n)-${CLIENT}-${SERVER}"
os::util::environment::setup_time_vars
os::cleanup::tmpdir
export JUNIT_REPORT_OUTPUT="${LOG_DIR}/raw_test_output.log"

# use a subshell and `if` statement to prevent `exit` calls from killing this script
if ! ( './gssapi-tests.sh' ) 2>&1; then
    return_code=$?
fi

cat "${JUNIT_REPORT_OUTPUT}" 1>&2
exit "${return_code:-0}"
