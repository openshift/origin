#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    rm -rf "${TMP_COMPLETION_ROOT}"
    os::test::junit::generate_report
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

platform="$(os::build::host_platform)"
if [[ "${platform}" != "linux/amd64" ]]; then
  os::log::warning "Completions cannot be verified on non-Linux systems (${platform})"
  exit 0
fi

COMPLETION_ROOT_REL="contrib/completions"
COMPLETION_ROOT="${OS_ROOT}/${COMPLETION_ROOT_REL}"
TMP_COMPLETION_ROOT_REL="_output/verify-generated-completions/"
TMP_COMPLETION_ROOT="${OS_ROOT}/${TMP_COMPLETION_ROOT_REL}/${COMPLETION_ROOT_REL}"

os::test::junit::declare_suite_start "verify/completions"
os::cmd::expect_success "${OS_ROOT}/hack/update-generated-completions.sh ${TMP_COMPLETION_ROOT_REL}"
os::cmd::expect_success "diff -Naupr -x 'OWNERS' ${COMPLETION_ROOT} ${TMP_COMPLETION_ROOT}"
os::test::junit::declare_suite_end