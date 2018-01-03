#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
VERIFY=--verify-only ${SCRIPT_ROOT}/hack/update-deepcopy.sh
