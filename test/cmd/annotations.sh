#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/annotate"
# This test validates empty values in key-value pairs set by the annotate command
os::cmd::expect_success_and_text 'oc process -f examples/zookeeper/template.json | oc apply -f -' 'pod "zookeeper-1" created'
os::cmd::expect_success_and_text 'oc annotate pod zookeeper-1 node-selector=""' 'pod "zookeeper-1" annotated'
os::cmd::expect_success_and_not_text 'oc get pod zookeeper-1 --template="{{index .metadata.annotations \"node-selector\"}}"' '.'

echo "annotate: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/label"
# This test validates empty values in key-value pairs set by the label command
os::cmd::expect_success_and_text 'oc label pod zookeeper-1 label2=""' 'pod "zookeeper-1" labeled'
os::cmd::expect_success_and_not_text 'oc get pod zookeeper-1 --template="{{.metadata.labels.label2}}"' '.'

echo "label: ok"
os::test::junit::declare_suite_end
