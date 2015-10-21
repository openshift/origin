#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# Cleanup cluster resources created by this test
(
  set +e
  oc delete project/example project/ui-test-project project/recreated-project
  oc delete sa/router -n default
  oadm policy reconcile-cluster-roles --confirm
  oadm policy reconcile-cluster-role-bindings --confirm
) 2>/dev/null 1>&2

defaultimage="openshift/origin-\${component}:latest"
USE_IMAGES=${USE_IMAGES:-$defaultimage}

# This test validates admin level commands including system policy

# Test admin manage-node operations
[ "$(openshift admin manage-node --help 2>&1 | grep 'Manage nodes')" ]

# create a node object to mess with
echo 'apiVersion: v1
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
    status: "True"
    type: Ready
' | oc create -f -

[ "$(oadm manage-node --selector='' --schedulable=true | grep --text 'Ready' | grep -v 'Sched')" ]
oc create -f examples/hello-openshift/hello-pod.json
#[ "$(oadm manage-node --list-pods | grep 'hello-openshift' | grep -E '(unassigned|assigned)')" ]
#[ "$(oadm manage-node --evacuate --dry-run | grep 'hello-openshift')" ]
#[ "$(oadm manage-node --schedulable=false | grep 'SchedulingDisabled')" ]
#[ "$(oadm manage-node --evacuate 2>&1 | grep 'Unable to evacuate')" ]
#[ "$(oadm manage-node --evacuate --force | grep 'hello-openshift')" ]
#[ ! "$(oadm manage-node --list-pods | grep 'hello-openshift')" ]
oc delete pods hello-openshift
echo "manage-node: ok"

oadm groups new group1 foo bar
oc get groups/group1 --no-headers | grep -q "foo, bar"
oadm groups add-users group1 baz
oc get groups/group1 --no-headers | grep -q "baz"
oadm groups remove-users group1 bar
[ ! "$(oc get groups/group1 --no-headers | grep -q "bar")" ]
echo "groups: ok"

oadm policy who-can get pods
oadm policy who-can get pods -n default
oadm policy who-can get pods --all-namespaces

oadm policy add-role-to-group cluster-admin system:unauthenticated
oadm policy add-role-to-user cluster-admin system:no-user
oadm policy remove-role-from-group cluster-admin system:unauthenticated
oadm policy remove-role-from-user cluster-admin system:no-user
oadm policy remove-group system:unauthenticated
oadm policy remove-user system:no-user
oadm policy add-cluster-role-to-group cluster-admin system:unauthenticated
oadm policy remove-cluster-role-from-group cluster-admin system:unauthenticated
oadm policy add-cluster-role-to-user cluster-admin system:no-user
oadm policy remove-cluster-role-from-user cluster-admin system:no-user

oadm policy add-scc-to-user privileged fake-user
oc get scc/privileged -o yaml | grep fake-user
oadm policy add-scc-to-user privileged -z fake-sa
oc get scc/privileged -o yaml | grep "system:serviceaccount:cmd-admin:fake-sa"
oadm policy add-scc-to-group privileged fake-group
oc get scc/privileged -o yaml | grep fake-group
oadm policy remove-scc-from-user privileged fake-user
[ ! "$(oc get scc/privileged -o yaml | grep fake-user)" ]
oadm policy remove-scc-from-user privileged -z fake-sa
[ ! "$(oc get scc/privileged -o yaml | grep 'system:serviceaccount:cmd-admin:fake-sa')" ]
oadm policy remove-scc-from-group privileged fake-group
[ ! "$(oadm policy add-scc-to-group privileged fake-group)" ]

oc delete clusterrole/cluster-status
[ ! "$(oc get clusterrole/cluster-status)" ]
oadm policy reconcile-cluster-roles
[ ! "$(oc get clusterrole/cluster-status)" ]
oadm policy reconcile-cluster-roles --confirm
oc get clusterrole/cluster-status
oc replace --force -f ./test/fixtures/basic-user.json
# display shows customized labels/annotations
[ "$(oadm policy reconcile-cluster-roles | grep custom-label)" ]
[ "$(oadm policy reconcile-cluster-roles | grep custom-annotation)" ]
oadm policy reconcile-cluster-roles --additive-only --confirm
# reconcile preserves added rules, labels, and annotations
[ "$(oc get clusterroles/basic-user -o json | grep custom-label)" ]
[ "$(oc get clusterroles/basic-user -o json | grep custom-annotation)" ]
[ "$(oc get clusterroles/basic-user -o json | grep groups)" ]
oadm policy reconcile-cluster-roles --confirm
[ ! "$(oc get clusterroles/basic-user -o yaml | grep groups)" ]

# Ensure a removed binding gets re-added
oc delete clusterrolebinding/cluster-status-binding
[ ! "$(oc get clusterrolebinding/cluster-status-binding)" ]
oadm policy reconcile-cluster-role-bindings
[ ! "$(oc get clusterrolebinding/cluster-status-binding)" ]
oadm policy reconcile-cluster-role-bindings --confirm
oc get clusterrolebinding/cluster-status-binding
# Customize a binding
oc replace --force -f ./test/fixtures/basic-users-binding.json
# display shows customized labels/annotations
[ "$(oadm policy reconcile-cluster-role-bindings | grep custom-label)" ]
[ "$(oadm policy reconcile-cluster-role-bindings | grep custom-annotation)" ]
oadm policy reconcile-cluster-role-bindings --confirm
# Ensure a customized binding's subjects, labels, annotations are retained by default
[ "$(oc get clusterrolebindings/basic-users -o json | grep custom-label)" ]
[ "$(oc get clusterrolebindings/basic-users -o json | grep custom-annotation)" ]
[ "$(oc get clusterrolebindings/basic-users -o json | grep custom-user)" ]
# Ensure a customized binding's roleref is corrected
[ ! "$(oc get clusterrolebindings/basic-users -o json | grep cluster-status)" ]
# Ensure --additive-only=false removes customized users from the binding
oadm policy reconcile-cluster-role-bindings --additive-only=false --confirm
[ ! "$(oc get clusterrolebindings/basic-users -o json | grep custom-user)" ]

echo "admin-policy: ok"

# Test the commands the UI projects page tells users to run
# These should match what is described in projects.html
oadm new-project ui-test-project --admin="createuser"
oadm policy add-role-to-user admin adduser -n ui-test-project
# Make sure project can be listed by oc (after auth cache syncs)
tryuntil '[ "$(oc get projects | grep "ui-test-project")" ]'
# Make sure users got added
[ "$(oc describe policybinding ':default' -n ui-test-project | grep createuser)" ]
[ "$(oc describe policybinding ':default' -n ui-test-project | grep adduser)" ]
echo "ui-project-commands: ok"


# Test deleting and recreating a project
oadm new-project recreated-project --admin="createuser1"
oc delete project recreated-project
tryuntil '! oc get project recreated-project'
oadm new-project recreated-project --admin="createuser2"
oc describe policybinding ':default' -n recreated-project | grep createuser2
echo "new-project: ok"

# Test running a router
[ ! "$(oadm router --dry-run | grep 'does not exist')" ]
echo '{"kind":"ServiceAccount","apiVersion":"v1","metadata":{"name":"router"}}' | oc create -f - -n default
oc get scc privileged -o yaml | sed '/users:/ a\
- system:serviceaccount:default:router\
' | oc replace scc privileged -f -
[ "$(oadm router -o yaml --credentials="${KUBECONFIG}" --service-account=router -n default | egrep 'image:.*-haproxy-router:')" ]
oadm router --credentials="${KUBECONFIG}" --images="${USE_IMAGES}" --service-account=router -n default
[ "$(oadm router -n default | grep 'service exists')" ]
echo "router: ok"

# Test running a registry
[ ! "$(oadm registry --dry-run | grep 'does not exist')"]
[ "$(oadm registry -o yaml --credentials="${KUBECONFIG}" | egrep 'image:.*-docker-registry')" ]
oadm registry --credentials="${KUBECONFIG}" --images="${USE_IMAGES}"
[ "$(oadm registry | grep 'service exists')" ]
echo "registry: ok"

# Test building a dependency tree
oc process -f examples/sample-app/application-template-stibuild.json -l build=sti | oc create -f -
[ "$(oadm build-chain ruby-20-centos7 -o dot | grep 'graph')" ]
oc delete all -l build=sti
echo "ex build-chain: ok"

oadm new-project example --admin="createuser"
oc project example
tryuntil oc get serviceaccount default
oc create -f test/fixtures/app-scenarios
oc status
oc status -o dot
echo "complex-scenarios: ok"
