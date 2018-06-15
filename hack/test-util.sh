#!/bin/bash
# This file ensures that the helper functions in util.sh behave as expected
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::cleanup::tmpdir
os::util::environment::setup_all_server_vars
export HOME="${FAKE_HOME_DIR}"

# set verbosity so we can see that command output renders correctly
VERBOSE=1

os::test::junit::declare_suite_start "cmd/util"

os::test::junit::declare_suite_start "cmd/util/positive"
# positive tests
os::cmd::expect_success 'exit 0'

os::cmd::expect_failure 'exit 10'

os::cmd::expect_success_and_text 'printf "hello" && exit 0' 'hello'

os::cmd::expect_failure_and_text 'printf "hello" && exit 19' 'hello'

os::cmd::expect_success_and_not_text 'echo "goodbye" && exit 0' 'hello'

os::cmd::expect_failure_and_not_text 'echo "goodbye" && exit 19' 'hello'

os::cmd::expect_code 'exit 195' '195'

os::cmd::expect_code_and_text 'echo "hello" && exit 213' '213' 'hello'

os::cmd::expect_code_and_not_text 'echo "goodbye" && exit 213' '213' 'hello'

echo "positive tests: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/util/negative"
# negative tests

if os::cmd::expect_success 'exit 1'; then
	exit 1
fi

if os::cmd::expect_failure 'exit 0'; then
	exit 1
fi

if os::cmd::expect_success_and_text 'echo "goodbye" && exit 0' 'hello'; then
	exit 1
fi

if os::cmd::expect_success_and_text 'echo "hello" && exit 1' 'hello'; then
	exit 1
fi

if os::cmd::expect_success_and_text 'echo "goodbye" && exit 1' 'hello'; then
	exit 1
fi

if os::cmd::expect_failure_and_text 'echo "goodbye" && exit 1' 'hello'; then
	exit 1
fi

if os::cmd::expect_failure_and_text 'echo "hello" && exit 0' 'hello'; then
	exit 1
fi

if os::cmd::expect_failure_and_text 'echo "goodbye" && exit 0' 'hello'; then
	exit 1
fi

if os::cmd::expect_success_and_not_text 'echo "hello" && exit 0' 'hello'; then
	exit 1
fi

if os::cmd::expect_success_and_not_text 'echo "goodbye" && exit 1' 'hello'; then
	exit 1
fi

if os::cmd::expect_success_and_not_text 'echo "hello" && exit 1' 'hello'; then
	exit 1
fi

if os::cmd::expect_failure_and_not_text 'echo "goodbye" && exit 0' 'hello'; then
	exit 1
fi

if os::cmd::expect_failure_and_not_text 'echo "hello" && exit 1' 'hello'; then
	exit 1
fi

if os::cmd::expect_failure_and_not_text 'echo "hello" && exit 0' 'hello'; then
	exit 1
fi

if os::cmd::expect_code 'exit 1' '200'; then
	exit 1
fi

if os::cmd::expect_code_and_text 'echo "hello" && exit 0' '1' 'hello'; then
	exit 1
fi

if os::cmd::expect_code_and_text 'echo "goodbye" && exit 1' '1' 'hello'; then
	exit 1
fi

if os::cmd::expect_code_and_text 'echo "goodbye" && exit 0' '1' 'hello'; then
	exit 1
fi

if os::cmd::expect_code_and_not_text 'echo "goodbye" && exit 0' '1' 'hello'; then
	exit 1
fi

if os::cmd::expect_code_and_not_text 'echo "hello" && exit 1' '1' 'hello'; then
	exit 1
fi

if os::cmd::expect_code_and_not_text 'echo "hello" && exit 0' '1' 'hello'; then
	exit 1
fi

echo "negative tests: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/util/complex"
# complex input tests

# pipes
os::cmd::expect_success 'echo "hello" | grep hello'

os::cmd::expect_success 'echo "-1" | xargs ls'

# variables
VAR=hello
os::cmd::expect_success_and_text 'echo $(echo "${VAR}")' 'hello'
unset VAR

# semicolon
os::cmd::expect_success 'echo "hello"; pwd'

# spaces in strings
os::cmd::expect_success_and_text 'echo "-v marker"' 'v marker'

# curly braces
os::cmd::expect_success_and_text 'ls "${OS_ROOT}"/hack/update-generated-co{n,m}*.sh' 'completions'

os::cmd::expect_success_and_text 'ls "${OS_ROOT}"/hack/update-generated-co{n,m}*.sh' 'conversions'

# integer arithmetic
os::cmd::expect_success_and_text 'if (( 1 )); then echo "hello"; fi' 'hello'

os::cmd::expect_success_and_text 'echo $(( 1 - 20 ))' '\-19' # we need to escape for grep

# redirects
os::cmd::expect_failure_and_text 'grep' '(Usage|usage)'

os::cmd::expect_success_and_not_text 'pwd 1>/dev/null' '.'

os::cmd::expect_failure_and_not_text 'grep 2>/dev/null' '(Usage|usage)'

# here document/string
os::cmd::expect_success 'grep hello <<EOF
hello
EOF
'

os::cmd::expect_success 'grep hello <<< hello'

echo "complex tests: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/util/output"
# test for output correctness

# expect_code
output=$(os::cmd::expect_code 'exit 0' '0')
echo "${output}" | grep -q 'SUCCESS'

output=$(os::cmd::expect_code 'exit 1' '0') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'

output=$(os::cmd::expect_code 'exit 1' '1')
echo "${output}" | grep -q 'SUCCESS'

output=$(os::cmd::expect_code 'exit 0' '1') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'

output=$(os::cmd::expect_code 'exit 99' '99')
echo "${output}" | grep -q 'SUCCESS'

output=$(os::cmd::expect_code 'exit 1' '99') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'

# expect_success
output=$(os::cmd::expect_success 'exit 0')
echo "${output}" | grep -q 'SUCCESS'

output=$(os::cmd::expect_success 'exit 1') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'

# expect_failure
output=$(os::cmd::expect_failure 'exit 1')
echo "${output}" | grep -q 'SUCCESS'

output=$(os::cmd::expect_failure 'exit 0') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'

# expect_code_and_text
output=$(os::cmd::expect_code_and_text 'echo "hello" && exit 0' '0' 'hello')
echo "${output}" | grep -q 'SUCCESS'

output=$(os::cmd::expect_code_and_text 'echo "hello" && exit 1' '0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q 'hello'

output=$(os::cmd::expect_code_and_text 'echo "goodbye" && exit 0' '0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the output content test failed'
echo "${output}" | grep -q 'goodbye'

output=$(os::cmd::expect_code_and_text 'echo "goodbye" && exit 1' '0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q '; the output content test failed'
echo "${output}" | grep -q 'goodbye'

# expect_success_and_text
output=$(os::cmd::expect_success_and_text 'echo "hello" && exit 0' 'hello')
echo "${output}" | grep -q 'SUCCESS'

output=$(os::cmd::expect_success_and_text 'echo "hello" && exit 1' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q 'hello'

output=$(os::cmd::expect_success_and_text 'echo "goodbye" && exit 0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the output content test failed'
echo "${output}" | grep -q 'goodbye'

output=$(os::cmd::expect_success_and_text 'echo "goodbye" && exit 1' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q '; the output content test failed'
echo "${output}" | grep -q 'goodbye'

# expect_failure_and_text
output=$(os::cmd::expect_failure_and_text 'echo "hello" && exit 1' 'hello')
echo "${output}" | grep -q 'SUCCESS'

output=$(os::cmd::expect_failure_and_text 'echo "hello" && exit 0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q 'hello'

output=$(os::cmd::expect_failure_and_text 'echo "goodbye" && exit 1' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the output content test failed'
echo "${output}" | grep -q 'goodbye'

output=$(os::cmd::expect_failure_and_text 'echo "goodbye" && exit 0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q '; the output content test failed'
echo "${output}" | grep -q 'goodbye'

# expect_code_and_not_text
output=$(os::cmd::expect_code_and_not_text 'echo "goodbye" && exit 0' '0' 'hello')
echo "${output}" | grep -q 'SUCCESS'

output=$(os::cmd::expect_code_and_not_text 'echo "goodbye" && exit 1' '0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q 'goodbye'

output=$(os::cmd::expect_code_and_not_text 'echo "hello" && exit 0' '0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the output content test failed'
echo "${output}" | grep -q 'hello'

output=$(os::cmd::expect_code_and_not_text 'echo "hello" && exit 1' '0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q '; the output content test failed'
echo "${output}" | grep -q 'hello'

# expect_success_and_not_text
output=$(os::cmd::expect_success_and_not_text 'echo "goodbye" && exit 0' 'hello')
echo "${output}" | grep -q 'SUCCESS'

output=$(os::cmd::expect_success_and_not_text 'echo "goodbye" && exit 1' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q 'goodbye'

output=$(os::cmd::expect_success_and_not_text 'echo "hello" && exit 0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the output content test failed'
echo "${output}" | grep -q 'hello'

output=$(os::cmd::expect_success_and_not_text 'echo "hello" && exit 1' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q '; the output content test failed'
echo "${output}" | grep -q 'hello'

# expect_failure_and_not_text
output=$(os::cmd::expect_failure_and_not_text 'echo "goodbye" && exit 1' 'hello')
echo "${output}" | grep -q 'SUCCESS'

output=$(os::cmd::expect_failure_and_not_text 'echo "goodbye" && exit 0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q 'goodbye'

output=$(os::cmd::expect_failure_and_not_text 'echo "hello" && exit 1' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the output content test failed'
echo "${output}" | grep -q 'hello'

output=$(os::cmd::expect_failure_and_not_text 'echo "hello" && exit 0' 'hello') || true
echo "${output}" | grep -q 'FAILURE'
echo "${output}" | grep -q 'the command returned the wrong error code'
echo "${output}" | grep -q '; the output content test failed'
echo "${output}" | grep -q 'hello'

echo "output tests: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/util/tryuntil"
function current_time_millis_mod_1000() {
	mod=$(expr $(date +%s000) % 1000)
	if [ $mod -eq 0 ]; then
		echo "success"
		return 0
	else
		echo "failure"
		return 1
	fi
}
os::cmd::try_until_text 'current_time_millis_mod_1000' 'success' $(( 2 * second )) '0'

# force a time-out fail
if os::cmd::try_until_text 'current_time_millis_mod_1000' 'typo' $(( 1 * second )); then
	exit 1
fi

os::cmd::try_until_success 'current_time_millis_mod_1000' $(( 2 * second )) '0'

# force a time-out fail
if os::cmd::try_until_success 'exit 1' $(( 1 * second )); then
	exit 1
fi

output=$(os::cmd::try_until_success 'exit 1' $(( 1 * second ))) || true
echo "${output}" | grep -q 'the command timed out'

function not_current_time_millis_mod_1000() {
	mod=$(expr $(date +%s000) % 1000)
	if [ $mod -eq 0 ]; then
		echo "failure"
		return 1
	else
		echo "success"
		return 0
	fi
}
os::cmd::try_until_failure 'not_current_time_millis_mod_1000' $(( 2 * second )) '0'

# force a timeout
if os::cmd::try_until_failure 'exit 0' $(( 1 * second )); then
	exit 1
fi

echo "try_until: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/util/compression"
TMPDIR="${TMPDIR:-"/tmp"}"
TEST_DIR=${TMPDIR}/openshift/origin/test/cmd
rm -rf ${TEST_DIR} || true
mkdir -p ${TEST_DIR}

echo -e 'success
line 2
\x1e
success
line 2
\x1e
success
line 2
\x1e
success NEW
line 2
\x1e
success NEW
line 2
\x1e
success NEW
line 2
\x1e
success NEW
line 2
\x1e
success OLD
line 2
\x1e
success OLD
line 2
\x1e
success OLD
line 2
\x1e
\x1e
\x1e
\x1e
\x1e' > ${TEST_DIR}/compress_test.txt

echo "success
line 2
... repeated 3 times
success NEW
line 2
... repeated 4 times
success OLD
line 2
... repeated 3 times" > ${TEST_DIR}/expected-compressed.out

os::cmd::internal::compress_output ${TEST_DIR}//compress_test.txt > ${TEST_DIR}/actual-compressed.out
diff ${TEST_DIR}/expected-compressed.out ${TEST_DIR}/actual-compressed.out
echo "compression: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end

os::test::junit::check_test_counters