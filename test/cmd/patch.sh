#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/patch"
patch='{"metadata":{"annotations":{"comment":"patch comment"}}}'

os::cmd::expect_success_and_text 'oc process -f examples/zookeeper/template.json | oc apply -f -' 'pod "zookeeper-1" created'
# test we can record changes for openshift objects
os::cmd::expect_success_and_text "oc patch is zookeeper-346-jdk7 -p '$patch' --record" 'zookeeper-346-jdk7" patched'
os::cmd::expect_success_and_text "oc get is zookeeper-346-jdk7 -o yaml" 'kubernetes.io/change-cause'
os::cmd::expect_success_and_text "oc get is zookeeper-346-jdk7 -o yaml" 'comment'
# test we can patch and record change for kubernetes objects
os::cmd::expect_success_and_text "oc patch pod zookeeper-1 -p '$patch' --record" 'zookeeper-1" patched'
os::cmd::expect_success_and_text "oc get pod zookeeper-1 -o yaml" 'kubernetes.io/change-cause'
os::cmd::expect_success_and_text "oc get pod zookeeper-1 -o yaml" 'comment'

echo "patch: ok"
os::test::junit::declare_suite_end
