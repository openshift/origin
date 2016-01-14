#!/bin/bash

# This command runs any exposed integration tests for the developer tools

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
cd "${OS_ROOT}"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

tools/junitreport/test/integration.sh

hack/test/trap

echo "test-tools: ok"