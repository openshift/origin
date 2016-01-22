#!/bin/bash
#
# This script tests the miscellaneous functions in os::util

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/cmd.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"
source "${OS_ROOT}/hack/lib/util/text.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"

os::util::trap::init
os::log::stacktrace::install

# ensure the output from os::util::describe_return_code is correct
os::cmd::expect_success_and_text 'os::util::describe_return_code 0' '\[INFO\] hack/test\-lib/util/misc succeeded'
os::cmd::expect_success_and_text 'START_TIME=0 os::util::describe_return_code 0' '\[INFO\] hack/test\-lib/util/misc succeeded after [0-9]+s'
os::cmd::expect_success_and_text 'os::util::describe_return_code 1' '\[ERROR\] hack/test\-lib/util/misc failed'
os::cmd::expect_success_and_text 'START_TIME=0 os::util::describe_return_code 1' '\[ERROR\] hack/test\-lib/util/misc failed after [0-9]+s'

# ensure that os::util::install_describe_return_code sets the envar correctly
os::cmd::expect_success_and_text 'unset OS_DESCRIBE_RETURN_CODE; os::util::install_describe_return_code; echo "${OS_DESCRIBE_RETURN_CODE}"' 'true'
