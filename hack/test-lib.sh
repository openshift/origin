#!/bin/bash

# This script runs all of the test written for our Bash libraries.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
	os::test::junit::generate_report
	os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

library_tests="$( find 'hack/test-lib/' -type f -executable )"
for test in ${library_tests}; do
	# run each library test found in a subshell so that we can isolate them
	( ${test} )
	echo "$(basename "${test//.sh}"): ok"
done