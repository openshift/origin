#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

project="$( oc project -q )"
testpod="apiVersion: v1
kind: Pod
metadata:
  name: testpod
spec:
  containers:
  - image: node
    imagePullPolicy: IfNotPresent
    name: testpod
  volumes:
  - emptyDir: {}
    name: tmp"

os::test::junit::declare_suite_start "cmd/policy"
# This test validates user level policy
os::cmd::expect_success_and_text 'oc whoami --as deads' "deads"

os::cmd::expect_success 'oc adm policy add-cluster-role-to-user sudoer wheel'
os::cmd::try_until_text 'oc policy who-can impersonate users system:admin' "wheel"
os::cmd::try_until_text 'oc policy who-can impersonate groups system:masters' "wheel"
os::cmd::try_until_text 'oc policy who-can impersonate systemusers system:admin' "wheel"
os::cmd::try_until_text 'oc policy who-can impersonate systemgroups system:masters' "wheel"
os::cmd::expect_success 'oc login -u wheel -p pw'
os::cmd::expect_success_and_text 'oc whoami' "wheel"
os::cmd::expect_failure 'oc whoami --as deads'
os::cmd::expect_success_and_text 'oc whoami --as=system:admin' "system:admin"
os::cmd::expect_success_and_text 'oc policy can-i --list --as=system:admin' '.*'

os::cmd::expect_success 'oc login -u local-admin -p pw'
os::cmd::expect_success 'oc new-project policy-login'
os::cmd::expect_failure 'oc whoami --as=system:admin'
os::cmd::expect_success_and_text 'oc whoami --as=system:serviceaccount:policy-login:default' "system:serviceaccount:policy-login:default"
os::cmd::expect_failure 'oc whoami --as=system:serviceaccount:another:default'
os::cmd::expect_success "oc login -u system:admin -n '${project}'"
os::cmd::expect_success 'oc delete project policy-login'

# validate --serviceaccount values
os::cmd::expect_success_and_text 'oc policy add-role-to-user admin -z default' 'role "admin" added\: "default"'
os::cmd::expect_failure_and_text 'oc policy add-role-to-user admin -z system:serviceaccount:test:default' 'should only be used with short\-form serviceaccount names'
os::cmd::expect_failure_and_text 'oc policy add-role-to-user admin -z :invalid' '"\:invalid" is not a valid serviceaccount name'

# This test validates user level policy
os::cmd::expect_failure_and_text 'oc policy add-role-to-user' 'you must specify a role'
os::cmd::expect_failure_and_text 'oc policy add-role-to-user -z NamespaceWithoutRole' 'you must specify a role'
os::cmd::expect_failure_and_text 'oc policy add-role-to-user view' 'you must specify at least one user or service account'

os::cmd::expect_success_and_text 'oc policy add-role-to-group cluster-admin --rolebinding-name cluster-admin system:unauthenticated' 'role "cluster-admin" added: "system:unauthenticated"'
os::cmd::expect_success_and_text 'oc policy add-role-to-user --rolebinding-name cluster-admin cluster-admin system:no-user' 'role "cluster-admin" added: "system:no-user"'
os::cmd::expect_success 'oc get rolebinding/cluster-admin --no-headers'
os::cmd::expect_success_and_text 'oc get rolebinding/cluster-admin --no-headers' 'system:no-user'

os::cmd::expect_success_and_text 'oc policy add-role-to-user --rolebinding-name cluster-admin cluster-admin -z=one,two --serviceaccount=three,four' 'role "cluster-admin" added: \["one" "two" "three" "four"\]'
os::cmd::expect_success 'oc get rolebinding/cluster-admin --no-headers'
os::cmd::expect_success_and_text 'oc get rolebinding/cluster-admin --no-headers' 'one'
os::cmd::expect_success_and_text 'oc get rolebinding/cluster-admin --no-headers' 'four'

os::cmd::expect_success_and_text 'oc policy remove-role-from-group --rolebinding-name cluster-admin cluster-admin system:unauthenticated' 'role "cluster-admin" removed: "system:unauthenticated"'

os::cmd::expect_success_and_text 'oc policy remove-role-from-user --rolebinding-name cluster-admin cluster-admin system:no-user' 'role "cluster-admin" removed: "system:no-user"'
os::cmd::expect_success_and_text 'oc policy remove-role-from-user --rolebinding-name cluster-admin cluster-admin -z=one,two --serviceaccount=three,four' 'role "cluster-admin" removed: \["one" "two" "three" "four"\]'
os::cmd::expect_failure_and_text 'oc get rolebinding/cluster-admin --no-headers' 'NotFound'

os::cmd::expect_success 'oc policy remove-group system:unauthenticated'
os::cmd::expect_success 'oc policy remove-user system:no-user'

# Test failure to mix and mismatch role/rolebiding removal
os::cmd::expect_success 'oc login -u local-admin -p pw'
os::cmd::expect_success 'oc new-project mismatch-prj'
os::cmd::expect_success 'oc create rolebinding match --clusterrole=admin --user=user'
os::cmd::expect_success 'oc create rolebinding mismatch --clusterrole=edit --user=user'
os::cmd::expect_failure_and_text 'oc policy remove-role-from-user admin user --rolebinding-name mismatch' 'rolebinding mismatch'
os::cmd::expect_success_and_text 'oc policy remove-user user' 'user'
os::cmd::expect_failure_and_text 'oc get rolebinding mismatch --no-headers' 'NotFound'
os::cmd::expect_failure_and_text 'oc get rolebinding match --no-headers' 'NotFound'
os::cmd::expect_success "oc login -u system:admin -n '${project}'"
os::cmd::expect_success 'oc delete project mismatch-prj'

# check to make sure that our SCC policies don't prevent GC from deleting pods
os::cmd::expect_success 'oc create -f ${OS_ROOT}/test/testdata/privileged-pod.yaml'
os::cmd::expect_success 'oc delete pod/test-build-pod-issue --cascade=false'
os::cmd::try_until_failure 'oc get pods pod/test-build-pod-issue'


os::cmd::expect_success_and_text 'oc policy add-role-to-user admin namespaced-user' 'role "admin" added: "namespaced-user"'
# Ensure the user has create permissions on builds, but that build strategy permissions are granted through the authenticated users group
os::cmd::try_until_text              'oc adm policy who-can create builds' 'namespaced-user'
os::cmd::expect_success_and_not_text 'oc adm policy who-can create builds/docker' 'namespaced-user'
os::cmd::expect_success_and_not_text 'oc adm policy who-can create builds/custom' 'namespaced-user'
os::cmd::expect_success_and_not_text 'oc adm policy who-can create builds/source' 'namespaced-user'
os::cmd::expect_success_and_not_text 'oc adm policy who-can create builds/jenkinspipeline' 'namespaced-user'
os::cmd::expect_success_and_text     'oc adm policy who-can create builds/docker' 'system:authenticated'
os::cmd::expect_success_and_text     'oc adm policy who-can create builds/source' 'system:authenticated'
os::cmd::expect_success_and_text     'oc adm policy who-can create builds/jenkinspipeline' 'system:authenticated'
# if this method for removing access to docker/custom/source/jenkinspipeline builds changes, docs need to be updated as well
os::cmd::expect_success_and_text 'oc adm policy remove-cluster-role-from-group system:build-strategy-docker system:authenticated' 'cluster role "system:build-strategy-docker" removed: "system:authenticated"'
os::cmd::expect_success_and_text 'oc adm policy remove-cluster-role-from-group system:build-strategy-source system:authenticated' 'cluster role "system:build-strategy-source" removed: "system:authenticated"'
os::cmd::expect_success_and_text 'oc adm policy remove-cluster-role-from-group system:build-strategy-jenkinspipeline system:authenticated' 'cluster role "system:build-strategy-jenkinspipeline" removed: "system:authenticated"'

# ensure build strategy permissions no longer exist
os::cmd::try_until_failure           'oc adm policy who-can create builds/source | grep system:authenticated'
os::cmd::expect_success_and_not_text 'oc adm policy who-can create builds/docker' 'system:authenticated'
os::cmd::expect_success_and_not_text 'oc adm policy who-can create builds/source' 'system:authenticated'
os::cmd::expect_success_and_not_text 'oc adm policy who-can create builds/jenkinspipeline' 'system:authenticated'

# validate --output and --dry-run flags for oc-adm-policy sub-commands
os::cmd::expect_success_and_text 'oc adm policy remove-role-from-user admin namespaced-user -o yaml' 'name: admin'
os::cmd::expect_success_and_text 'oc adm policy add-role-to-user admin namespaced-user -o yaml' 'name: namespaced-user'

os::cmd::expect_success_and_text 'oc adm policy remove-role-from-user admin namespaced-user --dry-run' 'role "admin" removed: "namespaced\-user" \(dry run\)'
os::cmd::expect_success_and_text 'oc adm policy add-role-to-user admin namespaced-user --dry-run' 'role "admin" added: "namespaced\-user" \(dry run\)'

# ensure that running an `oc adm policy` sub-command with --output does not actually perform any changes
os::cmd::expect_success_and_text 'oc adm policy who-can create pods -o yaml' '\- namespaced\-user'

os::cmd::expect_success_and_text 'oc adm policy scc-subject-review -u namespaced-user --output yaml -f - << __EOF__
$testpod
__EOF__' 'name: testpod'
os::cmd::expect_success_and_text 'oc adm policy scc-subject-review -u namespaced-user --output wide -f - << __EOF__
$testpod
__EOF__' 'Pod/testpod'

os::cmd::expect_success_and_text 'oc adm policy scc-review --output yaml -f - << __EOF__
$testpod
__EOF__' 'allowedServiceAccounts: null'

os::cmd::expect_success_and_text 'oc adm policy add-role-to-group view testgroup -o yaml' 'name: view'
os::cmd::expect_success_and_text 'oc adm policy add-cluster-role-to-group cluster-reader testgroup -o yaml' '\- testgroup'
os::cmd::expect_success_and_text 'oc adm policy add-cluster-role-to-user cluster-reader namespaced-user -o yaml' 'name: namespaced\-user'

os::cmd::expect_success_and_text 'oc adm policy add-role-to-group view testgroup --dry-run' 'role "view" added: "testgroup" \(dry run\)'
os::cmd::expect_success_and_text 'oc adm policy add-cluster-role-to-group cluster-reader testgroup --dry-run' 'cluster role "cluster\-reader" added: "testgroup" \(dry run\)'
os::cmd::expect_success_and_text 'oc adm policy add-cluster-role-to-user cluster-reader namespaced-user --dry-run' 'cluster role "cluster\-reader" added: "namespaced\-user" \(dry run\)'

os::cmd::expect_success 'oc adm policy add-role-to-group view testgroup'
os::cmd::expect_success 'oc adm policy add-cluster-role-to-group cluster-reader testgroup'
os::cmd::expect_success 'oc adm policy add-cluster-role-to-user cluster-reader namespaced-user'

# ensure that removing missing target causes error.
os::cmd::expect_failure_and_text 'oc adm policy remove-cluster-role-from-user admin ghost' 'error: unable to find target \[ghost\]'
os::cmd::expect_failure_and_text 'oc adm policy remove-cluster-role-from-user admin -z ghost' 'error: unable to find target \[ghost\]'

os::cmd::expect_success_and_text 'oc adm policy remove-role-from-group view testgroup -o yaml' 'subjects: \[\]'
os::cmd::expect_success_and_text 'oc adm policy remove-cluster-role-from-group cluster-reader testgroup -o yaml' 'name: cluster\-readers'
os::cmd::expect_success_and_text 'oc adm policy remove-cluster-role-from-user cluster-reader namespaced-user -o yaml' 'name: cluster\-reader'

os::cmd::expect_success_and_text 'oc adm policy remove-role-from-group view testgroup --dry-run' 'role "view" removed: "testgroup" \(dry run\)'
os::cmd::expect_success_and_text 'oc adm policy remove-cluster-role-from-group cluster-reader testgroup --dry-run' 'cluster role "cluster\-reader" removed: "testgroup" \(dry run\)'
os::cmd::expect_success_and_text 'oc adm policy remove-cluster-role-from-user cluster-reader namespaced-user --dry-run' 'cluster role "cluster\-reader" removed: "namespaced\-user" \(dry run\)'

os::cmd::expect_success_and_text 'oc adm policy remove-user namespaced-user -o yaml' "namespace: ${project}"
os::cmd::expect_success_and_text 'oc adm policy remove-user namespaced-user --dry-run' "Removing admin from users \[namespaced\-user\] in project ${project}"

os::cmd::expect_success_and_text 'oc adm policy add-scc-to-user anyuid namespaced-user -o yaml' '\- namespaced\-user'
os::cmd::expect_success_and_text 'oc adm policy add-scc-to-user anyuid namespaced-user --dry-run' 'scc "anyuid" added to: \["namespaced\-user"\] \(dry run\)'

os::cmd::expect_success_and_text 'oc adm policy add-scc-to-group anyuid testgroup -o yaml' '\- testgroup'
os::cmd::expect_success_and_text 'oc adm policy add-scc-to-group anyuid testgroup --dry-run' 'scc "anyuid" added to groups: \["testgroup"\] \(dry run\)'

os::cmd::expect_success_and_not_text 'oc adm policy remove-scc-from-user anyuid namespaced-user -o yaml' '\- namespaced\-user'
os::cmd::expect_success_and_text 'oc adm policy remove-scc-from-user anyuid namespaced-user --dry-run' 'scc "anyuid" removed from: \["namespaced\-user"\] \(dry run\)'

os::cmd::expect_success_and_not_text 'oc adm policy remove-scc-from-group anyuid testgroup -o yaml' '\- testgroup'
os::cmd::expect_success_and_text 'oc adm policy remove-scc-from-group anyuid testgroup --dry-run' 'scc "anyuid" removed from groups: \["testgroup"\] \(dry run\)'

# ensure system:authenticated users can not create custom builds by default, but can if explicitly granted access
os::cmd::expect_success_and_not_text 'oc adm policy who-can create builds/custom' 'system:authenticated'
os::cmd::expect_success_and_text 'oc adm policy add-cluster-role-to-group system:build-strategy-custom system:authenticated' 'cluster role "system:build-strategy-custom" added: "system:authenticated"'
os::cmd::expect_success_and_text 'oc adm policy who-can create builds/custom' 'system:authenticated'

os::cmd::expect_success 'oc adm policy reconcile-cluster-role-bindings --confirm'

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


os::cmd::expect_failure 'oc policy scc-subject-review'
os::cmd::expect_failure 'oc policy scc-review'
os::cmd::expect_failure 'oc policy scc-subject-review -u invalid --namespace=noexist'
os::cmd::expect_failure_and_text 'oc policy scc-subject-review -f ${OS_ROOT}/test/testdata/pspreview_unsupported_statefulset.yaml' 'error: StatefulSet "rd" with spec.volumeClaimTemplates currently not supported.'
os::cmd::expect_failure_and_text 'oc policy scc-subject-review -z foo,bar -f ${OS_ROOT}/test/testdata/job.yaml'  'error: only one Service Account is supported'
os::cmd::expect_failure_and_text 'oc policy scc-subject-review -z system:serviceaccount:test:default,system:serviceaccount:test:builder -f ${OS_ROOT}/test/testdata/job.yaml'  'error: only one Service Account is supported'
os::cmd::expect_failure_and_text 'oc policy scc-review -f ${OS_ROOT}/test/testdata/pspreview_unsupported_statefulset.yaml' 'error: StatefulSet "rd" with spec.volumeClaimTemplates currently not supported.'
os::cmd::expect_success_and_text 'oc policy scc-subject-review -f ${OS_ROOT}/test/testdata/job.yaml -o=jsonpath={.status.AllowedBy.name}' 'anyuid'
os::cmd::expect_success_and_text 'oc policy scc-subject-review -f ${OS_ROOT}/test/testdata/redis-slave.yaml -o=jsonpath={.status.AllowedBy.name}' 'anyuid'
# In the past system:admin only had access to a few SCCs, so the following command resulted in the privileged SCC being used
# Since SCCs are now authorized via RBAC, and system:admin can perform all RBAC actions == system:admin can access all SCCs now
# Thus the following command now results in the use of the hostnetwork SCC which is the most restrictive SCC that still allows the pod to run
os::cmd::expect_success_and_text 'oc policy scc-subject-review -f ${OS_ROOT}/test/testdata/nginx_pod.yaml -o=jsonpath={.status.AllowedBy.name}' 'hostnetwork'
# Make sure that the legacy ungroupified objects continue to work by directly doing a create
os::cmd::expect_success_and_text 'oc create -f ${OS_ROOT}/test/testdata/legacy_ungroupified_psp_review.yaml -o=jsonpath={.status.allowedBy.name}' 'restricted'
os::cmd::expect_success "oc login -u bob -p bobpassword"
os::cmd::expect_success_and_text 'oc whoami' 'bob'
os::cmd::expect_success 'oc new-project policy-second'
os::cmd::expect_success_and_text 'oc policy scc-subject-review -f ${OS_ROOT}/test/testdata/job.yaml -o=jsonpath={.status.AllowedBy.name}' 'restricted'
os::cmd::expect_success_and_text 'oc policy scc-subject-review -f ${OS_ROOT}/test/testdata/job.yaml --no-headers=true' 'Job/hello   restricted'
os::cmd::expect_success_and_text 'oc policy scc-subject-review -f ${OS_ROOT}/test/testdata/two_jobs.yaml -o=jsonpath={.status.AllowedBy.name}' 'restrictedrestricted'
os::cmd::expect_success_and_text 'oc policy scc-review -f ${OS_ROOT}/test/testdata/job.yaml -ojsonpath={.status.allowedServiceAccounts}' '\[\]'
os::cmd::expect_success_and_text 'oc policy scc-review -f ${OS_ROOT}/test/extended/testdata/deployments/deployment-simple.yaml -ojsonpath={.status.allowedServiceAccounts}' '\[\]'
os::cmd::expect_failure 'oc policy scc-subject-review -f ${OS_ROOT}/test/testdata/external-service.yaml'
os::cmd::expect_success "oc login -u system:admin -n '${project}'"
os::cmd::expect_success_and_text 'oc policy scc-subject-review -u bob -g system:authenticated -f ${OS_ROOT}/test/testdata/job.yaml -n policy-second -o=jsonpath={.status.allowedBy.name}' 'restricted'
os::cmd::expect_success_and_text 'oc policy scc-subject-review -u bob -f ${OS_ROOT}/test/testdata/job.yaml -n policy-second --no-headers=true' 'Job/hello   <none>'
os::cmd::expect_success_and_text 'oc policy scc-subject-review -z default -f ${OS_ROOT}/test/testdata/job.yaml' ''
os::cmd::expect_success_and_text 'oc policy scc-subject-review -z default -g system:authenticated -f ${OS_ROOT}/test/testdata/job.yaml' 'restricted'
os::cmd::expect_failure_and_text 'oc policy scc-subject-review -u alice -z default -g system:authenticated -f ${OS_ROOT}/test/testdata/job.yaml' 'error: --user and --serviceaccount are mutually exclusive'
os::cmd::expect_success_and_text 'oc policy scc-subject-review -z system:serviceaccount:alice:default -g system:authenticated -f ${OS_ROOT}/test/testdata/job.yaml' 'restricted'
os::cmd::expect_success_and_text 'oc policy scc-subject-review -u alice -g system:authenticated -f ${OS_ROOT}/test/testdata/job.yaml' 'restricted'
os::cmd::expect_failure_and_text 'oc policy scc-subject-review -u alice -g system:authenticated -n noexist -f ${OS_ROOT}/test/testdata/job.yaml' 'error: unable to compute Pod Security Policy Subject Review for "hello": namespaces "noexist" not found'
os::cmd::expect_success 'oc create -f ${OS_ROOT}/test/testdata/scc_lax.yaml'
os::cmd::expect_success "oc login -u bob -p bobpassword"
os::cmd::expect_success_and_text 'oc policy scc-review -f ${OS_ROOT}/test/testdata/job.yaml --no-headers=true' 'Job/hello   default   lax'
os::cmd::expect_success_and_text 'oc policy scc-review -z default  -f ${OS_ROOT}/test/testdata/job.yaml --no-headers=true' 'Job/hello   default   lax'
os::cmd::expect_success_and_text 'oc policy scc-review -z system:serviceaccount:policy-second:default  -f ${OS_ROOT}/test/testdata/job.yaml --no-headers=true' 'Job/hello   default   lax'
os::cmd::expect_success_and_text 'oc policy scc-review -f ${OS_ROOT}/test/extended/testdata/deployments/deployment-simple.yaml --no-headers=true' 'DeploymentConfig/deployment-simple   default   lax'
os::cmd::expect_success_and_text 'oc policy scc-review -f ${OS_ROOT}/test/testdata/nginx_pod.yaml --no-headers=true' ''
os::cmd::expect_failure_and_text 'oc policy scc-review -z default -f ${OS_ROOT}/test/testdata/job.yaml --namespace=no-exist' 'error: unable to compute Pod Security Policy Review for "hello": podsecuritypolicyreviews.security.openshift.io is forbidden: User "bob" cannot create podsecuritypolicyreviews.security.openshift.io in the namespace "no-exist": User "bob" cannot create podsecuritypolicyreviews.security.openshift.io in project "no-exist"'
os::cmd::expect_failure_and_text 'oc policy scc-review -z default -f ${OS_ROOT}/test/testdata/pspreview_unsupported_statefulset.yaml' 'error: StatefulSet "rd" with spec.volumeClaimTemplates currently not supported.'
os::cmd::expect_failure_and_text 'oc policy scc-review -z no-exist -f ${OS_ROOT}/test/testdata/job.yaml' 'error: unable to compute Pod Security Policy Review for "hello": unable to retrieve ServiceAccount no-exist: serviceaccount "no-exist" not found'
os::cmd::expect_success "oc login -u system:admin -n '${project}'"
os::cmd::expect_success 'oc delete project policy-second'

# adjust the cluster-admin role to check defaulting and coverage checks
# this is done here instead of an integration test because we need to make sure the actual yaml serializations work
workingdir=$(mktemp -d)
cp ${OS_ROOT}/test/testdata/bootstrappolicy/cluster_admin_1.0.yaml ${workingdir}
os::util::sed "s/RESOURCE_VERSION//g" ${workingdir}/cluster_admin_1.0.yaml
os::cmd::expect_success "oc create -f ${workingdir}/cluster_admin_1.0.yaml"
os::cmd::expect_success 'oc adm policy add-cluster-role-to-user alternate-cluster-admin alternate-cluster-admin-user'

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

# alternate-cluster-admin can restrict himself to less groups (no star)
os::cmd::try_until_text "oc policy who-can update clusterrroles" "alternate-cluster-admin-user"
resourceversion=$(oc get clusterrole/alternate-cluster-admin -o=jsonpath="{.metadata.resourceVersion}")
cp ${OS_ROOT}/test/testdata/bootstrappolicy/cluster_admin_without_apigroups.yaml ${workingdir}
os::util::sed "s/RESOURCE_VERSION/${resourceversion}/g" ${workingdir}/cluster_admin_without_apigroups.yaml
os::cmd::expect_success "oc replace --config=${new_kubeconfig} clusterrole/alternate-cluster-admin -f ${workingdir}/cluster_admin_without_apigroups.yaml"

# alternate-cluster-admin should NOT have the power add back star now (anything other than star is considered less so this mimics testing against no groups)
os::cmd::try_until_failure "oc policy who-can update hpa.autoscaling | grep -q alternate-cluster-admin-user"
resourceversion=$(oc get clusterrole/alternate-cluster-admin -o=jsonpath="{.metadata.resourceVersion}")
cp ${OS_ROOT}/test/testdata/bootstrappolicy/alternate_cluster_admin.yaml ${workingdir}
os::util::sed "s/RESOURCE_VERSION/${resourceversion}/g" ${workingdir}/alternate_cluster_admin.yaml
os::cmd::expect_failure_and_text "oc replace --config=${new_kubeconfig} clusterrole/alternate-cluster-admin -f ${workingdir}/alternate_cluster_admin.yaml" "attempt to grant extra privileges"

# This test validates cluster level policy for serviceaccounts
# ensure service account cannot list pods at the namespace level
os::cmd::expect_success_and_text "oc policy can-i list pods --as=system:serviceaccount:cmd-policy:testserviceaccount" "no"
os::cmd::expect_success_and_text "oc adm policy add-role-to-user view -z=testserviceaccount" "role \"view\" added: \"testserviceaccount\""
# ensure service account can list pods at the namespace level after "view" role is added, but not at the cluster level
os::cmd::try_until_text "oc policy can-i list pods --as=system:serviceaccount:${project}:testserviceaccount" "yes"
os::cmd::try_until_text "oc policy can-i list pods --all-namespaces --as=system:serviceaccount:${project}:testserviceaccount" "no"
# ensure service account can list pods at the cluster level after "cluster-reader" cluster role is added
os::cmd::expect_success_and_text "oc adm policy add-cluster-role-to-user cluster-reader -z=testserviceaccount" "cluster role \"cluster-reader\" added: \"testserviceaccount\""
os::cmd::try_until_text "oc policy can-i list pods --all-namespaces --as=system:serviceaccount:${project}:testserviceaccount" "yes"

echo "policy: ok"
os::test::junit::declare_suite_end
