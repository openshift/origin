#!/bin/bash
#
# This test case generates an error inside of a script with errexit set

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../../..
source "${OS_ROOT}/hack/lib/util/trap.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"

os::util::trap::init_err
os::log::stacktrace::install

grep >/dev/null 2>&1
