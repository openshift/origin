#!/bin/bash

# This script runs all of the test written for our Bash libraries.
function exit_trap() {
    local return_code=$?

    end_time=$(date +%s)

    if [[ "${return_code}" -eq "0" ]]; then
        verb="succeeded"
    else
        verb="failed"
    fi

    echo "$0 ${verb} after $((${end_time} - ${start_time})) seconds"
    exit "${return_code}"
}

trap exit_trap EXIT

start_time=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
os::util::environment::setup_tmpdir_vars "test-lib"

cd "${OS_ROOT}"

library_tests="$( find 'hack/test-lib/' -type f -executable )"
for test in ${library_tests}; do
	# run each library test found in a subshell so that we can isolate them
	( ${test} )
	echo "$(basename "${test//.sh}"): ok"
done