#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
FUNCTIONAL_DIR="${SCRIPT_DIR}/.."

. "${FUNCTIONAL_DIR}/lib.sh"

# disable starting heketi server via the scripts.
# the test cases need to control the server
# directly
export HEKETI_SERVER="${SCRIPT_DIR}/heketi-server"
export HEKETI_TEST_SERVER=no
functional_tests
