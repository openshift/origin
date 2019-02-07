#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::test::junit::generate_report
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::test::junit::declare_suite_start "verify/versions"
os::cmd::expect_success "VERIFY=1 ${OS_ROOT}/hack/update-generated-versions.sh"
os::test::junit::declare_suite_end