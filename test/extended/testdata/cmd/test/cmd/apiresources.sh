#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/apiresources"

os::cmd::expect_success_and_text 'oc api-resources' 'imagestreamtags'
os::cmd::expect_success_and_text 'oc api-resources --api-group=build.openshift.io' 'BuildConfig'
os::cmd::expect_success_and_text 'oc api-resources --namespaced=false' 'Image'
os::cmd::expect_success_and_text 'oc api-resources --verbs=get' 'project.openshift.io'

os::cmd::expect_success_and_text 'oc api-versions' 'route.openshift.io/v1'

echo "apiresources: ok"
os::test::junit::declare_suite_end
