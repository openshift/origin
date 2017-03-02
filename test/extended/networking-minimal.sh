#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"

# Wrapper to configure networking.sh to run a minimal set of tests.
NETWORKING_E2E_MINIMAL=1 "${OS_ROOT}/test/extended/networking.sh"
