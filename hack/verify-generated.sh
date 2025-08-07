#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::test::junit::generate_report
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::test::junit::declare_suite_start "verify/generated"
os::cmd::expect_success "${OS_ROOT}/hack/update-generated.sh"
os::cmd::expect_success "git diff --exit-code ${OS_ROOT}/test/extended/util/image/zz_generated.txt"

os::test::junit::declare_suite_end
