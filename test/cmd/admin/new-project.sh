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
  oc delete project/recreated-project
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/admin/new-project"
# Test deleting and recreating a project
os::cmd::expect_success 'oadm new-project recreated-project --admin="createuser1"'
os::cmd::expect_success 'oc delete project recreated-project'
os::cmd::try_until_failure 'oc get project recreated-project'
os::cmd::expect_success 'oadm new-project recreated-project --admin="createuser2"'
os::cmd::expect_success_and_text "oc describe policybinding ':default' -n recreated-project" 'createuser2'
echo "new-project: ok"
os::test::junit::declare_suite_end
