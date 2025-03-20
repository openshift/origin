#!/usr/bin/env bash

mkdir -p junit
PODMAN=$(pwd)/bin/podman bats -T --formatter junit --filter-tags '!ci:parallel' test/system/ > junit/serial-nested-container.xml
echo "First tests done"
PODMAN=$(pwd)/bin/podman bats -T --formatter junit --filter-tags ci:parallel -j $(nproc) test/system/ > junit/parallel-nested-container.xml
echo "Second tests done"
touch done
# wait for the test results to be retrieved
tail -f /dev/null
