#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oadm policy reconcile-cluster-roles --confirm --additive-only=false
  oadm policy reconcile-cluster-role-bindings --confirm --additive-only=false
) &>/dev/null

os::test::junit::declare_suite_start "cmd/admin/role-reapers"
os::cmd::expect_success "oc process -f test/extended/testdata/roles/policy-roles.yaml -v NAMESPACE='$(oc project -q)' | oc create -f -"
os::cmd::expect_success "oc get rolebinding/basic-users"
os::cmd::expect_success "oc delete role/basic-user"
os::cmd::expect_failure "oc get rolebinding/basic-users"
os::cmd::expect_success "oc create -f test/extended/testdata/roles/policy-clusterroles.yaml"
os::cmd::expect_success "oc get clusterrolebinding/basic-users2"
os::cmd::expect_success "oc delete clusterrole/basic-user2"
os::cmd::expect_failure "oc get clusterrolebinding/basic-users2"
os::cmd::expect_success "oc policy add-role-to-user edit foo"
os::cmd::expect_success "oc get rolebinding/edit"
os::cmd::expect_success "oc delete clusterrole/edit"
os::cmd::expect_failure "oc get rolebinding/edit"
os::cmd::expect_success "oadm policy reconcile-cluster-roles --confirm"
os::cmd::expect_success "oadm policy reconcile-cluster-role-bindings --confirm"
echo "admin-role-reapers: ok"
os::test::junit::declare_suite_end
