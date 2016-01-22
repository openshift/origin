#!/bin/bash

# This script sets up a go workspace locally and builds all go components.

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"

os::util::trap::init
os::log::stacktrace::install

os::build::build_binaries "$@"
os::build::place_bins "$@"
os::build::make_openshift_binary_symlinks

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
