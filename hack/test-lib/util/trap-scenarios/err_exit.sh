#!/bin/bash
#
# This test case installs a debug handler for EXIT and exits with error code 55

# remove upstream handlers
trap ERR
trap EXIT
unset OS_USE_STACKTRACE

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../../..
source "${OS_ROOT}/hack/lib/util/trap.sh"
os::util::trap::init

# unset the handler for ERR
trap ERR

# toggle debug handler for EXIT
OS_TRAP_DEBUG="true"

# force an EXIT
exit 55
