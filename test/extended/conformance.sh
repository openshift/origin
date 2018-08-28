#!/bin/bash
#
# Runs the conformance extended tests for OpenShift
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
source "${OS_ROOT}/test/extended/setup.sh"

os::test::extended::setup
os::test::extended::focus "$@"

exitstatus=0

# run parallel tests
os::log::info "Running parallel tests N=${PARALLEL_NODES:-<default>}"
SUITE=openshift/conformance/parallel/minimal TEST_PARALLEL="${PARALLEL_NODES:-5}" TEST_REPORT_FILE_NAME=conformance_parallel os::test::extended::run -- -test.timeout 6h ${TEST_EXTENDED_ARGS-} || exitstatus=$?

# run tests in serial
os::log::info "Running serial tests"
SUITE=openshift/conformance/serial/minimal TEST_REPORT_FILE_NAME=conformance_serial os::test::extended::run -- -test.timeout 2h ${TEST_EXTENDED_ARGS-} || exitstatus=$?

exit $exitstatus
