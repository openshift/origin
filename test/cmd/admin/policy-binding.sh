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
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/admin/policybinding"
# Admin can't bind local roles without cluster-admin permissions
os::cmd::expect_success "oc process -f test/extended/testdata/roles/policy-roles.yaml -v NAMESPACE='$(oc project -q)' | oc create -f -"
os::cmd::expect_success "oc create -f test/extended/testdata/roles/empty-role.yaml -n '$(oc project -q)'"
os::cmd::expect_success "oc delete 'policybinding/$(oc project -q):default' -n '$(oc project -q)'"
os::cmd::expect_success 'oadm policy add-role-to-user admin local-admin  -n '$(oc project -q)''
os::cmd::try_until_text "oc policy who-can get policybindings -n '$(oc project -q)'" "local-admin"
os::cmd::expect_success 'oc login -u local-admin -p pw'
os::cmd::expect_failure 'oc policy add-role-to-user empty-role other --role-namespace='$(oc project -q)''
os::cmd::expect_success 'oc login -u system:admin'
os::cmd::expect_success "oc create policybinding '$(oc project -q)' -n '$(oc project -q)'"
os::cmd::expect_success 'oc login -u local-admin -p pw'
os::cmd::expect_success 'oc policy add-role-to-user empty-role other --role-namespace='$(oc project -q)' -n '$(oc project -q)''
os::cmd::expect_success 'oc login -u system:admin'
os::cmd::expect_success "oc delete role/empty-role -n '$(oc project -q)'"
echo "policybinding-required: ok"
os::test::junit::declare_suite_end
