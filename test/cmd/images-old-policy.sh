#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

project="$( oc project -q )"
# Cleanup cluster resources created by this test
(
  set +e
  oc login -u system:admin
  oc project "${project}"
  oadm policy reconcile-cluster-roles --additive-only=false --confirm
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/images-old-policy"

os::cmd::expect_success "oadm policy add-role-to-user admin image-user -n '${project}'"
os::cmd::expect_success "oc delete clusterrole/admin --cascade=false"
os::cmd::expect_success "oc create -f '${OS_ROOT}/test/testdata/admin-role-minus-create-istag.yaml'"

os::cmd::try_until_text "oc policy who-can get pods -n ${project}" "image-user"
os::cmd::expect_success "oc login -u image-user -p asdf -n '${project}'"
os::cmd::expect_success "oc project '${project}'"

export IMAGES_TESTS_POSTFIX="-old-policy"
source "${OS_ROOT}/test/cmd/images_tests.sh"

os::cmd::expect_success 'oc login -u system:admin'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles --additive-only=false --confirm'

os::test::junit::declare_suite_end