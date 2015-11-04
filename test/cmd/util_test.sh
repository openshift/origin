#!/bin/bash
# This file ensures that the helper functions in util.sh behave as expected

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/test/cmd/util.sh"
os::log::install_errexit

function echo_and_exit() {
	echo $1
	exit $2
}

mkdir -p /tmp/openshift/origin/test/cmd/
JUNIT_OUTPUT_FILE=/tmp/openshift-cmd/junit_output.txt

# expect_code
output=$(test::cmd::expect_code 'exit 0' '0' 2>&1)
echo ${output} | grep -e 'SUCCESS' >& /dev/null

output=$(test::cmd::expect_code 'exit 1' '0' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null

output=$(test::cmd::expect_code 'exit 1' '1' 2>&1)
echo ${output} | grep -e 'SUCCESS' >& /dev/null

output=$(test::cmd::expect_code 'exit 0' '1' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null

output=$(test::cmd::expect_code 'exit 99' '99' 2>&1)
echo ${output} | grep -e 'SUCCESS' >& /dev/null

output=$(test::cmd::expect_code 'exit 1' '99' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null

# expect_success
output=$(test::cmd::expect_success 'exit 0' 2>&1)
echo ${output} | grep -e 'SUCCESS' >& /dev/null

output=$(test::cmd::expect_success 'exit 1' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null

# expect_failure
output=$(test::cmd::expect_failure 'exit 1' 2>&1)
echo ${output} | grep -e 'SUCCESS' >& /dev/null

output=$(test::cmd::expect_failure 'exit 0' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null

# error codes other than 1 *are* failures but expect_failure explicitly expects 1
output=$(test::cmd::expect_failure 'exit 10' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null

# expect_code_and_text
output=$(test::cmd::expect_code_and_text 'echo_and_exit hello 0' '0' 'hello' 2>&1)
echo ${output} | grep -e 'SUCCESS' >& /dev/null

output=$(test::cmd::expect_code_and_text 'echo_and_exit hello 1' '0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e 'hello' >& /dev/null

output=$(test::cmd::expect_code_and_text 'echo_and_exit goodbye 0' '0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the output content test failed' >& /dev/null
echo ${output} | grep -e 'goodbye' >& /dev/null

output=$(test::cmd::expect_code_and_text 'echo_and_exit goodbye 1' '0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e '; the output content test failed' >& /dev/null
echo ${output} | grep -e 'goodbye' >& /dev/null

# expect_success_and_text
output=$(test::cmd::expect_success_and_text 'echo_and_exit hello 0' 'hello' 2>&1)
echo ${output} | grep -e 'SUCCESS' >& /dev/null

output=$(test::cmd::expect_success_and_text 'echo_and_exit hello 1' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e 'hello' >& /dev/null

output=$(test::cmd::expect_success_and_text 'echo_and_exit goodbye 0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the output content test failed' >& /dev/null
echo ${output} | grep -e 'goodbye' >& /dev/null

output=$(test::cmd::expect_success_and_text 'echo_and_exit goodbye 1' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e '; the output content test failed' >& /dev/null
echo ${output} | grep -e 'goodbye' >& /dev/null

# expect_failure_and_text
output=$(test::cmd::expect_failure_and_text 'echo_and_exit hello 1' 'hello' 2>&1)
echo ${output} | grep -e 'SUCCESS' >& /dev/null

output=$(test::cmd::expect_failure_and_text 'echo_and_exit hello 0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e 'hello' >& /dev/null

output=$(test::cmd::expect_failure_and_text 'echo_and_exit goodbye 1' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the output content test failed' >& /dev/null
echo ${output} | grep -e 'goodbye' >& /dev/null

output=$(test::cmd::expect_failure_and_text 'echo_and_exit goodbye 0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e '; the output content test failed' >& /dev/null
echo ${output} | grep -e 'goodbye' >& /dev/null

# expect_code_and_not_text
output=$(test::cmd::expect_code_and_not_text 'echo_and_exit goodbye 0' '0' 'hello' 2>&1)
echo ${output} | grep -e 'SUCCESS' >& /dev/null

output=$(test::cmd::expect_code_and_not_text 'echo_and_exit goodbye 1' '0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e 'goodbye' >& /dev/null

output=$(test::cmd::expect_code_and_not_text 'echo_and_exit hello 0' '0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the output content test failed' >& /dev/null
echo ${output} | grep -e 'hello' >& /dev/null

output=$(test::cmd::expect_code_and_not_text 'echo_and_exit hello 1' '0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e '; the output content test failed' >& /dev/null
echo ${output} | grep -e 'hello' >& /dev/null

# expect_success_and_not_text
output=$(test::cmd::expect_success_and_not_text 'echo_and_exit goodbye 0' 'hello' 2>&1)
echo ${output} | grep -e 'SUCCESS' >& /dev/null

output=$(test::cmd::expect_success_and_not_text 'echo_and_exit goodbye 1' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e 'goodbye' >& /dev/null

output=$(test::cmd::expect_success_and_not_text 'echo_and_exit hello 0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the output content test failed' >& /dev/null
echo ${output} | grep -e 'hello' >& /dev/null

output=$(test::cmd::expect_success_and_not_text 'echo_and_exit hello 1' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e '; the output content test failed' >& /dev/null
echo ${output} | grep -e 'hello' >& /dev/null

# expect_failure_and_not_text
output=$(test::cmd::expect_failure_and_not_text 'echo_and_exit goodbye 1' 'hello' 2>&1)
echo ${output} | grep -e 'SUCCESS' >& /dev/null

output=$(test::cmd::expect_failure_and_not_text 'echo_and_exit goodbye 0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e 'goodbye' >& /dev/null

output=$(test::cmd::expect_failure_and_not_text 'echo_and_exit hello 1' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the output content test failed' >& /dev/null
echo ${output} | grep -e 'hello' >& /dev/null

output=$(test::cmd::expect_failure_and_not_text 'echo_and_exit hello 0' 'hello' 2>&1) || true
echo ${output} | grep -e 'FAILURE' >& /dev/null
echo ${output} | grep -e 'the command returned the wrong error code' >& /dev/null
echo ${output} | grep -e '; the output content test failed' >& /dev/null
echo ${output} | grep -e 'hello' >& /dev/null

echo "SUCCESS"

rm -rf /tmp/openshift-cmd/