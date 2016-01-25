#!/bin/bash

# This script runs all of the test written for our Bash libraries.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/lib/util/trap.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"

os::util::trap::init
os::log::stacktrace::install

cd "${OS_ROOT}"

library_tests="$( find 'hack/test-lib/' -not -path '*-scenarios*' -type f -executable )"
for test in ${library_tests}; do
	# run each library test found in a subshell so that it cannot exit this script prematurely
	( ${test} )
	echo "$(basename "${test}"): ok"
done
		