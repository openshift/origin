#!/usr/bin/env bash

mkdir -p serial-junit
PODMAN=$(pwd)/bin/podman bats -T --report-formatter junit -o serial-junit --filter-tags '!ci:parallel' test/system/ || touch fail
mkdir -p parallel-junit
PODMAN=$(pwd)/bin/podman bats -T --report-formatter junit -o parallel-junit --filter-tags ci:parallel -j $(nproc) test/system/ || touch fail
touch done
echo "Finished running tests."
# wait for the test results to be retrieved
tail -f /dev/null
