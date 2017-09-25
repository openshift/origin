#!/bin/bash
#
# This script verifies that generated Swagger self-describing documentation is up to date.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::test::junit::generate_report
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::test::junit::declare_suite_start "verify/swagger-descriptions"
os::cmd::expect_success "VERIFY=true ${OS_ROOT}/hack/update-generated-swagger-descriptions.sh"
os::test::junit::declare_suite_end