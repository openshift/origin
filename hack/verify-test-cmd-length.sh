#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/lib/init.sh"

cd "${OS_ROOT}"

echo "===== Verifying CLI Integration Test Bucket Lengths ====="

buckets="$( find "${OS_ROOT}/test/cmd" -type f -name '*.sh' | sort )"

blacklist="
test/cmd/basicresources.sh
test/cmd/builds.sh
test/cmd/deployments.sh
test/cmd/help.sh
test/cmd/images.sh
test/cmd/newapp.sh
test/cmd/policy.sh
"

IFS=$'\n'
for bucket in ${buckets}; do
    relative_bucket="$( os::util::repository_relative_path "${bucket}" )"

    if grep -q "${relative_bucket}" <<<"${blacklist}"; then
        # this test bucket is on our blacklist, we can skip it
        continue
    fi

    num_tests="$( grep -c 'os::cmd::' "${bucket}" )"
    if (( num_tests > 50 )); then
        os::log::error "Test bucket ${relative_bucket} has $(( num_tests - 50 )) too many tests!"
        failed="true"
    elif [[ -n "${VERBOSE:-}" ]]; then
        os::log::info "Test bucket ${relative_bucket} has $(( 50 - num_tests )) fewer tests than the quota."
    fi
done

if [[ -n "${failed:-}" ]]; then
    os::log::error "Split the long test buckets into smaller buckets for readability and addressability."
    os::log::error "Failure: validating test-cmd bucket lengths failed"
else
    os::log::info "Success: validating test-cmd bucket lengths succeeded"
fi

# ex: ts=2 sw=2 et filetype=sh
