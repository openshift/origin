#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/get"
os::cmd::expect_success_and_text 'oc create -f examples/storage-examples/local-storage-examples/local-nginx-pod.json' "pod \"local-nginx\" created"
# mixed resource output should print resource kind
# prefix even when only one type of resource is present
os::cmd::expect_success_and_text 'oc get all' "po/local-nginx"
# specific resources should not have their kind prefixed
os::cmd::expect_success_and_text 'oc get pod' "local-nginx"
echo "oc get: ok"
os::test::junit::declare_suite_end