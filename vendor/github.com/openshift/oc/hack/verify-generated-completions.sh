#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    rm -rf "${TMP_COMPLETION_ROOT}"
    exit "${return_code}"
}
trap "cleanup" EXIT

COMPLETION_ROOT_REL="contrib/completions"
COMPLETION_ROOT="${OS_ROOT}/${COMPLETION_ROOT_REL}"
TMP_COMPLETION_ROOT_REL="_output/verify-generated-completions/"
TMP_COMPLETION_ROOT="${OS_ROOT}/${TMP_COMPLETION_ROOT_REL}"

platform="$(os::build::host_platform)"
if [[ "${platform}" != "linux/amd64" ]]; then
  os::log::warning "Completions cannot be verified on non-Linux systems (${platform})"
  exit 0
fi

${OS_ROOT}/hack/update-generated-completions.sh ${TMP_COMPLETION_ROOT_REL}
diff -Naupr -x 'OWNERS' ${COMPLETION_ROOT} ${TMP_COMPLETION_ROOT}
