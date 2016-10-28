#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Test that resource printer includes resource kind on multiple resources
os::test::junit::declare_suite_start "cmd/basicresources/printer"
os::cmd::expect_success 'oc create imagestream test1'
os::cmd::expect_success 'oc new-app node'
os::cmd::expect_success_and_text 'oc get all' 'is/test1'
os::cmd::expect_success_and_not_text 'oc get is' 'is/test1'

# Test that resource printer includes namespaces for buildconfigs with custom strategies
os::cmd::expect_success 'oc create -f examples/sample-app/application-template-custombuild.json'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample' 'deploymentconfig "frontend" created'
os::cmd::expect_success_and_text 'oc get all --all-namespaces' 'cmd-printer[\ ]+bc\/ruby\-sample\-build'

# Test that infos printer supports all outputFormat options
os::cmd::expect_success_and_text 'oc new-app node -o yaml | oc set env -f - MYVAR=value' 'deploymentconfig "node" updated'
os::cmd::expect_success 'oc new-app node -o yaml | oc set env -f - MYVAR=value -o custom-colums="NAME:.metadata.name"'
os::cmd::expect_success_and_text 'oc new-app node -o yaml | oc set env -f - MYVAR=value -o yaml' 'apiVersion: v1'
os::cmd::expect_success_and_text 'oc new-app node -o yaml | oc set env -f - MYVAR=value -o json' '"apiVersion": "v1"'
os::cmd::expect_success_and_text 'oc new-app node -o yaml | oc set env -f - MYVAR=value -o wide' 'node'
os::cmd::expect_success_and_text 'oc new-app node -o yaml | oc set env -f - MYVAR=value -o name' 'deploymentconfig/node'
echo "resource printer: ok"
os::test::junit::declare_suite_end