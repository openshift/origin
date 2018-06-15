#!/bin/bash

# This command runs any exposed integration tests for the developer tools
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::test::junit::declare_suite_start 'tools'

os::util::ensure::built_binary_exists 'junitreport'
os::cmd::expect_success 'tools/junitreport/test/integration.sh'

echo "test-tools: ok"
os::test::junit::declare_suite_end
