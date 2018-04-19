#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/registry/login"
os::cmd::expect_success_and_text "oc registry login -z 'default' --registry=localhost:5000 --to=- --skip-check" "auth"
os::cmd::expect_failure_and_text "oc registry login -z 'default' --registry=localhost2 --to=- 2>&1" "unable to check your credentials"
os::cmd::expect_success_and_text "oc registry login -z 'default' --registry=localhost2 --to=/tmp/test --skip-check && cat /tmp/test" "localhost2"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/registry/info"
os::cmd::expect_success 'oc tag --source=docker openshift/origin-control-plane:latest newrepo:latest'
os::cmd::expect_success "oc registry info"
os::cmd::expect_failure_and_text "oc registry info --internal --public" "only one of --internal or --public"
os::test::junit::declare_suite_end
