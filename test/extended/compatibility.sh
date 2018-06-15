#!/bin/bash
#
# Runs extended compatibility tests with a previous version
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
source "${OS_ROOT}/test/extended/setup.sh"

# Previous version to test against
PREVIOUS_VERSION="v1.3.0"

export API_SERVER_VERSION="${RUN_PREVIOUS_API:+${PREVIOUS_VERSION}}"
export CONTROLLER_VERSION="${RUN_PREVIOUS_CONTROLLER:+${PREVIOUS_VERSION}}"

# For now, compatibility tests will not require a node
# so tests can execute quicker
export SKIP_NODE=1

os::test::extended::setup
os::test::extended::focus "$@"


os::log::info "Running compatibility tests"
FOCUS="\[Compatibility\]" SKIP="${SKIP_TESTS:-}" TEST_REPORT_FILE_NAME=compatibility os::test::extended::run -- -test.timeout 2h
