#!/bin/bash
#
# Runs the conformance extended tests for OpenShift
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
source "${OS_ROOT}/test/extended/setup.sh"

os::test::extended::setup
os::test::extended::focus "$@"

exitstatus=0

echo "DEBUG BEGIN ***************************************************"
printf '%s:\t"%s"\n' \
    "SERVER_CONFIG_DIR" "${SERVER_CONFIG_DIR:-}" \
    "MASTER_CONFIG_DIR" "${MASTER_CONFIG_DIR:-}" \
    "imagePolicyConfig" "$(sed -n  '/^imagePolicyConfig/,/^[^ ]/p' "${MASTER_CONFIG_DIR:-}/master-config.yaml")"
    "OPENSHIFT_DEFAULT_REGISTRY" "${OPENSHIFT_DEFAULT_REGISTRY}"
echo "DEBUG END *****************************************************"

# run parallel tests
os::log::info "Running parallel tests N=${PARALLEL_NODES:-<default>}"
TEST_PARALLEL="${PARALLEL_NODES:-5}" TEST_REPORT_FILE_NAME=conformance_parallel os::test::extended::run -- -suite "parallel.conformance.openshift.io" -test.timeout 6h ${TEST_EXTENDED_ARGS-} || exitstatus=$?

# run tests in serial
os::log::info "Running serial tests"
TEST_REPORT_FILE_NAME=conformance_serial os::test::extended::run -- -suite "serial.conformance.openshift.io" -test.timeout 2h ${TEST_EXTENDED_ARGS-} || exitstatus=$?

exit $exitstatus
