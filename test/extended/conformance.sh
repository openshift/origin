#!/bin/bash
#
# Runs the conformance extended tests for OpenShift

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/test/extended/setup.sh"
cd "${OS_ROOT}"

os::test::extended::setup
os::test::extended::start_server
os::test::extended::focus "$@"
os::test::extended::conformance
