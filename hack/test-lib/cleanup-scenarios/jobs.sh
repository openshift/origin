#!/bin/bash
#
# This test case starts jobs in the background, to enable testing os::cleanup::kill_all_running_jobs

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/cleanup.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"

os::util::trap::init
os::log::stacktrace::install
os::cleanup::install_kill_all_running_jobs

"${OS_ROOT}/hack/test-lib/cleanup-scenarios/ptree.sh" &

"${OS_ROOT}/hack/test-lib/cleanup-scenarios/ptree.sh" &

"${OS_ROOT}/hack/test-lib/cleanup-scenarios/ptree.sh" &
