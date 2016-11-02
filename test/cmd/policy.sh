#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

project="$( oc project -q )"

os::test::junit::declare_suite_start "cmd/policy"
# This test validates user level policy
os::cmd::expect_success_and_text 'oc whoami --as deads' "deads"

os::cmd::expect_success 'oadm policy add-cluster-role-to-user sudoer wheel'
os::cmd::try_until_text 'oc policy who-can impersonate systemusers system:admin' "wheel"
os::cmd::expect_success 'oc login -u wheel -p pw'
os::cmd::expect_success_and_text 'oc whoami' "wheel"
os::cmd::expect_failure 'oc whoami --as deads'
os::cmd::expect_success_and_text 'oc whoami --as=system:admin' "system:admin"
os::cmd::expect_success_and_text 'oc policy can-i --list --as=system:admin' '.*'

os::cmd::expect_success 'oc login -u local-admin -p pw'
os::cmd::expect_success 'oc new-project foo'
os::cmd::expect_failure 'oc whoami --as=system:admin'
os::cmd::expect_success_and_text 'oc whoami --as=system:serviceaccount:foo:default' "system:serviceaccount:foo:default"
os::cmd::expect_failure 'oc whoami --as=system:serviceaccount:another:default'
os::cmd::expect_success "oc login -u system:admin -n '${project}'"
os::cmd::expect_success 'oc delete project foo'


# This test validates user level policy
os::cmd::expect_failure_and_text 'oc policy add-role-to-user' 'you must specify a role'
os::cmd::expect_failure_and_text 'oc policy add-role-to-user -z NamespaceWithoutRole' 'you must specify a role'
os::cmd::expect_failure_and_text 'oc policy add-role-to-user view' 'you must specify at least one user or service account'

os::cmd::expect_success 'oc policy add-role-to-group cluster-admin system:unauthenticated'
os::cmd::expect_success 'oc policy add-role-to-user cluster-admin system:no-user'
os::cmd::expect_success 'oc get rolebinding/cluster-admin --no-headers'
os::cmd::expect_success_and_text 'oc get rolebinding/cluster-admin --no-headers' 'system:no-user'

os::cmd::expect_success 'oc policy add-role-to-user cluster-admin -z=one,two --serviceaccount=three,four'
os::cmd::expect_success 'oc get rolebinding/cluster-admin --no-headers'
os::cmd::expect_success_and_text 'oc get rolebinding/cluster-admin --no-headers' 'one'
os::cmd::expect_success_and_text 'oc get rolebinding/cluster-admin --no-headers' 'four'

os::cmd::expect_success 'oc policy remove-role-from-group cluster-admin system:unauthenticated'

os::cmd::expect_success 'oc policy remove-role-from-user cluster-admin system:no-user'
os::cmd::expect_success 'oc policy remove-role-from-user cluster-admin -z=one,two --serviceaccount=three,four'
os::cmd::expect_success 'oc get rolebinding/cluster-admin --no-headers'
os::cmd::expect_success_and_not_text 'oc get rolebinding/cluster-admin --no-headers' 'four'

os::cmd::expect_success 'oc policy remove-group system:unauthenticated'
os::cmd::expect_success 'oc policy remove-user system:no-user'


os::cmd::expect_success 'oc policy add-role-to-user admin namespaced-user'
# Ensure the user has create permissions on builds, but that build strategy permissions are granted through the authenticated users group
os::cmd::try_until_text              'oadm policy who-can create builds' 'namespaced-user'
os::cmd::expect_success_and_not_text 'oadm policy who-can create builds/docker' 'namespaced-user'
os::cmd::expect_success_and_not_text 'oadm policy who-can create builds/custom' 'namespaced-user'
os::cmd::expect_success_and_not_text 'oadm policy who-can create builds/source' 'namespaced-user'
os::cmd::expect_success_and_not_text 'oadm policy who-can create builds/jenkinspipeline' 'namespaced-user'
os::cmd::expect_success_and_text     'oadm policy who-can create builds/docker' 'system:authenticated'
os::cmd::expect_success_and_text     'oadm policy who-can create builds/source' 'system:authenticated'
os::cmd::expect_success_and_text     'oadm policy who-can create builds/jenkinspipeline' 'system:authenticated'
# if this method for removing access to docker/custom/source/jenkinspipeline builds changes, docs need to be updated as well
os::cmd::expect_success 'oadm policy remove-cluster-role-from-group system:build-strategy-docker system:authenticated'
os::cmd::expect_success 'oadm policy remove-cluster-role-from-group system:build-strategy-source system:authenticated'
os::cmd::expect_success 'oadm policy remove-cluster-role-from-group system:build-strategy-jenkinspipeline system:authenticated'
# ensure build strategy permissions no longer exist
os::cmd::try_until_failure           'oadm policy who-can create builds/source | grep system:authenticated'
os::cmd::expect_success_and_not_text 'oadm policy who-can create builds/docker' 'system:authenticated'
os::cmd::expect_success_and_not_text 'oadm policy who-can create builds/source' 'system:authenticated'
os::cmd::expect_success_and_not_text 'oadm policy who-can create builds/jenkinspipeline' 'system:authenticated'

# ensure system:authenticated users can not create custom builds by default, but can if explicitly granted access
os::cmd::expect_success_and_not_text 'oadm policy who-can create builds/custom' 'system:authenticated'
os::cmd::expect_success 'oadm policy add-cluster-role-to-group system:build-strategy-custom system:authenticated'
os::cmd::expect_success_and_text 'oadm policy who-can create builds/custom' 'system:authenticated'

os::cmd::expect_success 'oadm policy reconcile-cluster-role-bindings --confirm'

os::cmd::expect_success_and_text 'oc policy can-i --list' 'get update.*imagestreams/layers'
os::cmd::expect_success_and_text 'oc policy can-i create pods --all-namespaces' 'yes'
os::cmd::expect_success_and_text 'oc policy can-i create pods' 'yes'
os::cmd::expect_success_and_text 'oc policy can-i create pods --as harold' 'no'
os::cmd::expect_failure 'oc policy can-i create pods --as harold --user harold'
os::cmd::expect_failure 'oc policy can-i --list --as harold --user harold'
os::cmd::expect_failure 'oc policy can-i create pods --as harold -q'

os::cmd::expect_success_and_text 'oc policy can-i create pods --user system:admin' 'yes'
os::cmd::expect_success_and_text 'oc policy can-i create pods --groups system:unauthenticated --groups system:masters' 'yes'
os::cmd::expect_success_and_text 'oc policy can-i create pods --groups system:unauthenticated' 'no'
os::cmd::expect_success_and_text 'oc policy can-i create pods --user harold' 'no'

os::cmd::expect_success_and_text 'oc policy can-i --list --user system:admin' 'get update.*imagestreams/layers'
os::cmd::expect_success_and_text 'oc policy can-i --list --groups system:unauthenticated --groups system:cluster-readers' 'get.*imagestreams/layers'
os::cmd::expect_success_and_not_text 'oc policy can-i --list --groups system:unauthenticated' 'get update.*imagestreams/layers'
os::cmd::expect_success_and_not_text 'oc policy can-i --list --user harold --groups system:authenticated' 'get update.*imagestreams/layers'
os::cmd::expect_success_and_text 'oc policy can-i --list --user harold --groups system:authenticated' 'create get.*buildconfigs/webhooks'



# adjust the cluster-admin role to check defaulting and coverage checks
# this is done here instead of an integration test because we need to make sure the actual yaml serializations work
workingdir=$(mktemp -d)
cp ${OS_ROOT}/test/testdata/bootstrappolicy/cluster_admin_1.0.yaml ${workingdir}
os::util::sed "s/RESOURCE_VERSION//g" ${workingdir}/cluster_admin_1.0.yaml
os::cmd::expect_success "oc create -f ${workingdir}/cluster_admin_1.0.yaml"
os::cmd::expect_success 'oadm policy add-cluster-role-to-user alternate-cluster-admin alternate-cluster-admin-user'

# switch to test user to be sure that default project admin policy works properly
new_kubeconfig="${workingdir}/tempconfig"
os::cmd::expect_success "oc config view --raw > $new_kubeconfig"
os::cmd::expect_success "oc login -u alternate-cluster-admin-user -p anything --config=${new_kubeconfig}"

# alternate-cluster-admin should default to having star rights, so he should be able to update his role to that
os::cmd::try_until_text "oc policy who-can update clusterrroles" "alternate-cluster-admin-user"
resourceversion=$(oc get clusterrole/alternate-cluster-admin -o=jsonpath="{.metadata.resourceVersion}")
cp ${OS_ROOT}/test/testdata/bootstrappolicy/alternate_cluster_admin.yaml ${workingdir}
os::util::sed "s/RESOURCE_VERSION/${resourceversion}/g" ${workingdir}/alternate_cluster_admin.yaml
os::cmd::expect_success "oc replace --config=${new_kubeconfig} clusterrole/alternate-cluster-admin -f ${workingdir}/alternate_cluster_admin.yaml"

# alternate-cluster-admin can restrict himself to no groups
os::cmd::try_until_text "oc policy who-can update clusterrroles" "alternate-cluster-admin-user"
resourceversion=$(oc get clusterrole/alternate-cluster-admin -o=jsonpath="{.metadata.resourceVersion}")
cp ${OS_ROOT}/test/testdata/bootstrappolicy/cluster_admin_without_apigroups.yaml ${workingdir}
os::util::sed "s/RESOURCE_VERSION/${resourceversion}/g" ${workingdir}/cluster_admin_without_apigroups.yaml
os::cmd::expect_success "oc replace --config=${new_kubeconfig} clusterrole/alternate-cluster-admin -f ${workingdir}/cluster_admin_without_apigroups.yaml"

# alternate-cluster-admin should NOT have the power add back star now
os::cmd::try_until_failure "oc policy who-can update hpa.extensions | grep -q alternate-cluster-admin-user"
resourceversion=$(oc get clusterrole/alternate-cluster-admin -o=jsonpath="{.metadata.resourceVersion}")
cp ${OS_ROOT}/test/testdata/bootstrappolicy/alternate_cluster_admin.yaml ${workingdir}
os::util::sed "s/RESOURCE_VERSION/${resourceversion}/g" ${workingdir}/alternate_cluster_admin.yaml
os::cmd::expect_failure_and_text "oc replace --config=${new_kubeconfig} clusterrole/alternate-cluster-admin -f ${workingdir}/alternate_cluster_admin.yaml" "cannot grant extra privileges"

echo "policy: ok"
os::test::junit::declare_suite_end
