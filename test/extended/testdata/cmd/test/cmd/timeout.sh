#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/request-timeout"
# This test validates the global request-timeout option
os::cmd::expect_success 'oc create deploymentconfig testdc --image=busybox'
os::cmd::expect_success_and_text 'oc get dc/testdc -w -v=5 --request-timeout=1s 2>&1' 'Timeout exceeded while reading body'
os::cmd::expect_success_and_text 'oc get dc/testdc --request-timeout=1s' 'testdc'
os::cmd::expect_success_and_text 'oc get dc/testdc --request-timeout=1' 'testdc'
os::cmd::expect_success_and_text 'oc get pods --watch -v=5 --request-timeout=1s 2>&1' 'Timeout exceeded while reading body'

echo "request-timeout: ok"
os::test::junit::declare_suite_end
