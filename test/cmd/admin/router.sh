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
  oc delete sa/router -n default
  exit 0
) &>/dev/null


defaultimage="openshift/origin-\${component}:latest"
USE_IMAGES=${USE_IMAGES:-$defaultimage}

os::test::junit::declare_suite_start "cmd/admin/router"
# Test running a router
os::cmd::expect_failure_and_text 'oadm router --dry-run' 'does not exist'
encoded_json='{"kind":"ServiceAccount","apiVersion":"v1","metadata":{"name":"router"}}'
os::cmd::expect_success "echo '${encoded_json}' | oc create -f - -n default"
os::cmd::expect_success "oadm policy add-scc-to-user privileged system:serviceaccount:default:router"
os::cmd::expect_success_and_text "oadm router -o yaml --credentials=${KUBECONFIG} --service-account=router -n default" 'image:.*\-haproxy\-router:'
os::cmd::expect_success "oadm router --credentials=${KUBECONFIG} --images='${USE_IMAGES}' --service-account=router -n default"
os::cmd::expect_success_and_text 'oadm router -n default' 'service exists'
os::cmd::expect_success_and_text 'oc get dc/router -o yaml -n default' 'readinessProbe'
echo "router: ok"
os::test::junit::declare_suite_end
