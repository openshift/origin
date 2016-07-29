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

os::test::junit::declare_suite_start "cmd/admin/reconcile-cluster-roles"
os::cmd::expect_success 'oc delete clusterrole/cluster-status --cascade=false'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles --confirm --loglevel=8'
os::cmd::expect_success 'oc get clusterrole/cluster-status'
# check the reconcile again with a specific cluster role name
os::cmd::expect_success 'oc delete clusterrole/cluster-status --cascade=false'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles cluster-admin --confirm'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles clusterrole/cluster-status --confirm'
os::cmd::expect_success 'oc get clusterrole/cluster-status'

# test reconciliation protection by replacing the basic-user role with one that has missing default permissions, and extra non-default permissions
os::cmd::expect_success 'oc replace --force -f ./test/testdata/basic-user-with-groups-without-projectrequests.yaml'
# 1. mark the role as protected, and ensure the role is skipped by reconciliation
os::cmd::expect_success 'oc annotate clusterrole/basic-user openshift.io/reconcile-protect=true'
os::cmd::expect_success_and_text     'oadm policy reconcile-cluster-roles basic-user --additive-only=false --confirm' 'skipped: clusterrole/basic-user'
# 2. unmark the role as protected, and ensure reconcile expects to remove extra permissions, and put back removed permissions
os::cmd::expect_success 'oc annotate clusterrole/basic-user openshift.io/reconcile-protect=false --overwrite'
os::cmd::expect_success_and_text     'oc get clusterrole/basic-user -o jsonpath="{.rules[*].resources}"' 'groups'
os::cmd::expect_success_and_not_text 'oc get clusterrole/basic-user -o jsonpath="{.rules[*].resources}"' 'projectrequests'
os::cmd::expect_success_and_not_text 'oadm policy reconcile-cluster-roles basic-user -o jsonpath="{.items[*].rules[*].resources}" --additive-only=false' 'groups'
os::cmd::expect_success_and_text     'oadm policy reconcile-cluster-roles basic-user -o jsonpath="{.items[*].rules[*].resources}" --additive-only=false' 'projectrequests'
# reconcile updates the role
os::cmd::expect_success_and_text     'oadm policy reconcile-cluster-roles basic-user --additive-only=false --confirm' 'clusterrole/basic-user'
# a second reconcile doesn't need to update the role
os::cmd::expect_success_and_not_text 'oadm policy reconcile-cluster-roles basic-user --additive-only=false --confirm' 'clusterrole/basic-user'

# test label/annotation reconciliation by replacing the basic-user role with one that has custom labels, annotations, and permissions
os::cmd::expect_success 'oc replace --force -f ./test/testdata/basic-user-with-annotations-labels-groups-without-projectrequests.yaml'
# display shows customized labels/annotations
os::cmd::expect_success_and_text 'oadm policy reconcile-cluster-roles' 'custom-label'
os::cmd::expect_success_and_text 'oadm policy reconcile-cluster-roles' 'custom-annotation'
os::cmd::expect_success_and_text 'oadm policy reconcile-cluster-roles --additive-only --confirm' 'clusterrole/basic-user'
# reconcile preserves added rules, labels, and annotations
os::cmd::expect_success_and_text 'oc get clusterroles/basic-user -o json' 'custom-label'
os::cmd::expect_success_and_text 'oc get clusterroles/basic-user -o json' 'custom-annotation'
os::cmd::expect_success_and_text 'oc get clusterroles/basic-user -o json' 'groups'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles --additive-only=false --confirm'
os::cmd::expect_success_and_not_text 'oc get clusterroles/basic-user -o yaml' 'groups'
echo "admin-reconcile-cluster-roles: ok"
os::test::junit::declare_suite_end
