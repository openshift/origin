#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete user test-cmd-user
  oc delete identity test-idp:test-uid
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/admin/user-creation"
os::cmd::expect_success 'oc create user                test-cmd-user'
os::cmd::expect_success 'oc create identity            test-idp:test-uid'
os::cmd::expect_success 'oc create useridentitymapping test-idp:test-uid test-cmd-user'
os::cmd::expect_success_and_text 'oc describe identity test-idp:test-uid' 'test-cmd-user'
os::cmd::expect_success_and_text 'oc describe user     test-cmd-user' 'test-idp:test-uid'
os::test::junit::declare_suite_end
