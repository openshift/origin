#!/bin/bash
#
# Runs the conformance extended tests for OpenShift
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
source "${OS_ROOT}/test/extended/setup.sh"

os::test::extended::setup
os::test::extended::focus "$@"

function join { local IFS="$1"; shift; echo "$*"; }

parallel_only=( "${CONFORMANCE_TESTS[@]}" )
serial_only=( "${SERIAL_TESTS[@]}" )
if [[ "${OPENSHIFT_SKIP_ALLOWALL_AUTH_TESTS:-false}" = "true" ]]; then
  os::log::warn "Skipping tests requiring the AllowAllPasswordIdentityProvider to OPENSHIFT_ALLOWALL_AUTH_ENABLED"
  parallel_exclude=( "${EXCLUDED_TESTS[@]}" "${SERIAL_TESTS[@]}" "${ALLOWALL_AUTH_REQUIRED_TESTS[@]}" )
  serial_exclude=( "${EXCLUDED_TESTS[@]}" "${ALLOWALL_AUTH_REQUIRED_TESTS[@]}" )
else
  parallel_exclude=( "${EXCLUDED_TESTS[@]}" "${SERIAL_TESTS[@]}" )
  serial_exclude=( "${EXCLUDED_TESTS[@]}" )
fi

pf=$(join '|' "${parallel_only[@]}")
ps=$(join '|' "${parallel_exclude[@]}")
sf=$(join '|' "${serial_only[@]}")
ss=$(join '|' "${serial_exclude[@]}")


echo "[INFO] Running the following tests:"
TEST_REPORT_DIR= TEST_OUTPUT_QUIET=true ${EXTENDEDTEST} "--ginkgo.focus=${pf}" "--ginkgo.skip=${ps}" --ginkgo.dryRun --ginkgo.noColor | grep ok | grep -v skip | cut -c 20- | sort
TEST_REPORT_DIR= TEST_OUTPUT_QUIET=true ${EXTENDEDTEST} "--ginkgo.focus=${sf}" "--ginkgo.skip=${ss}" --ginkgo.dryRun --ginkgo.noColor | grep ok | grep -v skip | cut -c 20- | sort
echo

exitstatus=0

# run parallel tests
nodes="${PARALLEL_NODES:-5}"
echo "[INFO] Running parallel tests N=${nodes}"
TEST_REPORT_FILE_NAME=conformance_parallel ${EXTENDEDTEST} "--ginkgo.focus=${pf}" "--ginkgo.skip=${ps}" "--ginkgo.progress=true" "--ginkgo.parallel.total=${nodes}" --ginkgo.v --test.timeout 6h || exitstatus=$?

# run tests in serial
echo "[INFO] Running serial tests"
TEST_REPORT_FILE_NAME=conformance_serial ${EXTENDEDTEST} "--ginkgo.focus=${sf}" "--ginkgo.skip=${ss}" --ginkgo.v --test.timeout 2h || exitstatus=$?

exit $exitstatus
