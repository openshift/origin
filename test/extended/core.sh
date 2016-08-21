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


# print the tests we are skipping
echo "[INFO] The following tests are excluded:"
TEST_REPORT_DIR= TEST_OUTPUT_QUIET=true ${EXTENDEDTEST} "--ginkgo.skip=${ss}" --ginkgo.dryRun --ginkgo.noColor | grep skip | cut -c 20- | sort
echo

exitstatus=0

# run parallel tests
nodes="${PARALLEL_NODES:-5}"
echo "[INFO] Running parallel tests N=${nodes}"
TEST_REPORT_FILE_NAME=core_parallel ${GINKGO} -v "-focus=${pf}" "-skip=${ps}" -p -nodes "${nodes}" ${EXTENDEDTEST} -- -ginkgo.v -test.timeout 6h || exitstatus=$?

# run tests in serial
echo "[INFO] Running serial tests"
TEST_REPORT_FILE_NAME=core_serial ${GINKGO} -v "-focus=${sf}" "-skip=${ss}" ${EXTENDEDTEST} -- -ginkgo.v -test.timeout 2h || exitstatus=$?

exit $exitstatus
