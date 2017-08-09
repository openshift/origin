#!/bin/bash

# This script verifies that package trees
# conform to our import restrictions
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::util::ensure::built_binary_exists 'import-verifier'

import-verifier "${OS_ROOT}/hack/import-restrictions.json"