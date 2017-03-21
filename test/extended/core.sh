#!/bin/bash
#
# Runs all standard extended tests against either an existing cluster (TEST_ONLY=1)
# or a standard started server.
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
source "${OS_ROOT}/test/extended/setup.sh"

os::test::extended::setup
os::test::extended::focus "$@"

function join { local IFS="$1"; shift; echo "$*"; }

parallel_only=()
parallel_exclude=( "${EXCLUDED_TESTS[@]}" "${SERIAL_TESTS[@]}" )
serial_only=( "${SERIAL_TESTS[@]}" )
serial_exclude=( "${EXCLUDED_TESTS[@]}" )

pf=$(join '|' "${parallel_only[@]:-}")
ps=$(join '|' "${parallel_exclude[@]}")
sf=$(join '|' "${serial_only[@]}")
ss=$(join '|' "${serial_exclude[@]}")

exitstatus=0

# run parallel tests
os::log::info "Running parallel tests N=${PARALLEL_NODES:-<default>}"
TEST_PARALLEL="${PARALLEL_NODES:-5}" FOCUS="${pf}" SKIP="${ps}" TEST_REPORT_FILE_NAME=core_parallel os::test::extended::run -- -test.timeout 6h ${TEST_EXTENDED_ARGS-} || exitstatus=$?

# run tests in serial
os::log::info ""
os::log::info "Running serial tests"
FOCUS="${sf}" SKIP="${ss}" TEST_REPORT_FILE_NAME=core_serial os::test::extended::run -- -test.timeout 2h ${TEST_EXTENDED_ARGS-} || exitstatus=$?

os::test::extended::merge_junit

exit $exitstatus
