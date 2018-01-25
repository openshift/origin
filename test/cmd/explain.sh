#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/explain"
# This test validates that the explain command works with openshift resources

os::cmd::expect_success 'oc explain dc'
os::cmd::expect_success_and_text 'oc explain dc.status.replicas' 'FIELD\: replicas'

os::cmd::expect_success 'oc explain routes'
os::cmd::expect_success_and_text 'oc explain route.metadata.name' 'string'

os::cmd::expect_success 'oc explain bc'
os::cmd::expect_success 'oc explain image'
os::cmd::expect_success 'oc explain is'

os::cmd::expect_success 'oc explain cronjob'

echo "explain: ok"
os::test::junit::declare_suite_end

