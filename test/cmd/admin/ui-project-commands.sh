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
  oc delete project/ui-test-project
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/admin/ui-project-commands"
# Test the commands the UI projects page tells users to run
# These should match what is described in projects.html
os::cmd::expect_success 'oadm new-project ui-test-project --admin="createuser"'
os::cmd::expect_success 'oadm policy add-role-to-user admin adduser -n ui-test-project'
# Make sure project can be listed by oc (after auth cache syncs)
os::cmd::try_until_text 'oc get projects' 'ui\-test\-project'
# Make sure users got added
os::cmd::expect_success_and_text "oc describe policybinding ':default' -n ui-test-project" 'createuser'
os::cmd::expect_success_and_text "oc describe policybinding ':default' -n ui-test-project" 'adduser'
echo "ui-project-commands: ok"
os::test::junit::declare_suite_end
