#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/describe"
# This test validates non-duplicate errors when describing an existing resource without a defined describer
os::cmd::expect_success 'oc new-app node'
os::cmd::try_until_success 'eventnum=$(oc get events | wc -l) && [[ $eventnum -gt 1 ]]'
os::cmd::expect_failure_and_text 'oc describe events' 'error: no description has been implemented for "Event"'
# ensure no duplicate errors when attempting to describe events
os::cmd::expect_failure_and_text 'oc describe events 2>&1 | wc -l' '1'

echo "describer: ok"
os::test::junit::declare_suite_end
