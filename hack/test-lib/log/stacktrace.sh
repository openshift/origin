#!/bin/bash
#
# This script tests the os::log::stacktrace library

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/cmd.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"

os::util::trap::init
os::log::stacktrace::install

pushd "${OS_ROOT}/hack/test-lib/log/stacktrace-scenarios" >/dev/null

# ensure that install sets the envar
os::cmd::expect_success_and_text "unset ${OS_USE_STACKTRACE}; os::log::stacktrace::install; echo ${OS_USE_STACKTRACE}" 'true'

# ensure that install toggles errtrace
os::cmd::expect_success_and_text "set +o errtrace; os::log::stacktrace::install; set +o" '\-o errtrace'
os::cmd::expect_success_and_text "set -o errtrace; os::log::stacktrace::install; set +o" '\-o errtrace'

# an error in a function causes a stacktrace with errexit set
os::cmd::expect_code_and_text './err_in_func' '2' '\[ERROR\] ./err_in_func:29: `grep > /dev/null 2>&1` exited with status 2.
\[INFO\] 		Stack Trace: 
\[INFO\] 		  1: ./err_in_func:29: `grep > /dev/null 2>&1`
\[INFO\] 		  2: ./err_in_func:25: grandchild
\[INFO\] 		  3: ./err_in_func:21: child
\[INFO\] 		  4: ./err_in_func:17: parent
\[INFO\] 		  5: ./err_in_func:32: grandparent
\[INFO\]   Exiting with code 2.'

# an error in a function causes no stacktrace with errexit unset
os::cmd::expect_code_and_not_text './err_in_func_no_errexit' '2' '.'

# an error in a script causes a stacktrace with errexit set
os::cmd::expect_code_and_text './err_in_script' '2' '\[ERROR\] ./err_in_script:16: `grep > /dev/null 2>&1` exited with status 2.
\[INFO\] 		Stack Trace: 
\[INFO\] 		  1: ./err_in_script:16: `grep > /dev/null 2>&1`
\[INFO\]   Exiting with code 2.'

# an error in a script causes no stacktrace with errexit unset
os::cmd::expect_code_and_not_text './err_in_script_no_errexit' '2' '.'

popd >/dev/null
