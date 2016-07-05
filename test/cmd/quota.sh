#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/quota"

os::cmd::expect_success 'oc new-project foo --as=deads'
os::cmd::expect_success 'oc label namespace/foo owner=deads'
os::cmd::expect_success 'oc create clusterquota for-deads --project-selector=owner=deads --hard=pods=10'
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n foo --as deads -o name' "for-deads"
os::cmd::expect_success 'oc delete project foo'

echo "quota: ok"
os::test::junit::declare_suite_end
