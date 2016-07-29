#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/admin/security-context-constraints"
os::cmd::expect_success 'oadm policy who-can get pods'
os::cmd::expect_success 'oadm policy who-can get pods -n default'
os::cmd::expect_success 'oadm policy who-can get pods --all-namespaces'
# check to make sure that the resource arg conforms to resource rules
os::cmd::expect_success_and_text 'oadm policy who-can get Pod' "Resource:  pods"
os::cmd::expect_success_and_text 'oadm policy who-can get PodASDF' "Resource:  PodASDF"
os::cmd::expect_success_and_text 'oadm policy who-can get hpa.autoscaling -n default' "Resource:  horizontalpodautoscalers.autoscaling"
os::cmd::expect_success_and_text 'oadm policy who-can get hpa.v1.autoscaling -n default' "Resource:  horizontalpodautoscalers.autoscaling"
os::cmd::expect_success_and_text 'oadm policy who-can get hpa.extensions -n default' "Resource:  horizontalpodautoscalers.extensions"
os::cmd::expect_success_and_text 'oadm policy who-can get hpa -n default' "Resource:  horizontalpodautoscalers.autoscaling"

os::cmd::expect_success 'oadm policy add-role-to-group cluster-admin system:unauthenticated'
os::cmd::expect_success 'oadm policy add-role-to-user cluster-admin system:no-user'
os::cmd::expect_success 'oadm policy add-role-to-user admin -z fake-sa'
os::cmd::expect_success_and_text 'oc get rolebinding/admin -o jsonpath={.subjects}' 'fake-sa'
os::cmd::expect_success 'oadm policy remove-role-from-user admin -z fake-sa'
os::cmd::expect_success_and_not_text 'oc get rolebinding/admin -o jsonpath={.subjects}' 'fake-sa'
os::cmd::expect_success 'oadm policy add-role-to-user admin -z fake-sa'
os::cmd::expect_success_and_text 'oc get rolebinding/admin -o jsonpath={.subjects}' 'fake-sa'
os::cmd::expect_success "oadm policy remove-role-from-user admin system:serviceaccount:$(oc project -q):fake-sa"
os::cmd::expect_success_and_not_text 'oc get rolebinding/admin -o jsonpath={.subjects}' 'fake-sa'
os::cmd::expect_success 'oadm policy remove-role-from-group cluster-admin system:unauthenticated'
os::cmd::expect_success 'oadm policy remove-role-from-user cluster-admin system:no-user'
os::cmd::expect_success 'oadm policy remove-group system:unauthenticated'
os::cmd::expect_success 'oadm policy remove-user system:no-user'
os::cmd::expect_success 'oadm policy add-cluster-role-to-group cluster-admin system:unauthenticated'
os::cmd::expect_success 'oadm policy remove-cluster-role-from-group cluster-admin system:unauthenticated'
os::cmd::expect_success 'oadm policy add-cluster-role-to-user cluster-admin system:no-user'
os::cmd::expect_success 'oadm policy remove-cluster-role-from-user cluster-admin system:no-user'

os::cmd::expect_success 'oadm policy add-scc-to-user privileged fake-user'
os::cmd::expect_success_and_text 'oc get scc/privileged -o yaml' 'fake-user'
os::cmd::expect_success 'oadm policy add-scc-to-user privileged -z fake-sa'
os::cmd::expect_success_and_text 'oc get scc/privileged -o yaml' "system:serviceaccount:$(oc project -q):fake-sa"
os::cmd::expect_success 'oadm policy add-scc-to-group privileged fake-group'
os::cmd::expect_success_and_text 'oc get scc/privileged -o yaml' 'fake-group'
os::cmd::expect_success 'oadm policy remove-scc-from-user privileged fake-user'
os::cmd::expect_success_and_not_text 'oc get scc/privileged -o yaml' 'fake-user'
os::cmd::expect_success 'oadm policy remove-scc-from-user privileged -z fake-sa'
os::cmd::expect_success_and_not_text 'oc get scc/privileged -o yaml' "system:serviceaccount:$(oc project -q):fake-sa"
os::cmd::expect_success 'oadm policy remove-scc-from-group privileged fake-group'
os::cmd::expect_success_and_not_text 'oc get scc/privileged -o yaml' 'fake-group'
echo "admin-scc: ok"
os::test::junit::declare_suite_end
