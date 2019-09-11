#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    rm -rf "${TMP_GENERATED_BOOTSTRAP_DIR}"
    os::test::junit::generate_report
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

TMP_GENERATED_BOOTSTRAP_DIR="_output/verify-bindata"

os::test::junit::declare_suite_start "verify/bindata"
os::cmd::expect_success "OUTPUT_ROOT=${TMP_GENERATED_BOOTSTRAP_DIR} ${OS_ROOT}/hack/update-generated-bindata.sh"
os::cmd::expect_success "git diff --quiet vendor/k8s.io/kubernetes/staging/src/k8s.io/kubectl/pkg/generated/bindata.go"
os::cmd::expect_success "git diff --quiet vendor/k8s.io/kubernetes/test/e2e/generated/bindata.go"
os::test::junit::declare_suite_end
