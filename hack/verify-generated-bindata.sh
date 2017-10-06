#!/bin/bash
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
os::cmd::expect_success "diff -Naup ${OS_ROOT}/pkg/oc/bootstrap/bindata.go ${TMP_GENERATED_BOOTSTRAP_DIR}/pkg/oc/bootstrap/bindata.go"
os::cmd::expect_success "diff -Naup ${OS_ROOT}/test/extended/testdata/bindata.go ${TMP_GENERATED_BOOTSTRAP_DIR}/test/extended/testdata/bindata.go"
os::test::junit::declare_suite_end