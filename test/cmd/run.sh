#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/run"
# This test validates the value of --image for oc run
os::cmd::expect_success_and_text 'oc run newdcforimage --image=validimagevalue' 'deploymentconfig "newdcforimage" created'
os::cmd::expect_failure_and_text 'oc run newdcforimage2 --image="InvalidImageValue0192"' 'error: Invalid image name "InvalidImageValue0192": invalid reference format'
os::cmd::expect_failure_and_text 'oc run test1 --image=busybox --attach --dry-run' "dry-run can't be used with attached containers options"
os::cmd::expect_failure_and_text 'oc run test1 --image=busybox --stdin --dry-run' "dry-run can't be used with attached containers options"
echo "oc run: ok"
os::test::junit::declare_suite_end
