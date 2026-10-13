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
arg="-f ${TEST_DATA}/test-bc.yaml"
os::cmd::expect_failure_and_text "oc set build-hook" "error: one or more build configs"
os::cmd::expect_failure_and_text "oc set build-hook ${arg}" "error: you must specify a type of hook"
os::cmd::expect_failure_and_text "oc set build-hook ${arg} --local --post-commit -o yaml -- echo 'hello world'" 'you must specify either a script or command for the build hook'
os::cmd::expect_success_and_text "oc set build-hook ${arg} --local --post-commit --command -o yaml -- echo 'hello world'" 'command:'
os::cmd::expect_success_and_text "oc set build-hook ${arg} --local --post-commit -o yaml --script='echo \"hello world\"'" 'script: echo \"hello world\"'
# Server object tests
os::cmd::expect_success "oc create -f ${TEST_DATA}/test-bc.yaml"
# Must specify --command or --script
os::cmd::expect_failure_and_text "oc set build-hook bc/test-buildconfig --post-commit" "you must specify either a script or command"
# Setting args for the default entrypoint is not supported
os::cmd::expect_failure_and_text "oc set build-hook test-buildconfig --post-commit -- foo bar" "you must specify either a script or command for the build hook"
# Set a command + args on a specific build config
os::cmd::expect_success_and_text "oc set build-hook bc/test-buildconfig --post-commit --command -- /bin/bash -c \"echo 'test'\"" "updated"
os::cmd::expect_success_and_text "oc get bc/test-buildconfig -o yaml" "command:"
os::cmd::expect_success_and_text "oc get bc/test-buildconfig -o yaml" "args:"
os::cmd::expect_success_and_not_text "oc get bc/test-buildconfig -o yaml" "script:"
# Set a script on a specific build config
os::cmd::expect_success_and_text "oc set build-hook bc/test-buildconfig --post-commit --script /bin/script.sh -- arg1 arg2" "updated"
os::cmd::expect_success_and_text "oc get bc/test-buildconfig -o yaml" "script:"
os::cmd::expect_success_and_text "oc get bc/test-buildconfig -o yaml" "args:"
os::cmd::expect_success_and_not_text "oc get bc/test-buildconfig -o yaml" "command:"
# Remove the postcommit hook
os::cmd::expect_success_and_text "oc set build-hook bc/test-buildconfig --post-commit --remove" "updated"
os::cmd::expect_success_and_not_text "oc get bc/test-buildconfig -o yaml" "args:"
os::cmd::expect_success_and_not_text "oc get bc/test-buildconfig -o yaml" "command:"
os::cmd::expect_success_and_not_text "oc get bc/test-buildconfig -o yaml" "script:"
# Set a command + args on all build configs
os::cmd::expect_success_and_text "oc set build-hook --all --post-commit --command -- /bin/bash -c \"echo 'test'\"" "updated"
os::cmd::expect_success_and_text "oc get bc/test-buildconfig -o yaml" "command:"
os::cmd::expect_success_and_text "oc get bc/test-buildconfig -o yaml" "args:"
os::cmd::expect_success_and_not_text "oc get bc/test-buildconfig -o yaml" "script:"
# Set a script on all build configs
os::cmd::expect_success_and_text "oc set build-hook --all --post-commit --script /bin/script.sh" "updated"
os::cmd::expect_success_and_text "oc get bc/test-buildconfig -o yaml" "script:"
os::cmd::expect_success_and_not_text "oc get bc/test-buildconfig -o yaml" "args:"
os::cmd::expect_success_and_not_text "oc get bc/test-buildconfig -o yaml" "command:"

os::cmd::expect_success "oc delete bc/test-buildconfig"
# ensure command behaves as expected when an empty file is given
workingdir=$(mktemp -d)
touch "${workingdir}/emptyfile.json"
os::cmd::expect_failure_and_text "oc set build-hook -f ${workingdir}/emptyfile.json --post-commit=true --script=foo" "no resources found"
echo "set build-hook: ok"
os::test::junit::declare_suite_end
