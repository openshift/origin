#!/bin/bash
#
# This script tests the os::log::system library

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/cmd.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"
source "${OS_ROOT}/hack/lib/log/system.sh"

os::util::trap::init_err
os::log::stacktrace::install

# ensure that installing cleanup sets the envar
os::cmd::expect_success_and_text 'unset OS_CLEANUP_SYSTEM_LOGGER; os::log::system::install_cleanup; echo ${OS_CLEANUP_SYSTEM_LOGGER}' 'true'

# ensure that logger runs on start
os::cmd::expect_success_and_text 'LOG_DIR=/tmp os::log::system::start
ps --pid=${LOGGER_PID} --format=command
kill -SIGKILL ${LOGGER_PID}' 'sar \-A \-o'

# ensure that logger is killed on cleanup
os::cmd::expect_success_and_not_text '( LOG_DIR=/tmp os::log::system::start ) &
child=$!
kill -SIGTERM "${child}"
wait "${child}"
pstree $$ -a' 'sar \-A \-o'
