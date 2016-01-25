#!/bin/bash
#
# This test case sets up a containerized OpenShift master 

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/lib/os.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"

os::util::trap::init

BASETMPDIR="${BASETMPDIR:-/tmp}/openshift/test-lib/"

setup_env_vars
reset_tmp_dir
os::configure_server
os::start_container
