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

os::test::junit::declare_suite_start "cmd/admin/reconcile-cluster-role-bindings"
# Ensure a removed binding gets re-added
os::cmd::expect_success 'oc delete clusterrolebinding/cluster-status-binding'
os::cmd::expect_failure 'oc get clusterrolebinding/cluster-status-binding'
os::cmd::expect_success 'oadm policy reconcile-cluster-role-bindings'
os::cmd::expect_failure 'oc get clusterrolebinding/cluster-status-binding'
os::cmd::expect_success 'oadm policy reconcile-cluster-role-bindings --confirm'
os::cmd::expect_success 'oc get clusterrolebinding/cluster-status-binding'
# Customize a binding
os::cmd::expect_success 'oc replace --force -f ./test/testdata/basic-users-binding.json'
# display shows customized labels/annotations
os::cmd::expect_success_and_text 'oadm policy reconcile-cluster-role-bindings' 'custom-label'
os::cmd::expect_success_and_text 'oadm policy reconcile-cluster-role-bindings' 'custom-annotation'
os::cmd::expect_success 'oadm policy reconcile-cluster-role-bindings --confirm'
# Ensure a customized binding's subjects, labels, annotations are retained by default
os::cmd::expect_success_and_text 'oc get clusterrolebindings/basic-users -o json' 'custom-label'
os::cmd::expect_success_and_text 'oc get clusterrolebindings/basic-users -o json' 'custom-annotation'
os::cmd::expect_success_and_text 'oc get clusterrolebindings/basic-users -o json' 'custom-user'
# Ensure a customized binding's roleref is corrected
os::cmd::expect_success_and_not_text 'oc get clusterrolebindings/basic-users -o json' 'cluster-status'
# Ensure --additive-only=false removes customized users from the binding
os::cmd::expect_success 'oadm policy reconcile-cluster-role-bindings --additive-only=false --confirm'
os::cmd::expect_success_and_not_text 'oc get clusterrolebindings/basic-users -o json' 'custom-user'
# check the reconcile again with a specific cluster role name
os::cmd::expect_success 'oc delete clusterrolebinding/basic-users'
os::cmd::expect_failure 'oc get clusterrolebinding/basic-users'
os::cmd::expect_success 'oadm policy reconcile-cluster-role-bindings cluster-admin --confirm'
os::cmd::expect_failure 'oc get clusterrolebinding/basic-users'
os::cmd::expect_success 'oadm policy reconcile-cluster-role-bindings basic-user --confirm'
os::cmd::expect_success 'oc get clusterrolebinding/basic-users'
echo "admin-reconcile-cluster-role-bindings: ok"
os::test::junit::declare_suite_end
