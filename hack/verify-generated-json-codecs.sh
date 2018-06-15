#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::test::junit::generate_report
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::test::junit::declare_suite_start "verify/codecs"
# no ugorji codecs should be checked in
os::cmd::expect_success_and_not_text "find ${OS_ROOT}/vendor/k8s.io/kubernetes -name 'types.generated.go'" "."
os::test::junit::declare_suite_end