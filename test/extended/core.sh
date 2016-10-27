#!/bin/bash
#
# Runs all standard extended tests against either an existing cluster (TEST_ONLY=1)
# or a standard started server.
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
source "${OS_ROOT}/test/extended/setup.sh"

# cgo must be disabled to have the symbol table available
export CGO_ENABLED=0

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


# print the tests we are skipping
os::log::info "The following tests are excluded:"
SKIP_ONLY=1 PRINT_TESTS=1 os::test::extended::test_list "--ginkgo.skip=${ss}" 
os::log::info ""

exitstatus=0

# run parallel tests
nodes="${PARALLEL_NODES:-5}"
os::log::info "Running parallel tests N=${nodes}"
FOCUS="${pf}" SKIP="${ps}" TEST_REPORT_FILE_NAME=core_parallel os::test::extended::run -p -nodes "${nodes}" -- ginkgo.v -test.timeout 6h || exitstatus=$?

# run tests in serial
os::log::info ""
os::log::info "Running serial tests"
FOCUS="${sf}" SKIP="${ss}" TEST_REPORT_FILE_NAME=core_serial os::test::extended::run -- -ginkgo.v -test.timeout 2h || exitstatus=$?

exit $exitstatus
