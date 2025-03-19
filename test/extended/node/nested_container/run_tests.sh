#!/usr/bin/env bash

mkdir -p junit
PODMAN=$(pwd)/bin/podman bats -T --formatter junit -o junit --filter-tags '!ci:parallel' test/system/
PODMAN=$(pwd)/bin/podman bats -T --formatter junit -o junit --filter-tags ci:parallel -j $(nproc) test/system/

# wait for the test results to be retrieved
tail -f /dev/null
