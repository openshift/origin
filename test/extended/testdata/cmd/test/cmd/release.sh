#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Test that resource printer includes resource kind on multiple resources
os::test::junit::declare_suite_start "cmd/release"
os::cmd::expect_success "oc adm release new --from-release ${RELEASE_IMAGE_LATEST}"
echo "adm release: ok"
os::test::junit::declare_suite_end
