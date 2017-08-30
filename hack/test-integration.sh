#!/bin/bash
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# build the test executable and make sure it's on the path
if [[ -z "${OPENSHIFT_SKIP_BUILD-}" ]]; then
  "${OS_ROOT}/hack/build-go.sh" "test/integration/integration.test"
fi
os::util::environment::update_path_var

COVERAGE_SPEC=" " DETECT_RACES=false TMPDIR="${BASETMPDIR}" TIMEOUT=45m GOTESTFLAGS="-sub.timeout=120s" "${OS_ROOT}/hack/test-go.sh" "test/integration/runner"
