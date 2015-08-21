#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# This test validates user level policy

oc policy add-role-to-group cluster-admin system:unauthenticated
oc policy add-role-to-user cluster-admin system:no-user
oc get rolebinding/cluster-admin --no-headers | grep -q "system:no-user"

oc policy add-role-to-user cluster-admin -z=one,two --serviceaccount=three,four
oc get rolebinding/cluster-admin --no-headers | grep -q "system:serviceaccount:cmd-policy:one"
oc get rolebinding/cluster-admin --no-headers | grep -q "system:serviceaccount:cmd-policy:four"

oc policy remove-role-from-group cluster-admin system:unauthenticated

oc policy remove-role-from-user cluster-admin system:no-user
oc policy remove-role-from-user cluster-admin -z=one,two --serviceaccount=three,four
[ ! "$(oc get rolebinding/cluster-admin --no-headers | grep -q "system:serviceaccount:cmd-policy:four")" ]

oc policy remove-group system:unauthenticated
oc policy remove-user system:no-user
echo "policy: ok"
