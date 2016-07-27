#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete project/example
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/admin/complex-scenarios"
os::cmd::expect_success 'oadm new-project example --admin="createuser"'
os::cmd::expect_success 'oc project example'
os::cmd::try_until_success 'oc get serviceaccount default'
os::cmd::expect_success 'oc create -f test/testdata/app-scenarios'
os::cmd::expect_success 'oc status'
os::cmd::expect_success_and_text 'oc status -o dot' '"example"'
echo "complex-scenarios: ok"
os::test::junit::declare_suite_end
