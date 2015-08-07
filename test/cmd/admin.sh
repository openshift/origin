#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

function tryuntil {
  timeout=$(($(date +%s) + 60))
  until eval "${@}" || [[ $(date +%s) -gt $timeout ]]; do :; done
}

# Cleanup cluster resources created by this test
(
  set +e
  oc delete project/example project/ui-test-project project/recreated-project
  oc delete sa/router -n default
  oadm policy reconcile-cluster-roles
) 2>/dev/null 1>&2

defaultimage="openshift/origin-\${component}:latest"
USE_IMAGES=${USE_IMAGES:-$defaultimage}

# This test validates admin level commands including system policy

# Test admin manage-node operations
[ "$(openshift admin manage-node --help 2>&1 | grep 'Manage nodes')" ]
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
oc delete clusterrole/cluster-status
[ ! "$(oc get clusterrole/cluster-status)" ]
oadm policy reconcile-cluster-roles
[ ! "$(oc get clusterrole/cluster-status)" ]
oadm policy reconcile-cluster-roles --confirm
oc get clusterrole/cluster-status
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
[ "$(oadm router -o yaml --credentials="${KUBECONFIG}" --service-account=router -n default | grep 'openshift/origin-haproxy-')" ]
oadm router --credentials="${KUBECONFIG}" --images="${USE_IMAGES}" --service-account=router -n default
[ "$(oadm router -n default | grep 'service exists')" ]
echo "router: ok"

# Test running a registry
[ ! "$(oadm registry --dry-run | grep 'does not exist')"]
[ "$(oadm registry -o yaml --credentials="${KUBECONFIG}" | grep 'openshift/origin-docker-registry')" ]
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