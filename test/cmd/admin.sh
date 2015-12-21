#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

# Cleanup cluster resources created by this test
(
  set +e
  oc delete project/example project/ui-test-project project/recreated-project
  oc delete sa/router -n default
  oc delete node/fake-node
  oc delete groups/group1
  oc delete groups/cascaded-group
  oc delete groups/orphaned-group
  oc delete users/cascaded-user
  oc delete users/orphaned-user
  oc delete identities/anypassword:orphaned-user
  oc delete identities/anypassword:cascaded-user
  oadm policy reconcile-cluster-roles --confirm
  oadm policy reconcile-cluster-role-bindings --confirm
) 2>/dev/null 1>&2

defaultimage="openshift/origin-\${component}:latest"
USE_IMAGES=${USE_IMAGES:-$defaultimage}

# This test validates admin level commands including system policy

# Test admin manage-node operations
os::cmd::expect_success_and_text 'openshift admin manage-node --help' 'Manage nodes'

# create a node object to mess with
os::cmd::expect_success "echo 'apiVersion: v1
kind: Node
metadata:
  labels:
      kubernetes.io/hostname: fake-node
  name: fake-node
spec:
  externalID: fake-node
status:
  conditions:
  - lastHeartbeatTime: 2015-09-08T16:58:02Z
    lastTransitionTime: 2015-09-04T11:49:06Z
    reason: kubelet is posting ready status
    status: \"True\"
    type: Ready
' | oc create -f -"

os::cmd::expect_success_and_text 'oadm manage-node --selector= --schedulable=true' 'Ready'
os::cmd::expect_success_and_not_text 'oadm manage-node --selector= --schedulable=true' 'Sched'

os::cmd::expect_success 'oc create -f examples/hello-openshift/hello-pod.json'
# os::cmd::expect_success_and_text 'oadm manage-node --list-pods' 'hello-openshift'
# os::cmd::expect_success_and_text 'oadm manage-node --list-pods' '(unassigned|assigned)'
# os::cmd::expect_success_and_text 'oadm manage-node --evacuate --dry-run' 'hello-openshift'
# os::cmd::expect_success_and_text 'oadm manage-node --schedulable=false' 'SchedulingDisabled'
# os::cmd::expect_failure_and_text 'oadm manage-node --evacuate' 'Unable to evacuate'
# os::cmd::expect_success_and_text 'oadm manage-node --evacuate --force' 'hello-openshift'
# os::cmd::expect_success_and_text 'oadm manage-node --list-pods' 'hello-openshift'
os::cmd::expect_success 'oc delete pods hello-openshift'
echo "manage-node: ok"

os::cmd::expect_success 'oadm groups new group1 foo bar'
os::cmd::expect_success_and_text 'oc get groups/group1 --no-headers' 'foo, bar'
os::cmd::expect_success 'oadm groups add-users group1 baz'
os::cmd::expect_success_and_text 'oc get groups/group1 --no-headers' 'baz'
os::cmd::expect_success 'oadm groups remove-users group1 bar'
os::cmd::expect_success_and_not_text 'oc get groups/group1 --no-headers' 'bar'
echo "groups: ok"

os::cmd::expect_success 'oadm policy who-can get pods'
os::cmd::expect_success 'oadm policy who-can get pods -n default'
os::cmd::expect_success 'oadm policy who-can get pods --all-namespaces'

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

os::cmd::expect_success 'oc delete clusterrole/cluster-status --cascade=false'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles --confirm'
os::cmd::expect_success 'oc get clusterrole/cluster-status'
# check the reconcile again with a specific cluster role name
os::cmd::expect_success 'oc delete clusterrole/cluster-status --cascade=false'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles cluster-admin --confirm'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles clusterrole/cluster-status --confirm'
os::cmd::expect_success 'oc get clusterrole/cluster-status'

os::cmd::expect_success 'oc replace --force -f ./test/fixtures/basic-user.json'
# display shows customized labels/annotations
os::cmd::expect_success_and_text 'oadm policy reconcile-cluster-roles' 'custom-label'
os::cmd::expect_success_and_text 'oadm policy reconcile-cluster-roles' 'custom-annotation'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles --additive-only --confirm'
# reconcile preserves added rules, labels, and annotations
os::cmd::expect_success_and_text 'oc get clusterroles/basic-user -o json' 'custom-label'
os::cmd::expect_success_and_text 'oc get clusterroles/basic-user -o json' 'custom-annotation'
os::cmd::expect_success_and_text 'oc get clusterroles/basic-user -o json' 'groups'
os::cmd::expect_success 'oadm policy reconcile-cluster-roles --confirm'
os::cmd::expect_success_and_not_text 'oc get clusterroles/basic-user -o yaml' 'groups'
echo "admin-reconcile-cluster-roles: ok"

# Ensure a removed binding gets re-added
os::cmd::expect_success 'oc delete clusterrolebinding/cluster-status-binding'
os::cmd::expect_failure 'oc get clusterrolebinding/cluster-status-binding'
os::cmd::expect_success 'oadm policy reconcile-cluster-role-bindings'
os::cmd::expect_failure 'oc get clusterrolebinding/cluster-status-binding'
os::cmd::expect_success 'oadm policy reconcile-cluster-role-bindings --confirm'
os::cmd::expect_success 'oc get clusterrolebinding/cluster-status-binding'
# Customize a binding
os::cmd::expect_success 'oc replace --force -f ./test/fixtures/basic-users-binding.json'
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
echo "admin-reconcile-cluster-role-bindings: ok"

os::cmd::expect_success "oc create -f test/extended/fixtures/roles/policy-roles.yaml"
os::cmd::expect_success "oc get rolebinding/basic-users"
os::cmd::expect_success "oc delete role/basic-user"
os::cmd::expect_failure "oc get rolebinding/basic-users"
os::cmd::expect_success "oc create -f test/extended/fixtures/roles/policy-clusterroles.yaml"
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

echo "admin-policy: ok"

# Test the commands the UI projects page tells users to run
# These should match what is described in projects.html
os::cmd::expect_success 'oadm new-project ui-test-project --admin="createuser"'
os::cmd::expect_success 'oadm policy add-role-to-user admin adduser -n ui-test-project'
# Make sure project can be listed by oc (after auth cache syncs)
os::cmd::try_until_text 'oc get projects' 'ui\-test\-project'
# Make sure users got added
os::cmd::expect_success_and_text "oc describe policybinding ':default' -n ui-test-project" 'createuser'
os::cmd::expect_success_and_text "oc describe policybinding ':default' -n ui-test-project" 'adduser'
echo "ui-project-commands: ok"


# Test deleting and recreating a project
os::cmd::expect_success 'oadm new-project recreated-project --admin="createuser1"'
os::cmd::expect_success 'oc delete project recreated-project'
os::cmd::try_until_failure 'oc get project recreated-project'
os::cmd::expect_success 'oadm new-project recreated-project --admin="createuser2"'
os::cmd::expect_success_and_text "oc describe policybinding ':default' -n recreated-project" 'createuser2'
echo "new-project: ok"

# Test running a router
os::cmd::expect_failure_and_text 'oadm router --dry-run' 'does not exist'
encoded_json='{"kind":"ServiceAccount","apiVersion":"v1","metadata":{"name":"router"}}'
os::cmd::expect_success "echo '${encoded_json}' | oc create -f - -n default"
os::cmd::expect_success "oadm policy add-scc-to-user privileged system:serviceaccount:default:router"
os::cmd::expect_success_and_text "oadm router -o yaml --credentials=${KUBECONFIG} --service-account=router -n default" 'image:.*-haproxy-router:'
os::cmd::expect_success "oadm router --credentials=${KUBECONFIG} --images='${USE_IMAGES}' --service-account=router -n default"
os::cmd::expect_success_and_text 'oadm router -n default' 'service exists'
os::cmd::expect_success_and_text 'oc get dc/router -o yaml -n default' 'readinessProbe'
echo "router: ok"

# Test running a registry
os::cmd::expect_failure_and_text 'oadm registry --dry-run' 'does not exist'
os::cmd::expect_success_and_text "oadm registry -o yaml --credentials=${KUBECONFIG}" 'image:.*-docker-registry'
os::cmd::expect_success "oadm registry --credentials=${KUBECONFIG} --images='${USE_IMAGES}'"
os::cmd::expect_success_and_text 'oadm registry' 'service exists'
os::cmd::expect_success_and_text 'oc describe svc/docker-registry' 'Session Affinity:\s*ClientIP'
echo "registry: ok"

# Test building a dependency tree
os::cmd::expect_success 'oc process -f examples/sample-app/application-template-stibuild.json -l build=sti | oc create -f -'
# Test both the type/name resource syntax and the fact that istag/origin-ruby-sample:latest is still
# not created but due to a buildConfig pointing to it, we get back its graph of deps.
os::cmd::expect_success_and_text 'oadm build-chain istag/origin-ruby-sample' 'imagestreamtag/origin-ruby-sample:latest'
os::cmd::expect_success_and_text 'oadm build-chain ruby-22-centos7 -o dot' 'graph'
os::cmd::expect_success 'oc delete all -l build=sti'
echo "ex build-chain: ok"

os::cmd::expect_success 'oadm new-project example --admin="createuser"'
os::cmd::expect_success 'oc project example'
os::cmd::try_until_success 'oc get serviceaccount default'
os::cmd::expect_success 'oc create -f test/fixtures/app-scenarios'
os::cmd::expect_success 'oc status'
os::cmd::expect_success 'oc status -o dot'
echo "complex-scenarios: ok"

# Test reconciling SCCs
os::cmd::expect_success 'oc delete scc/restricted'
os::cmd::expect_failure 'oc get scc/restricted'
os::cmd::expect_success 'oadm policy reconcile-sccs'
os::cmd::expect_failure 'oc get scc/restricted'
os::cmd::expect_success 'oadm policy reconcile-sccs --confirm'
os::cmd::expect_success 'oc get scc/restricted'

os::cmd::expect_success 'oadm policy add-scc-to-user restricted my-restricted-user'
os::cmd::expect_success_and_text 'oc get scc/restricted -o yaml' 'my-restricted-user'
os::cmd::expect_success 'oadm policy reconcile-sccs --confirm'
os::cmd::expect_success_and_text 'oc get scc/restricted -o yaml' 'my-restricted-user'

os::cmd::expect_success 'oadm policy remove-scc-from-group restricted system:authenticated'
os::cmd::expect_success_and_not_text 'oc get scc/restricted -o yaml' 'system:authenticated'
os::cmd::expect_success 'oadm policy reconcile-sccs --confirm'
os::cmd::expect_success_and_text 'oc get scc/restricted -o yaml' 'system:authenticated'
echo "reconcile-scc: ok"


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
