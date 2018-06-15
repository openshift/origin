#!/bin/bash
#
# This script tests os::test::junit functionality.

function exit_trap() {
    local return_code=$?

    end_time=$(date +%s)

    if [[ "${return_code}" -eq "0" ]]; then
        verb="succeeded"
    else
        verb="failed"
    fi

    echo "$0 ${verb} after $((${end_time} - ${start_time})) seconds"
    exit "${return_code}"
}

trap exit_trap EXIT

start_time=$(date +%s)
source "$( dirname "${BASH_SOURCE}" )/../../lib/init.sh"

# envars used to track these interactions are not propagated out of the subshells used to run these commands
# therefore each os::cmd call is its own sandbox and complicated scenarios need to play out inside one call
# however, envars from this scope *are* propagated into each subshell, so they need to be cleared in each call

os::test::junit::declare_suite_start 'lib/test/junit'

# shouldn't be able to end a suite straight away
os::cmd::expect_failure_and_text 'unset NUM_OS_JUNIT_SUITES_IN_FLIGHT NUM_OS_JUNIT_TESTS_IN_FLIGHT JUNIT_REPORT_OUTPUT
os::test::junit::declare_suite_end' '\[ERROR\] jUnit suite marker could not be placed, expected suites in flight, got 0'
# should be able to start one straight away
os::cmd::expect_success 'unset NUM_OS_JUNIT_SUITES_IN_FLIGHT NUM_OS_JUNIT_TESTS_IN_FLIGHT JUNIT_REPORT_OUTPUT
os::test::junit::declare_suite_start whatever'
# should be able to start and end a suite
os::cmd::expect_success 'unset NUM_OS_JUNIT_SUITES_IN_FLIGHT NUM_OS_JUNIT_TESTS_IN_FLIGHT JUNIT_REPORT_OUTPUT
os::test::junit::declare_suite_start whatever
os::test::junit::declare_suite_end'
# should not be able to end more suites than are in flight
os::cmd::expect_failure_and_text 'unset NUM_OS_JUNIT_SUITES_IN_FLIGHT NUM_OS_JUNIT_TESTS_IN_FLIGHT JUNIT_REPORT_OUTPUT
os::test::junit::declare_suite_start whatever
os::test::junit::declare_suite_end
os::test::junit::declare_suite_end' '\[ERROR\] jUnit suite marker could not be placed, expected suites in flight, got 0'
# should not be able to end more suites than are in flight
os::cmd::expect_failure_and_text 'unset NUM_OS_JUNIT_SUITES_IN_FLIGHT NUM_OS_JUNIT_TESTS_IN_FLIGHT JUNIT_REPORT_OUTPUT
os::test::junit::declare_suite_start whatever
os::test::junit::declare_suite_start whateverelse
os::test::junit::declare_suite_end
os::test::junit::declare_suite_end
os::test::junit::declare_suite_end' '\[ERROR\] jUnit suite marker could not be placed, expected suites in flight, got 0'
# should be able to staart a test
os::cmd::expect_success 'unset NUM_OS_JUNIT_SUITES_IN_FLIGHT NUM_OS_JUNIT_TESTS_IN_FLIGHT JUNIT_REPORT_OUTPUT
os::test::junit::declare_suite_start whatever
os::test::junit::declare_test_start'
# shouldn't be able to end a test that hasn't been started
os::cmd::expect_failure_and_text 'unset NUM_OS_JUNIT_SUITES_IN_FLIGHT NUM_OS_JUNIT_TESTS_IN_FLIGHT JUNIT_REPORT_OUTPUT
os::test::junit::declare_test_end' '\[ERROR\] jUnit test marker could not be placed, expected one test in flight, got 0'
# should be able to start and end a test case
os::cmd::expect_success 'unset NUM_OS_JUNIT_SUITES_IN_FLIGHT NUM_OS_JUNIT_TESTS_IN_FLIGHT JUNIT_REPORT_OUTPUT
os::test::junit::declare_suite_start whatever
os::test::junit::declare_test_start
os::test::junit::declare_test_end'
# shouldn't be able to end too many test cases
os::cmd::expect_failure_and_text 'unset NUM_OS_JUNIT_SUITES_IN_FLIGHT NUM_OS_JUNIT_TESTS_IN_FLIGHT JUNIT_REPORT_OUTPUT
os::test::junit::declare_suite_start whatever
os::test::junit::declare_test_start
os::test::junit::declare_test_end
os::test::junit::declare_test_end' '\[ERROR\] jUnit test marker could not be placed, expected one test in flight, got 0'
# shouldn't be able to start a test without a suite
os::cmd::expect_failure_and_text 'unset NUM_OS_JUNIT_SUITES_IN_FLIGHT NUM_OS_JUNIT_TESTS_IN_FLIGHT JUNIT_REPORT_OUTPUT
os::test::junit::declare_test_start' '\[ERROR\] jUnit test marker could not be placed, expected suites in flight, got 0'

os::test::junit::declare_suite_end