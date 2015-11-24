#!/bin/bash
# This file ensures that the helper functions in util.sh behave as expected

set -o errexit
set -o nounset
set -o pipefail
# set -x

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

mkdir -p /tmp/openshift/origin/test/cmd/
JUNIT_OUTPUT_FILE=/tmp/openshift-cmd/junit_output.txt

# set verbosity so we can see that command output renders correctly
VERBOSE=1

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
os::cmd::expect_failure_and_text 'grep' 'for more information'

os::cmd::expect_success_and_not_text 'pwd 1>/dev/null' '.' 

os::cmd::expect_failure_and_not_text 'grep 2>/dev/null' '.'

# here document/string
os::cmd::expect_success 'grep hello <<EOF
hello
EOF
'

os::cmd::expect_success 'grep hello <<< hello'

echo "complex tests: ok"


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

rm -rf /tmp/openshift-cmd/