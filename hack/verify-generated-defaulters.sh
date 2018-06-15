#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::test::junit::generate_report
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::test::junit::declare_suite_start "verify/defaulters"
os::cmd::expect_success "${OS_ROOT}/hack/update-generated-defaulters.sh --verify-only"
os::test::junit::declare_suite_end