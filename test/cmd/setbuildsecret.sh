#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/builds/setbuildsecret"
# Validate the set build-secret command
arg="-f test/testdata/test-bc.yaml"
os::cmd::expect_failure_and_text "oc set build-secret" "error: a secret name must be specified"
os::cmd::expect_failure_and_text "oc set build-secret ${arg}" "error: a secret name must be specified"
os::cmd::expect_failure_and_text "oc set build-secret ${arg} mysecret" "error: specify the type of secret"
os::cmd::expect_success_and_text "oc set build-secret ${arg} mysecret --push --local -o yaml" 'pushSecret:'
os::cmd::expect_success_and_text "oc set build-secret ${arg} mysecret --push --local -o yaml" 'name: mysecret'
os::cmd::expect_success_and_text "oc set build-secret ${arg} mysecret --pull --local -o yaml" 'pullSecret:'
os::cmd::expect_success_and_text "oc set build-secret ${arg} mysecret --pull --local -o yaml" 'name: mysecret'
os::cmd::expect_success_and_text "oc set build-secret ${arg} mysecret --source --local -o yaml" 'sourceSecret:'
os::cmd::expect_success_and_text "oc set build-secret ${arg} mysecret --source --local -o yaml" 'name: mysecret'
os::cmd::expect_success_and_not_text "oc set build-secret ${arg} mysecret --push --local -o yaml | oc set build-secret --local -f - --remove --push -o yaml" 'pushSecret:'
os::cmd::expect_success_and_not_text "oc set build-secret ${arg} mysecret --pull --local -o yaml | oc set build-secret --local -f - --remove --pull -o yaml" 'pullSecret:'
os::cmd::expect_success_and_not_text "oc set build-secret ${arg} mysecret --push --local -o yaml | oc set build-secret --local -f - --remove --source -o yaml" 'sourceSecret:'
# Server object tests
os::cmd::expect_success "oc create -f test/testdata/test-bc.yaml"
os::cmd::expect_success_and_text "oc set build-secret test-buildconfig mysecret --push" "updated"
os::cmd::expect_success_and_text "oc set build-secret bc/test-buildconfig mysecret --push" "was not changed"
os::cmd::expect_success_and_text "oc get bc/test-buildconfig -o yaml" "pushSecret:"
os::cmd::expect_success_and_text "oc set build-secret bc/test-buildconfig --push --remove" "updated"
os::cmd::expect_success_and_not_text "oc get bc/test-buildconfig -o yaml" "pushSecret:"
os::cmd::expect_success "oc delete bc/test-buildconfig"
echo "set build-secret: ok"
os::test::junit::declare_suite_end
