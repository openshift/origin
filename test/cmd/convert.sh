#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all --all
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/convert"
# This test validates the convert command

os::cmd::expect_success "oc convert -f test/testdata/convert/job-v1.yaml | grep 'apiVersion: batch/v1'"
os::cmd::expect_success "oc convert -f test/testdata/convert/job-v2.json | grep 'apiVersion: batch/v1beta1'"

os::cmd::expect_success_and_text "oc convert -f test/testdata/convert | oc create --dry-run -f -" 'job "pi" created'
os::cmd::expect_success_and_text "oc convert -f test/testdata/convert | oc create --dry-run -f -" 'cronjob "hello" created'

echo "convert: ok"
os::test::junit::declare_suite_end
