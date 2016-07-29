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
  oc delete groups/group1
  oc delete groups/cascaded-group
  oc delete groups/orphaned-group
  oc delete users/cascaded-user
  oc delete users/orphaned-user
  oc delete identities/anypassword:orphaned-user
  oc delete identities/anypassword:cascaded-user
  oadm policy reconcile-cluster-roles --confirm --additive-only=false
  oadm policy reconcile-cluster-role-bindings --confirm --additive-only=false
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/admin/user-group-cascade"
# Create test users/identities and groups
os::cmd::expect_success 'oc login -u cascaded-user -p pw'
os::cmd::expect_success 'oc login -u orphaned-user -p pw'
os::cmd::expect_success 'oc login -u system:admin'
os::cmd::expect_success 'oadm groups new cascaded-group cascaded-user orphaned-user'
os::cmd::expect_success 'oadm groups new orphaned-group cascaded-user orphaned-user'
# Add roles, sccs to users/groups
os::cmd::expect_success 'oadm policy add-scc-to-user           restricted    cascaded-user  orphaned-user'
os::cmd::expect_success 'oadm policy add-scc-to-group          restricted    cascaded-group orphaned-group'
os::cmd::expect_success 'oadm policy add-role-to-user          cluster-admin cascaded-user  orphaned-user  -n default'
os::cmd::expect_success 'oadm policy add-role-to-group         cluster-admin cascaded-group orphaned-group -n default'
os::cmd::expect_success 'oadm policy add-cluster-role-to-user  cluster-admin cascaded-user  orphaned-user'
os::cmd::expect_success 'oadm policy add-cluster-role-to-group cluster-admin cascaded-group orphaned-group'

# Delete users
os::cmd::expect_success 'oc delete user  cascaded-user'
os::cmd::expect_success 'oc delete user  orphaned-user  --cascade=false'
# Verify all identities remain
os::cmd::expect_success 'oc get identities/anypassword:cascaded-user'
os::cmd::expect_success 'oc get identities/anypassword:orphaned-user'
# Verify orphaned user references are left
os::cmd::expect_success_and_text     "oc get clusterrolebindings/cluster-admins --output-version=v1 --template='{{.subjects}}'"            'orphaned-user'
os::cmd::expect_success_and_text     "oc get rolebindings/cluster-admin         --output-version=v1 --template='{{.subjects}}' -n default" 'orphaned-user'
os::cmd::expect_success_and_text     "oc get scc/restricted                     --output-version=v1 --template='{{.users}}'"               'orphaned-user'
os::cmd::expect_success_and_text     "oc get group/cascaded-group               --output-version=v1 --template='{{.users}}'"               'orphaned-user'
# Verify cascaded user references are removed
os::cmd::expect_success_and_not_text "oc get clusterrolebindings/cluster-admins --output-version=v1 --template='{{.subjects}}'"            'cascaded-user'
os::cmd::expect_success_and_not_text "oc get rolebindings/cluster-admin         --output-version=v1 --template='{{.subjects}}' -n default" 'cascaded-user'
os::cmd::expect_success_and_not_text "oc get scc/restricted                     --output-version=v1 --template='{{.users}}'"               'cascaded-user'
os::cmd::expect_success_and_not_text "oc get group/cascaded-group               --output-version=v1 --template='{{.users}}'"               'cascaded-user'

# Delete groups
os::cmd::expect_success 'oc delete group cascaded-group'
os::cmd::expect_success 'oc delete group orphaned-group --cascade=false'
# Verify orphaned group references are left
os::cmd::expect_success_and_text     "oc get clusterrolebindings/cluster-admins --output-version=v1 --template='{{.subjects}}'"            'orphaned-group'
os::cmd::expect_success_and_text     "oc get rolebindings/cluster-admin         --output-version=v1 --template='{{.subjects}}' -n default" 'orphaned-group'
os::cmd::expect_success_and_text     "oc get scc/restricted                     --output-version=v1 --template='{{.groups}}'"              'orphaned-group'
# Verify cascaded group references are removed
os::cmd::expect_success_and_not_text "oc get clusterrolebindings/cluster-admins --output-version=v1 --template='{{.subjects}}'"            'cascaded-group'
os::cmd::expect_success_and_not_text "oc get rolebindings/cluster-admin         --output-version=v1 --template='{{.subjects}}' -n default" 'cascaded-group'
os::cmd::expect_success_and_not_text "oc get scc/restricted                     --output-version=v1 --template='{{.groups}}'"              'cascaded-group'
echo "user-group-cascade: ok"
os::test::junit::declare_suite_end
