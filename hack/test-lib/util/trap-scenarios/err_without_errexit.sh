#!/bin/bash
#
# This test case installs a debug handler for ERR, unsets `errexit`, and generates an error, returning code
# 127 as well as the debug handler text

set +o errexit

# remove upstream handlers
trap ERR
trap EXIT
unset OS_USE_STACKTRACE

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../../..
source "${OS_ROOT}/hack/lib/util/trap.sh"
os::util::trap::init

# unset the handler for EXIT
trap EXIT

# toggle debug handler for ERR
OS_TRAP_DEBUG="true"

# force an ERR
not_a_function
