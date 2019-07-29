#!/usr/bin/env bash

# This script verifies that package trees
# conform to our import restrictions
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::test::junit::generate_report
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::util::ensure::built_binary_exists 'import-verifier'

os::test::junit::declare_suite_start "verify/imports"
os::cmd::expect_success "import-verifier ${OS_ROOT}/hack/import-restrictions.json"

# quick and dirty check that nothing under vendored kubernetes imports something from origin
os::cmd::expect_failure "egrep -r '\"github.com/openshift/origin/[^\"]+\"$' vendor/k8s.io/kubernetes"
os::cmd::expect_failure "egrep -r '\"github.com/openshift/openshift-apiserver/[^\"]+\"$' vendor/k8s.io/kubernetes"
os::cmd::expect_failure "egrep -r '\"github.com/openshift/openshift-controller-manager/[^\"]+\"$' vendor/k8s.io/kubernetes"
os::cmd::expect_failure "egrep -r '\"github.com/openshift/oc/[^\"]+\"$' vendor/k8s.io/kubernetes"
os::cmd::expect_failure "egrep -r '\"github.com/openshift/template-service-broker/[^\"]+\"$' vendor/k8s.io/kubernetes"

os::test::junit::declare_suite_end

