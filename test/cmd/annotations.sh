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
os::cmd::expect_success_and_text 'oc create -f examples/hello-openshift/hello-pod.json' 'pod "hello-openshift" created'
os::cmd::expect_success_and_text 'oc annotate pod hello-openshift node-selector=""' 'pod "hello-openshift" annotated'
os::cmd::expect_success_and_not_text 'oc get pod hello-openshift --template="{{index .metadata.annotations \"node-selector\"}}"' '.'

echo "annotate: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/label"
# This test validates empty values in key-value pairs set by the label command
os::cmd::expect_success_and_text 'oc label pod hello-openshift label2=""' 'pod "hello-openshift" labeled'
os::cmd::expect_success_and_not_text 'oc get pod hello-openshift --template="{{.metadata.labels.label2}}"' '.'

echo "label: ok"
os::test::junit::declare_suite_end
