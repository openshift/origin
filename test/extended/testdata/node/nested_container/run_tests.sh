#!/usr/bin/env bash

# BATS_TEST_RETRIES must be set inside the test file scope, not as an
# environment variable, because bats-exec-file unconditionally resets it
# to 0 at startup. Every podman test file does `load helpers` at file
# scope, so appending it to helpers.bash ensures it takes effect for
# every test.
echo "BATS_TEST_RETRIES=3" >> test/system/helpers.bash

mkdir -p serial-junit
PODMAN=$(pwd)/bin/podman bats -T --report-formatter junit -o serial-junit --filter-tags '!ci:parallel' test/system/ || touch fail
mkdir -p parallel-junit
PODMAN=$(pwd)/bin/podman bats -T --report-formatter junit -o parallel-junit --filter-tags ci:parallel -j $(nproc) test/system/ || touch fail
touch done
echo "Finished running tests."
# wait for the test results to be retrieved
tail -f /dev/null
