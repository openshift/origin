#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::test::junit::generate_report
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

# ensure we have the latest compiled binaries
os::util::ensure::built_binary_exists 'clicheck'

os::test::junit::declare_suite_start "verify/cli-conventions"
os::cmd::expect_success "clicheck"
os::test::junit::declare_suite_end