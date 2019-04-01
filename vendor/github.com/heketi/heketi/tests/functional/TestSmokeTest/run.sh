#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
FUNCTIONAL_DIR="${SCRIPT_DIR}/.."

. "${FUNCTIONAL_DIR}/lib.sh"

functional_tests
