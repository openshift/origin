#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# build the test executable and make sure it's on the path
if [[ -z "${OPENSHIFT_SKIP_BUILD-}" ]]; then
  "${OS_ROOT}/hack/build-go.sh" "test/integration/integration.test"
fi
os::util::environment::update_path_var

gotest_flags="${GOTEST_FLAGS:-}"
sub_tests="${1:-}"

# Filter to run sub tests and turn up verbosity to show which tests will be run
if [[ -n "${sub_tests}" ]]; then
	gotest_flags+=" -run TestIntegration/${sub_tests} -v"
fi

COVERAGE_SPEC=" " DETECT_RACES=false TMPDIR="${BASETMPDIR}" TIMEOUT=45m GOTEST_FLAGS="${gotest_flags}" "${OS_ROOT}/hack/test-go.sh" "test/integration/runner"
