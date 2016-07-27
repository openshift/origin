#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/admin/serviceaccounts"
# create a new service account
os::cmd::expect_success_and_text 'oc create serviceaccount my-sa-name' 'serviceaccount "my-sa-name" created'
os::cmd::expect_success 'oc get sa my-sa-name'

# extract token and ensure it links us back to the service account
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
