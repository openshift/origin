#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# This test validates user level policy

oc policy add-role-to-group cluster-admin system:unauthenticated
oc policy add-role-to-user cluster-admin system:no-user
oc policy remove-role-from-group cluster-admin system:unauthenticated
oc policy remove-role-from-user cluster-admin system:no-user
oc policy remove-group system:unauthenticated
oc policy remove-user system:no-user
echo "policy: ok"
