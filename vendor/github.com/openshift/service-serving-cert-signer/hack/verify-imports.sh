#!/bin/bash

# This script verifies that package trees
# conform to our import restrictions
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::util::ensure::built_binary_exists 'import-verifier'

os::test::junit::declare_suite_start "verify/imports"
os::cmd::expect_success "import-verifier ${OS_ROOT}/hack/import-restrictions.json"
os::test::junit::declare_suite_end