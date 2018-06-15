#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete project/example project/ui-test-project project/recreated-project
  oc delete sa/router -n default
  oc delete node/fake-node
  oc delete groups/shortoutputgroup
  oc delete groups/group1
  oc delete groups/cascaded-group
  oc delete groups/orphaned-group
  oc delete users/cascaded-user
  oc delete users/orphaned-user
  oc delete identities/anypassword:orphaned-user
  oc delete identities/anypassword:cascaded-user
  oc adm policy reconcile-cluster-roles --confirm --additive-only=false
  oc adm policy reconcile-cluster-role-bindings --confirm --additive-only=false
) &>/dev/null

project="$( oc project -q )"

defaultimage="openshift/origin-\${component}:latest"
USE_IMAGES=${USE_IMAGES:-$defaultimage}

export NODECONFIG="${NODE_CONFIG_DIR}/node-config.yaml"

os::test::junit::declare_suite_start "cmd/admin"
# This test validates admin level commands including system policy

os::test::junit::declare_suite_start "cmd/admin/start"
# Check failure modes of various system commands
os::cmd::expect_failure_and_text 'openshift start network' 'kubeconfig must be set'
os::cmd::expect_failure_and_text 'openshift start network --config=${NODECONFIG} --enable=kubelet' 'the following components are not recognized: kubelet'
os::cmd::expect_failure_and_text 'openshift start network --config=${NODECONFIG} --enable=kubelet,other' 'the following components are not recognized: kubelet, other'
os::cmd::expect_failure_and_text 'openshift start network --config=${NODECONFIG} --disable=other' 'the following components are not recognized: other'
os::cmd::expect_failure_and_text 'openshift start network --config=${NODECONFIG} --disable=dns,proxy,plugins' 'at least one node component must be enabled \(dns, plugins, proxy\)'
os::cmd::expect_failure_and_text 'openshift start node' 'kubeconfig must be set'
os::cmd::expect_failure_and_text 'openshift start node --config=${NODECONFIG} --disable=other' 'the following components are not recognized: other'
os::cmd::expect_failure_and_text 'openshift start node --config=${NODECONFIG} --disable=dns,kubelet,proxy,plugins' 'at least one node component must be enabled \(dns, kubelet, plugins, proxy\)'
os::cmd::expect_failure_and_text 'openshift start --write-config=/tmp/test --hostname=""' 'error: --hostname must have a value'
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/manage-node"
# Test admin manage-node operations
os::cmd::expect_success_and_text 'oc adm manage-node --help' 'Manage nodes'

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
  allocatable:
    cpu: \"4\"
    memory: 8010948Ki
    pods: \"110\"
  capacity:
    cpu: \"4\"
    memory: 8010948Ki
    pods: \"110\"
' | oc create -f -"

os::cmd::expect_success_and_text 'oc adm manage-node --selector= --schedulable=true' 'Ready'
os::cmd::expect_success_and_not_text 'oc adm manage-node --selector= --schedulable=true' 'SchedulingDisabled'
os::cmd::expect_success_and_not_text 'oc get node -o yaml' 'unschedulable: true'
os::cmd::expect_success_and_text 'oc adm manage-node --selector= --schedulable=false' 'SchedulingDisabled'
os::cmd::expect_success_and_text 'oc get node -o yaml' 'unschedulable: true'
# ensure correct serialization of podList output
os::cmd::expect_success_and_text "oc adm manage-node --list-pods --selector= -o jsonpath='{ .kind }'" 'List'
os::cmd::expect_success_and_text "oc adm manage-node --list-pods --selector=" 'NAMESPACE'

echo "manage-node: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/certs"
# check create-master-certs validation
os::cmd::expect_failure_and_text 'oc adm ca create-master-certs --hostnames=example.com --master='                                                'master must be provided'
os::cmd::expect_failure_and_text 'oc adm ca create-master-certs --hostnames=example.com --master=example.com'                                     'master must be a valid URL'
os::cmd::expect_failure_and_text 'oc adm ca create-master-certs --hostnames=example.com --master=https://example.com --public-master=example.com' 'public master must be a valid URL'

# check encrypt/decrypt of plain text
os::cmd::expect_success          "echo -n 'secret data 1' | oc adm ca encrypt --genkey='${ARTIFACT_DIR}/secret.key' --out='${ARTIFACT_DIR}/secret.encrypted'"
os::cmd::expect_success_and_text "oc adm ca decrypt --in='${ARTIFACT_DIR}/secret.encrypted' --key='${ARTIFACT_DIR}/secret.key'" '^secret data 1$'
# create a file with trailing whitespace
echo "data with newline" > "${ARTIFACT_DIR}/secret.whitespace.data"
os::cmd::expect_success_and_text "oc adm ca encrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.whitespace.data'      --out='${ARTIFACT_DIR}/secret.whitespace.encrypted'" 'Warning.*whitespace'
os::cmd::expect_success          "oc adm ca decrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.whitespace.encrypted' --out='${ARTIFACT_DIR}/secret.whitespace.decrypted'"
os::cmd::expect_success          "diff '${ARTIFACT_DIR}/secret.whitespace.data' '${ARTIFACT_DIR}/secret.whitespace.decrypted'"
# create a binary file
echo "hello" | gzip > "${ARTIFACT_DIR}/secret.data"
# encrypt using file and pipe input/output
os::cmd::expect_success "oc adm ca encrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.data' --out='${ARTIFACT_DIR}/secret.file-in-file-out.encrypted'"
os::cmd::expect_success "oc adm ca encrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.data'     > '${ARTIFACT_DIR}/secret.file-in-pipe-out.encrypted'"
os::cmd::expect_success "oc adm ca encrypt --key='${ARTIFACT_DIR}/secret.key'    < '${ARTIFACT_DIR}/secret.data'     > '${ARTIFACT_DIR}/secret.pipe-in-pipe-out.encrypted'"
# decrypt using all three methods
os::cmd::expect_success "oc adm ca decrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.file-in-file-out.encrypted' --out='${ARTIFACT_DIR}/secret.file-in-file-out.decrypted'"
os::cmd::expect_success "oc adm ca decrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.file-in-pipe-out.encrypted'     > '${ARTIFACT_DIR}/secret.file-in-pipe-out.decrypted'"
os::cmd::expect_success "oc adm ca decrypt --key='${ARTIFACT_DIR}/secret.key'    < '${ARTIFACT_DIR}/secret.pipe-in-pipe-out.encrypted'     > '${ARTIFACT_DIR}/secret.pipe-in-pipe-out.decrypted'"
# verify lossless roundtrip
os::cmd::expect_success "diff '${ARTIFACT_DIR}/secret.data' '${ARTIFACT_DIR}/secret.file-in-file-out.decrypted'"
os::cmd::expect_success "diff '${ARTIFACT_DIR}/secret.data' '${ARTIFACT_DIR}/secret.file-in-pipe-out.decrypted'"
os::cmd::expect_success "diff '${ARTIFACT_DIR}/secret.data' '${ARTIFACT_DIR}/secret.pipe-in-pipe-out.decrypted'"
echo "certs: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/groups"
os::cmd::expect_success_and_text 'oc adm groups new shortoutputgroup -o name' 'group/shortoutputgroup'
os::cmd::expect_failure_and_text 'oc adm groups new shortoutputgroup' 'groups.user.openshift.io "shortoutputgroup" already exists'
os::cmd::expect_failure_and_text 'oc adm groups new errorgroup -o blah' 'error: output format "blah" not recognized'
os::cmd::expect_failure_and_text 'oc get groups/errorgroup' 'groups.user.openshift.io "errorgroup" not found'
os::cmd::expect_success_and_text 'oc adm groups new group1 foo bar' 'group1.*foo, bar'
os::cmd::expect_success_and_text 'oc get groups/group1 --no-headers' 'foo, bar'
os::cmd::expect_success 'oc adm groups add-users group1 baz'
os::cmd::expect_success_and_text 'oc get groups/group1 --no-headers' 'baz'
os::cmd::expect_success 'oc adm groups remove-users group1 bar'
os::cmd::expect_success_and_not_text 'oc get groups/group1 --no-headers' 'bar'
os::cmd::expect_success_and_text 'oc adm prune auth users/baz' 'group.user.openshift.io/group1 updated'
echo "groups: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/admin-scc"
os::cmd::expect_success 'oc adm policy who-can get pods'
os::cmd::expect_success 'oc adm policy who-can get pods -n default'
os::cmd::expect_success 'oc adm policy who-can get pods --all-namespaces'
# check to make sure that the resource arg conforms to resource rules
os::cmd::expect_success_and_text 'oc adm policy who-can get Pod' "Resource:  pods"
os::cmd::expect_success_and_text 'oc adm policy who-can get PodASDF' "Resource:  PodASDF"
os::cmd::expect_success_and_text 'oc adm policy who-can get hpa.autoscaling -n default' "Resource:  horizontalpodautoscalers.autoscaling"
os::cmd::expect_success_and_text 'oc adm policy who-can get hpa.v1.autoscaling -n default' "Resource:  horizontalpodautoscalers.autoscaling"
os::cmd::expect_success_and_text 'oc adm policy who-can get hpa -n default' "Resource:  horizontalpodautoscalers.autoscaling"

os::cmd::expect_success 'oc adm policy add-role-to-group --rolebinding-name=cluster-admin cluster-admin system:unauthenticated'
os::cmd::expect_success 'oc adm policy add-role-to-user --rolebinding-name=cluster-admin cluster-admin system:no-user'
os::cmd::expect_success 'oc adm policy add-role-to-user --rolebinding-name=admin admin -z fake-sa'
os::cmd::expect_success_and_text 'oc get rolebinding/admin -o jsonpath={.subjects}' 'fake-sa'
os::cmd::expect_success 'oc adm policy remove-role-from-user admin -z fake-sa'
os::cmd::expect_success_and_not_text 'oc get rolebinding/admin -o jsonpath={.subjects}' 'fake-sa'
os::cmd::expect_success 'oc adm policy add-role-to-user --rolebinding-name=admin admin -z fake-sa'
os::cmd::expect_success_and_text 'oc get rolebinding/admin -o jsonpath={.subjects}' 'fake-sa'
os::cmd::expect_success "oc adm policy remove-role-from-user admin system:serviceaccount:$(oc project -q):fake-sa"
os::cmd::expect_success_and_not_text 'oc get rolebinding/admin -o jsonpath={.subjects}' 'fake-sa'
os::cmd::expect_success 'oc adm policy remove-role-from-group cluster-admin system:unauthenticated'
os::cmd::expect_success 'oc adm policy remove-role-from-user cluster-admin system:no-user'
os::cmd::expect_failure_and_text 'oc adm policy remove-role-from-user admin ghost' 'error: unable to find target \[ghost\]'
os::cmd::expect_failure_and_text 'oc adm policy remove-role-from-user admin -z ghost' 'error: unable to find target \[ghost\]'
os::cmd::expect_success 'oc adm policy remove-group system:unauthenticated'
os::cmd::expect_success 'oc adm policy remove-user system:no-user'
os::cmd::expect_success 'oc adm policy add-cluster-role-to-group cluster-admin system:unauthenticated'
os::cmd::expect_success 'oc adm policy remove-cluster-role-from-group cluster-admin system:unauthenticated'
os::cmd::expect_success 'oc adm policy add-cluster-role-to-user cluster-admin system:no-user'
os::cmd::expect_success 'oc adm policy remove-cluster-role-from-user cluster-admin system:no-user'
os::cmd::expect_success 'oc adm policy add-role-to-user view foo'
os::cmd::expect_success 'oc adm policy add-role-to-user view bar --rolebinding-name=custom'
os::cmd::expect_success 'oc adm policy add-role-to-user view baz --rolebinding-name=custom'
os::cmd::expect_success_and_text 'oc get rolebinding/view -o jsonpath="{.metadata.name},{.roleRef.name},{.userNames[*]}"' '^view,view,foo$'
os::cmd::expect_success_and_text 'oc get rolebinding/custom -o jsonpath="{.metadata.name},{.roleRef.name},{.userNames[*]}"' '^custom,view,bar baz$'
os::cmd::expect_failure_and_text 'oc adm policy add-role-to-user other fuz --rolebinding-name=custom' '^error: rolebinding custom found for role view, not other$'

os::cmd::expect_success 'oc adm policy add-scc-to-user privileged fake-user'
os::cmd::expect_success_and_text 'oc get scc/privileged -o yaml' 'fake-user'
os::cmd::expect_success 'oc adm policy add-scc-to-user privileged -z fake-sa'
os::cmd::expect_success_and_text 'oc get scc/privileged -o yaml' "system:serviceaccount:$(oc project -q):fake-sa"
os::cmd::expect_success 'oc adm policy add-scc-to-group privileged fake-group'
os::cmd::expect_success_and_text 'oc get scc/privileged -o yaml' 'fake-group'
os::cmd::expect_success 'oc adm policy remove-scc-from-user privileged fake-user'
os::cmd::expect_success_and_not_text 'oc get scc/privileged -o yaml' 'fake-user'
os::cmd::expect_success 'oc adm policy remove-scc-from-user privileged -z fake-sa'
os::cmd::expect_success_and_not_text 'oc get scc/privileged -o yaml' "system:serviceaccount:$(oc project -q):fake-sa"
os::cmd::expect_success 'oc adm policy remove-scc-from-group privileged fake-group'
os::cmd::expect_success_and_not_text 'oc get scc/privileged -o yaml' 'fake-group'

# check pruning
os::cmd::expect_success 'oc adm policy add-scc-to-user privileged fake-user'
os::cmd::expect_success_and_text 'oc adm prune auth users/fake-user' 'securitycontextconstraints.security.openshift.io/privileged updated'
os::cmd::expect_success 'oc adm policy add-scc-to-group privileged fake-group'
os::cmd::expect_success_and_text 'oc adm prune auth groups/fake-group' 'securitycontextconstraints.security.openshift.io/privileged updated'
echo "admin-scc: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/reconcile-cluster-roles"
os::cmd::expect_success 'oc delete clusterrole/cluster-status --cascade=false'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oc adm policy reconcile-cluster-roles'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oc adm policy reconcile-cluster-roles --confirm --loglevel=8'
os::cmd::expect_success 'oc get clusterrole/cluster-status'
# check the reconcile again with a specific cluster role name
os::cmd::expect_success 'oc delete clusterrole/cluster-status --cascade=false'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oc adm policy reconcile-cluster-roles cluster-admin --confirm'
os::cmd::expect_failure 'oc get clusterrole/cluster-status'
os::cmd::expect_success 'oc adm policy reconcile-cluster-roles clusterrole/cluster-status --confirm'
os::cmd::expect_success 'oc get clusterrole/cluster-status'

# test reconciliation protection by replacing the basic-user role with one that has missing default permissions, and extra non-default permissions
os::cmd::expect_success 'oc replace --force -f ./test/testdata/basic-user-with-groups-without-projectrequests.yaml'
# 1. mark the role as protected, and ensure the role is skipped by reconciliation
os::cmd::expect_success 'oc annotate clusterrole/basic-user openshift.io/reconcile-protect=true'
os::cmd::expect_success_and_text     'oc adm policy reconcile-cluster-roles basic-user --additive-only=false --confirm' 'skipped: clusterrole/basic-user'
# 2. unmark the role as protected, and ensure reconcile expects to remove extra permissions, and put back removed permissions
os::cmd::expect_success 'oc annotate clusterrole/basic-user openshift.io/reconcile-protect=false --overwrite'
os::cmd::expect_success_and_text     'oc get clusterrole/basic-user -o jsonpath="{.rules[*].resources}"' 'groups'
os::cmd::expect_success_and_not_text 'oc get clusterrole/basic-user -o jsonpath="{.rules[*].resources}"' 'projectrequests'
os::cmd::expect_success_and_not_text 'oc adm policy reconcile-cluster-roles basic-user -o jsonpath="{.items[*].rules[*].resources}" --additive-only=false' 'groups'
os::cmd::expect_success_and_text     'oc adm policy reconcile-cluster-roles basic-user -o jsonpath="{.items[*].rules[*].resources}" --additive-only=false' 'projectrequests'
# reconcile updates the role
os::cmd::expect_success_and_text     'oc adm policy reconcile-cluster-roles basic-user --additive-only=false --confirm' 'clusterrole/basic-user'
# a second reconcile doesn't need to update the role
os::cmd::expect_success_and_not_text 'oc adm policy reconcile-cluster-roles basic-user --additive-only=false --confirm' 'clusterrole/basic-user'

# test label/annotation reconciliation by replacing the basic-user role with one that has custom labels, annotations, and permissions
os::cmd::expect_success 'oc replace --force -f ./test/testdata/basic-user-with-annotations-labels-groups-without-projectrequests.yaml'
# display shows customized labels/annotations
os::cmd::expect_success_and_text 'oc adm policy reconcile-cluster-roles' 'custom-label'
os::cmd::expect_success_and_text 'oc adm policy reconcile-cluster-roles' 'custom-annotation'
os::cmd::expect_success_and_text 'oc adm policy reconcile-cluster-roles --additive-only --confirm' 'clusterrole/basic-user'
# reconcile preserves added rules, labels, and annotations
os::cmd::expect_success_and_text 'oc get clusterroles/basic-user -o json' 'custom-label'
os::cmd::expect_success_and_text 'oc get clusterroles/basic-user -o json' 'custom-annotation'
os::cmd::expect_success_and_text 'oc get clusterroles/basic-user -o json' 'groups'
os::cmd::expect_success 'oc adm policy reconcile-cluster-roles --additive-only=false --confirm'
os::cmd::expect_success_and_not_text 'oc get clusterroles/basic-user -o yaml' 'groups'
echo "admin-reconcile-cluster-roles: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/reconcile-cluster-role-bindings"
# Ensure a removed binding gets re-added
os::cmd::expect_success 'oc delete clusterrolebinding/cluster-status-binding'
os::cmd::expect_failure 'oc get clusterrolebinding/cluster-status-binding'
os::cmd::expect_success 'oc adm policy reconcile-cluster-role-bindings'
os::cmd::expect_failure 'oc get clusterrolebinding/cluster-status-binding'
os::cmd::expect_success 'oc adm policy reconcile-cluster-role-bindings --confirm'
os::cmd::expect_success 'oc get clusterrolebinding/cluster-status-binding'
# Customize a binding
os::cmd::expect_success 'oc replace --force -f ./test/testdata/basic-users-binding.json'
# display shows customized labels/annotations
os::cmd::expect_success_and_text 'oc adm policy reconcile-cluster-role-bindings' 'custom-label'
os::cmd::expect_success_and_text 'oc adm policy reconcile-cluster-role-bindings' 'custom-annotation'
os::cmd::expect_success 'oc adm policy reconcile-cluster-role-bindings --confirm'
# Ensure a customized binding's subjects, labels, annotations are retained by default
os::cmd::expect_success_and_text 'oc get clusterrolebindings/basic-users -o json' 'custom-label'
os::cmd::expect_success_and_text 'oc get clusterrolebindings/basic-users -o json' 'custom-annotation'
os::cmd::expect_success_and_text 'oc get clusterrolebindings/basic-users -o json' 'custom-user'
# Ensure a customized binding's roleref is corrected
os::cmd::expect_success_and_not_text 'oc get clusterrolebindings/basic-users -o json' 'cluster-status'
# Ensure --additive-only=false removes customized users from the binding
os::cmd::expect_success 'oc adm policy reconcile-cluster-role-bindings --additive-only=false --confirm'
os::cmd::expect_success_and_not_text 'oc get clusterrolebindings/basic-users -o json' 'custom-user'
# check the reconcile again with a specific cluster role name
os::cmd::expect_success 'oc delete clusterrolebinding/basic-users'
os::cmd::expect_failure 'oc get clusterrolebinding/basic-users'
os::cmd::expect_success 'oc adm policy reconcile-cluster-role-bindings cluster-admin --confirm'
os::cmd::expect_failure 'oc get clusterrolebinding/basic-users'
os::cmd::expect_success 'oc adm policy reconcile-cluster-role-bindings basic-user --confirm'
os::cmd::expect_success 'oc get clusterrolebinding/basic-users'
echo "admin-reconcile-cluster-role-bindings: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/role-reapers"
os::cmd::expect_success "oc process -f test/extended/testdata/roles/policy-roles.yaml -p NAMESPACE='${project}' | oc create -f -"
os::cmd::expect_success "oc get rolebinding/basic-users"
os::cmd::expect_success "oc delete role/basic-user"
os::cmd::expect_failure "oc get rolebinding/basic-users"
os::cmd::expect_success "oc create -f test/extended/testdata/roles/policy-clusterroles.yaml"
os::cmd::expect_success "oc get clusterrolebinding/basic-users2"
os::cmd::expect_success "oc delete clusterrole/basic-user2"
os::cmd::expect_failure "oc get clusterrolebinding/basic-users2"
os::cmd::expect_success "oc policy add-role-to-user edit foo"
os::cmd::expect_success "oc get rolebinding/edit"
os::cmd::expect_success "oc delete clusterrole/edit"
os::cmd::expect_failure "oc get rolebinding/edit"
os::cmd::expect_success "oc adm policy reconcile-cluster-roles --confirm"
os::cmd::expect_success "oc adm policy reconcile-cluster-role-bindings --confirm"

os::cmd::expect_success "oc process -f test/extended/testdata/roles/policy-roles.yaml -p NAMESPACE='${project}' | oc create -f -"
os::cmd::expect_success "oc get rolebinding/basic-users"
os::cmd::expect_success_and_text "oc adm prune auth role/basic-user" "rolebinding.rbac.authorization.k8s.io/basic-users deleted"
os::cmd::expect_success "oc get role/basic-user"
os::cmd::expect_success "oc delete role/basic-user"

os::cmd::expect_success "oc create -f test/extended/testdata/roles/policy-clusterroles.yaml"
os::cmd::expect_success "oc get clusterrolebinding/basic-users2"
os::cmd::expect_success_and_text "oc adm prune auth clusterrole/basic-user2"  "clusterrolebinding.rbac.authorization.k8s.io/basic-users2 deleted"
os::cmd::expect_success "oc get clusterrole/basic-user2"
os::cmd::expect_success "oc delete clusterrole/basic-user2"
echo "admin-role-reapers: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/role-selectors"
os::cmd::expect_success "oc create -f test/extended/testdata/roles/policy-clusterroles.yaml"
os::cmd::expect_success "oc get clusterrole/basic-user2"
os::cmd::expect_success "oc label clusterrole/basic-user2 foo=bar"
os::cmd::expect_success_and_not_text "oc get clusterroles --selector=foo=bar" "No resources found"
os::cmd::expect_success_and_text "oc get clusterroles --selector=foo=unknown" "No resources found"
os::cmd::expect_success "oc get clusterrolebinding/basic-users2"
os::cmd::expect_success "oc label clusterrolebinding/basic-users2 foo=bar"
os::cmd::expect_success_and_not_text "oc get clusterrolebindings --selector=foo=bar" "No resources found"
os::cmd::expect_success_and_text "oc get clusterroles --selector=foo=unknown" "No resources found"
os::cmd::expect_success "oc delete clusterrole/basic-user2"
os::test::junit::declare_suite_end
echo "admin-role-selectors: ok"

os::test::junit::declare_suite_start "cmd/admin/ui-project-commands"
# Test the commands the UI projects page tells users to run
# These should match what is described in projects.html
os::cmd::expect_success 'oc adm new-project ui-test-project --admin="createuser"'
os::cmd::expect_success 'oc adm policy add-role-to-user --rolebinding-name=admin admin adduser -n ui-test-project'
# Make sure project can be listed by oc (after auth cache syncs)
os::cmd::try_until_text 'oc get projects' 'ui\-test\-project'
# Make sure users got added
os::cmd::expect_success_and_text "oc get rolebinding admin -n ui-test-project -o jsonpath='{.subjects[*].name}'" '^createuser adduser$'
echo "ui-project-commands: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/new-project"
# Test deleting and recreating a project
os::cmd::expect_success 'oc adm new-project recreated-project --admin="createuser1"'
os::cmd::expect_success 'oc delete project recreated-project'
os::cmd::try_until_failure 'oc get project recreated-project'
os::cmd::expect_success 'oc adm new-project recreated-project --admin="createuser2"'
os::cmd::expect_success_and_text "oc get rolebinding admin -n recreated-project -o jsonpath='{.subjects[*].name}'" '^createuser2$'
echo "new-project: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/router"
# Test running a router
os::cmd::expect_failure_and_text 'oc adm router --dry-run' 'does not exist'
os::cmd::expect_success "oc adm policy add-scc-to-user privileged system:serviceaccount:default:router"
os::cmd::expect_success_and_text "oc adm router -o yaml --service-account=router -n default" 'image:.*\-haproxy\-router:'
os::cmd::expect_success "oc adm router --images='${USE_IMAGES}' --service-account=router -n default"
os::cmd::expect_success_and_text 'oc adm router -n default' 'service exists'
os::cmd::expect_success_and_text 'oc get dc/router -o yaml -n default' 'readinessProbe'
echo "router: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/registry"
# Test running a registry as a daemonset
os::cmd::expect_success "oc delete clusterrolebinding/registry-registry-role"
os::cmd::expect_failure_and_text 'oc adm registry --daemonset --dry-run' 'does not exist'
os::cmd::expect_success_and_text "oc adm registry --daemonset -o yaml" 'DaemonSet'
os::cmd::expect_success "oc adm registry --daemonset --images='${USE_IMAGES}'"
os::cmd::expect_success_and_text 'oc adm registry --daemonset' 'service exists'
os::cmd::try_until_text 'oc get ds/docker-registry --template="{{.status.desiredNumberScheduled}}"' '1'
# clean up so we can test non-daemonset
os::cmd::expect_success "oc adm registry --daemonset -o yaml | oc delete -f - -ncmd-admin --cascade=false"
echo "registry daemonset: ok"

# Test running a registry
os::cmd::expect_failure_and_text 'oc adm registry --dry-run' 'does not exist'
os::cmd::expect_success_and_text "oc adm registry -o yaml" 'image:.*\-docker\-registry'
os::cmd::expect_success "oc adm registry --images='${USE_IMAGES}'"
os::cmd::expect_success_and_text 'oc adm registry' 'service exists'
os::cmd::expect_success_and_text 'oc describe svc/docker-registry' 'Session Affinity:\s*ClientIP'
os::cmd::expect_success_and_text 'oc get dc/docker-registry -o yaml' 'readinessProbe'
os::cmd::expect_success_and_text 'oc env --list dc/docker-registry' 'REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ENFORCEQUOTA=false'
echo "registry: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/apply"
workingdir=$(mktemp -d)
os::cmd::expect_success "oc adm registry -o yaml > ${workingdir}/oadm_registry.yaml"
os::util::sed "s/5000/6000/g" ${workingdir}/oadm_registry.yaml
os::cmd::expect_success "oc apply -f ${workingdir}/oadm_registry.yaml"
os::cmd::expect_success_and_text 'oc get dc/docker-registry -o yaml' '6000'
echo "apply: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/build-chain"
# Test building a dependency tree
os::cmd::expect_success 'oc process -f examples/sample-app/application-template-stibuild.json -l build=sti | oc create -f -'
# Test both the type/name resource syntax and the fact that istag/origin-ruby-sample:latest is still
# not created but due to a buildConfig pointing to it, we get back its graph of deps.
os::cmd::expect_success_and_text 'oc adm build-chain istag/origin-ruby-sample' 'istag/origin-ruby-sample:latest'
os::cmd::expect_success_and_text 'oc adm build-chain ruby-22-centos7 -o dot' 'digraph "ruby-22-centos7:latest"'
os::cmd::expect_success 'oc delete all -l build=sti'
echo "ex build-chain: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/complex-scenarios"
# Make sure no one commits data with allocated values that could flake
os::cmd::expect_failure 'grep -r "clusterIP.*172" test/testdata/app-scenarios'
os::cmd::expect_success 'oc adm new-project example --admin="createuser"'
os::cmd::expect_success 'oc project example'
os::cmd::try_until_success 'oc get serviceaccount default'
os::cmd::expect_success 'oc create -f test/testdata/app-scenarios'
os::cmd::expect_success 'oc status'
os::cmd::expect_success_and_text 'oc status -o dot' '"example"'
echo "complex-scenarios: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/reconcile-security-context-constraints"
# Test reconciling SCCs
os::cmd::expect_success 'oc delete scc/restricted'
os::cmd::expect_failure 'oc get scc/restricted'
os::cmd::expect_success 'oc adm policy reconcile-sccs'
os::cmd::expect_failure 'oc get scc/restricted'
os::cmd::expect_success 'oc adm policy reconcile-sccs --confirm'
os::cmd::expect_success 'oc get scc/restricted'

os::cmd::expect_success 'oc adm policy add-scc-to-user restricted my-restricted-user'
os::cmd::expect_success_and_text 'oc get scc/restricted -o yaml' 'my-restricted-user'
os::cmd::expect_success 'oc adm policy reconcile-sccs --confirm'
os::cmd::expect_success_and_text 'oc get scc/restricted -o yaml' 'my-restricted-user'

os::cmd::expect_success 'oc adm policy remove-scc-from-group restricted system:authenticated'
os::cmd::expect_success_and_not_text 'oc get scc/restricted -o yaml' 'system:authenticated'
os::cmd::expect_success 'oc adm policy reconcile-sccs --confirm'
os::cmd::expect_success_and_text 'oc get scc/restricted -o yaml' 'system:authenticated'

os::cmd::expect_success 'oc label scc/restricted foo=bar'
os::cmd::expect_success_and_text 'oc get scc/restricted -o yaml' 'foo: bar'
os::cmd::expect_success 'oc adm policy reconcile-sccs --confirm --additive-only=true'
os::cmd::expect_success_and_text 'oc get scc/restricted -o yaml' 'foo: bar'
os::cmd::expect_success 'oc adm policy reconcile-sccs --confirm --additive-only=false'
os::cmd::expect_success_and_not_text 'oc get scc/restricted -o yaml' 'foo: bar'

os::cmd::expect_success 'oc annotate scc/restricted topic="my-foo-bar"'
os::cmd::expect_success_and_text 'oc get scc/restricted -o yaml' 'topic: my-foo-bar'
os::cmd::expect_success 'oc adm policy reconcile-sccs --confirm --additive-only=true'
os::cmd::expect_success_and_text 'oc get scc/restricted -o yaml' 'topic: my-foo-bar'
os::cmd::expect_success 'oc adm policy reconcile-sccs --confirm --additive-only=false'
os::cmd::expect_success_and_not_text 'oc get scc/restricted -o yaml' 'topic: my-foo-bar'
echo "reconcile-scc: ok"
os::test::junit::declare_suite_end

# cleanup the fake node that has been created so that it doesn't confuse other test-cmd scripts
os::cmd::expect_success "oc delete node/fake-node"

os::test::junit::declare_suite_start "cmd/admin/rolebinding-allowed"
# Admin can bind local roles without cluster-admin permissions
os::cmd::expect_success "oc create -f test/extended/testdata/roles/empty-role.yaml -n '${project}'"
os::cmd::expect_success 'oc adm policy add-role-to-user admin local-admin  -n '${project}''
os::cmd::expect_success 'oc login -u local-admin -p pw'
os::cmd::expect_success 'oc policy add-role-to-user empty-role other --role-namespace='${project}' -n '${project}''
os::cmd::expect_success 'oc login -u system:admin'
os::cmd::expect_success "oc delete role/empty-role -n '${project}'"
echo "cmd/admin/rolebinding-allowed: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/rolebinding-local-only"
# Admin cannot bind local roles from different namespace
otherproject='someotherproject'
os::cmd::expect_success "oc new-project '${otherproject}'"
os::cmd::expect_success "oc create -f test/extended/testdata/roles/empty-role.yaml -n '${project}'"
os::cmd::expect_success 'oc adm policy add-role-to-user admin local-admin  -n '${otherproject}''
os::cmd::expect_success 'oc login -u local-admin -p pw'
os::cmd::expect_failure_and_text 'oc policy add-role-to-user empty-role other --role-namespace='${project}' -n '${otherproject}'' "invalid origin role binding empty-role: attempts to reference role in namespace \"${project}\" instead of current namespace \"${otherproject}\""
os::cmd::expect_success 'oc login -u system:admin'
os::cmd::expect_success "oc delete role/empty-role -n '${project}'"
echo "rolebinding-local-only: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/user-group-cascade"
# Create test users/identities and groups
os::cmd::expect_success 'oc login -u cascaded-user -p pw'
os::cmd::expect_success 'oc login -u orphaned-user -p pw'
os::cmd::expect_success 'oc login -u system:admin'
os::cmd::expect_success_and_text 'oc adm groups new cascaded-group cascaded-user orphaned-user' 'cascaded-group.*cascaded-user, orphaned-user'
os::cmd::expect_success_and_text 'oc adm groups new orphaned-group cascaded-user orphaned-user' 'orphaned-group.*cascaded-user, orphaned-user'
# Add roles, sccs to users/groups
os::cmd::expect_success 'oc adm policy add-scc-to-user           restricted    cascaded-user  orphaned-user'
os::cmd::expect_success 'oc adm policy add-scc-to-group          restricted    cascaded-group orphaned-group'
os::cmd::expect_success 'oc adm policy add-role-to-user --rolebinding-name=cluster-admin cluster-admin cascaded-user  orphaned-user  -n default'
os::cmd::expect_success 'oc adm policy add-role-to-group --rolebinding-name=cluster-admin cluster-admin cascaded-group orphaned-group -n default'
os::cmd::expect_success 'oc adm policy add-cluster-role-to-user --rolebinding-name=cluster-admin cluster-admin cascaded-user  orphaned-user'
os::cmd::expect_success 'oc adm policy add-cluster-role-to-group --rolebinding-name=cluster-admin cluster-admin cascaded-group orphaned-group'

# Delete users
os::cmd::expect_success 'oc delete user  cascaded-user'
os::cmd::expect_success 'oc delete user  orphaned-user  --cascade=false'
# Verify all identities remain
os::cmd::expect_success 'oc get identities/anypassword:cascaded-user'
os::cmd::expect_success 'oc get identities/anypassword:orphaned-user'
# Verify orphaned user references are left
os::cmd::expect_success_and_text     "oc get clusterrolebindings/cluster-admins clusterrolebindings/cluster-admin --output-version=v1 -o jsonpath='{ .items[*].subjects }'" 'orphaned-user'
os::cmd::expect_success_and_text     "oc get rolebindings/cluster-admin         --output-version=v1 --template='{{.subjects}}' -n default" 'orphaned-user'
os::cmd::expect_success_and_text     "oc get scc/restricted                     --output-version=v1 --template='{{.users}}'"               'orphaned-user'
os::cmd::expect_success_and_text     "oc get group/cascaded-group               --output-version=v1 --template='{{.users}}'"               'orphaned-user'
# Verify cascaded user references are removed
os::cmd::expect_success_and_not_text "oc get clusterrolebindings/cluster-admins clusterrolebindings/cluster-admin --output-version=v1 -o jsonpath='{ .items[*].subjects }'" 'cascaded-user'
os::cmd::expect_success_and_not_text "oc get rolebindings/cluster-admin         --output-version=v1 --template='{{.subjects}}' -n default" 'cascaded-user'
os::cmd::expect_success_and_not_text "oc get scc/restricted                     --output-version=v1 --template='{{.users}}'"               'cascaded-user'
os::cmd::expect_success_and_not_text "oc get group/cascaded-group               --output-version=v1 --template='{{.users}}'"               'cascaded-user'

# Delete groups
os::cmd::expect_success 'oc delete group cascaded-group'
os::cmd::expect_success 'oc delete group orphaned-group --cascade=false'
# Verify orphaned group references are left
os::cmd::expect_success_and_text     "oc get clusterrolebindings/cluster-admins clusterrolebindings/cluster-admin --output-version=v1 -o jsonpath='{ .items[*].subjects }'" 'orphaned-group'
os::cmd::expect_success_and_text     "oc get rolebindings/cluster-admin         --output-version=v1 --template='{{.subjects}}' -n default" 'orphaned-group'
os::cmd::expect_success_and_text     "oc get scc/restricted                     --output-version=v1 --template='{{.groups}}'"              'orphaned-group'
# Verify cascaded group references are removed
os::cmd::expect_success_and_not_text "oc get clusterrolebindings/cluster-admins clusterrolebindings/cluster-admin --output-version=v1 -o jsonpath='{ .items[*].subjects }'" 'cascaded-group'
os::cmd::expect_success_and_not_text "oc get rolebindings/cluster-admin         --output-version=v1 --template='{{.subjects}}' -n default" 'cascaded-group'
os::cmd::expect_success_and_not_text "oc get scc/restricted                     --output-version=v1 --template='{{.groups}}'"              'cascaded-group'
echo "user-group-cascade: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/admin/serviceaccounts"
# create a new service account
os::cmd::expect_success_and_text 'oc create serviceaccount my-sa-name' 'serviceaccount "my-sa-name" created'
os::cmd::expect_success 'oc get sa my-sa-name'

# extract token and ensure it links us back to the service account
os::cmd::try_until_success 'oc sa get-token my-sa-name'
os::cmd::expect_success_and_text 'oc get user/~ --token="$( oc sa get-token my-sa-name )"' 'system:serviceaccount:.+:my-sa-name'

# add a new token and ensure it links us back to the service account
os::cmd::expect_success_and_text 'oc get user/~ --token="$( oc sa new-token my-sa-name )"' 'system:serviceaccount:.+:my-sa-name'

# add a new labeled token and ensure the label stuck
os::cmd::expect_success 'oc sa new-token my-sa-name --labels="mykey=myvalue,myotherkey=myothervalue"'
os::cmd::expect_success_and_text 'oc get secrets --selector="mykey=myvalue"' 'my-sa-name'
os::cmd::expect_success_and_text 'oc get secrets --selector="myotherkey=myothervalue"' 'my-sa-name'
os::cmd::expect_success_and_text 'oc get secrets --selector="mykey=myvalue,myotherkey=myothervalue"' 'my-sa-name'
echo "serviceacounts: ok"
os::test::junit::declare_suite_end

# user creation
os::test::junit::declare_suite_start "cmd/admin/user-creation"
os::cmd::expect_success 'oc create user                test-cmd-user'
os::cmd::expect_success 'oc create identity            test-idp:test-uid'
os::cmd::expect_success 'oc create useridentitymapping test-idp:test-uid test-cmd-user'
os::cmd::expect_success_and_text 'oc describe identity test-idp:test-uid' 'test-cmd-user'
os::cmd::expect_success_and_text 'oc describe user     test-cmd-user' 'test-idp:test-uid'
os::test::junit::declare_suite_end

# images
os::test::junit::declare_suite_start "cmd/admin/images"

# import image and check its information
os::cmd::expect_success "oc create -f ${OS_ROOT}/test/testdata/stable-busybox.yaml"
os::cmd::expect_success_and_text "oc adm top images" "sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6\W+default/busybox \(latest\)\W+<none>\W+<none>\W+yes\W+653\.4KiB"
os::cmd::expect_success_and_text "oc adm top imagestreams" "default/busybox\W+653\.4KiB\W+1\W+1"
os::cmd::expect_success "oc delete is/busybox -n default"

# log in as an image-pruner and test that oc adm prune images works against the atomic binary
os::cmd::expect_success "oc adm policy add-cluster-role-to-user system:image-pruner pruner --config='${MASTER_CONFIG_DIR}/admin.kubeconfig'"
os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u pruner -p anything"
os::cmd::expect_success_and_text "oc adm prune images" "Dry run enabled - no modifications will be made. Add --confirm to remove images"

echo "images: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
