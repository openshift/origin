#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
VERIFY=--verify-only ${SCRIPT_ROOT}/hack/update-codegen.sh
