#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/builds/setbuildhook"
# Validate the set build-hook command
arg="-f test/testdata/test-bc.yaml"
os::cmd::expect_failure_and_text "oc set build-hook" "error: one or more build configs"
os::cmd::expect_failure_and_text "oc set build-hook ${arg}" "error: you must specify a type of hook"
os::cmd::expect_success_and_text "oc set build-hook ${arg} --post-commit -o yaml -- echo 'hello world'" 'postCommit:'
os::cmd::expect_success_and_text "oc set build-hook ${arg} --post-commit -o yaml -- echo 'hello world'" 'args:'
os::cmd::expect_success_and_text "oc set build-hook ${arg} --post-commit -o yaml -- echo 'hello world'" '\- echo'
os::cmd::expect_success_and_text "oc set build-hook ${arg} --post-commit -o yaml -- echo 'hello world'" '\- hello world'
os::cmd::expect_success_and_not_text "oc set build-hook ${arg} --post-commit -o yaml -- echo 'hello world'" 'command:'
os::cmd::expect_success_and_text "oc set build-hook ${arg} --post-commit --command -o yaml -- echo 'hello world'" 'command:'
os::cmd::expect_success_and_text "oc set build-hook ${arg} --post-commit -o yaml --script='echo \"hello world\"'" 'script: echo \"hello world\"'
# Server object tests
os::cmd::expect_success "oc create -f test/testdata/test-bc.yaml"
os::cmd::expect_failure_and_text "oc set build-hook bc/test-buildconfig --post-commit" "you must specify either a script or command"
os::cmd::expect_success_and_text "oc set build-hook test-buildconfig --post-commit -- echo 'hello world'" "updated"
os::cmd::expect_success_and_text "oc set build-hook bc/test-buildconfig --post-commit -- echo 'hello world'" "was not changed"
os::cmd::expect_success_and_text "oc get bc/test-buildconfig -o yaml" "args:"
os::cmd::expect_success_and_text "oc set build-hook bc/test-buildconfig --post-commit --command -- /bin/bash -c \"echo 'test'\"" "updated"
os::cmd::expect_success_and_text "oc get bc/test-buildconfig -o yaml" "command:"
os::cmd::expect_success_and_text "oc set build-hook --all --post-commit -- echo 'all bc'" "updated"
os::cmd::expect_success_and_text "oc get bc -o yaml" "all bc"
os::cmd::expect_success_and_text "oc set build-hook bc/test-buildconfig --post-commit --remove" "updated"
os::cmd::expect_success_and_not_text "oc get bc/test-buildconfig -o yaml" "args:"
os::cmd::expect_success "oc delete bc/test-buildconfig"
echo "set build-hook: ok"
os::test::junit::declare_suite_end
