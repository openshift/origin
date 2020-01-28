#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::test::junit::generate_report
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::test::junit::declare_suite_start "verify/bindata"
os::cmd::expect_success "${OS_ROOT}/hack/update-generated-bindata.sh"
os::cmd::expect_success "git diff --quiet vendor/k8s.io/kubernetes/staging/src/k8s.io/kubectl/pkg/generated/bindata.go"
os::cmd::expect_success "git diff --quiet vendor/k8s.io/kubernetes/test/e2e/generated/bindata.go"
os::cmd::expect_success "git diff --quiet ${OS_ROOT}/test/extended/util/annotate/generated/"
os::cmd::expect_success "git diff --quiet ${OS_ROOT}/test/extended/testdata/bindata.go"

os::test::junit::declare_suite_end
