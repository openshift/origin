#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

project="$( oc project -q )"

os::test::junit::declare_suite_start "cmd/policy-storage-admin"

# Test storage-admin role and impersonation
os::cmd::expect_success 'oc adm policy add-cluster-role-to-user storage-admin storage-adm'
os::cmd::expect_success 'oc adm policy add-cluster-role-to-user storage-admin storage-adm2'
os::cmd::expect_success 'oc adm policy add-role-to-user admin storage-adm2'
os::cmd::expect_success_and_text 'oc policy who-can impersonate storage-admin' 'cluster-admin'

# Test storage-admin role as user level
os::cmd::expect_success 'oc login -u storage-adm -p pw'
os::cmd::expect_success_and_text 'oc whoami' "storage-adm"
os::cmd::expect_failure 'oc whoami --as=basic-user'
os::cmd::expect_failure 'oc whoami --as=cluster-admin'

# Test storage-admin can not do normal project scoped tasks
os::cmd::expect_success_and_text 'oc policy can-i create pods --all-namespaces' 'no'
os::cmd::expect_success_and_text 'oc policy can-i create projects' 'no'
os::cmd::expect_success_and_text 'oc policy can-i get pods --all-namespaces' 'no'
os::cmd::expect_success_and_text 'oc policy can-i create pvc' 'no'

# Test storage-admin can read pvc and create pv and storageclass
os::cmd::expect_success_and_text 'oc policy can-i get pvc --all-namespaces' 'yes'
os::cmd::expect_success_and_text 'oc policy can-i get storageclass' 'yes'
os::cmd::expect_success_and_text 'oc policy can-i create pv' 'yes'
os::cmd::expect_success_and_text 'oc policy can-i create storageclass' 'yes'

# Test failure to change policy on users for storage-admin
os::cmd::expect_failure_and_text 'oc policy add-role-to-user admin storage-adm' 'cannot list rolebindings'
os::cmd::expect_failure_and_text 'oc policy remove-user screeley' 'cannot list rolebindings'
os::cmd::expect_success 'oc logout'

# Test that scoped storage-admin now an admin in project foo
os::cmd::expect_success 'oc login -u storage-adm2 -p pw'
os::cmd::expect_success_and_text 'oc whoami' "storage-adm2"
os::cmd::expect_success 'oc new-project policy-can-i'
os::cmd::expect_success_and_text 'oc policy can-i create pod --all-namespaces' 'no'
os::cmd::expect_success_and_text 'oc policy can-i create pod' 'yes'
os::cmd::expect_success_and_text 'oc policy can-i create pvc' 'yes'
os::cmd::expect_success_and_text 'oc policy can-i create endpoints' 'yes'
os::cmd::expect_success 'oc delete project policy-can-i'
