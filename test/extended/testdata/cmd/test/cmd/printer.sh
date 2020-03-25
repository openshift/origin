#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Test that resource printer includes resource kind on multiple resources
os::test::junit::declare_suite_start "cmd/basicresources/printer"
os::cmd::expect_success 'oc create imagestream test1'
os::cmd::expect_success 'oc new-app node'
os::cmd::expect_success_and_text 'oc get all' 'imagestream.image.openshift.io/test1'
os::cmd::expect_success_and_not_text 'oc get is' 'imagestream.image.openshift.io/test1'

# Test that resource printer includes namespaces for buildconfigs with custom strategies
os::cmd::expect_success 'oc create -f ${TEST_DATA}/application-template-custombuild.json'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample' 'deployment.*.apps.* "frontend" created'
os::cmd::expect_success_and_text 'oc get all --all-namespaces' 'cmd-printer[\ ]+buildconfig.build.openshift.io\/ruby\-sample\-build'

# Test that infos printer supports all outputFormat options
os::cmd::expect_success_and_text 'oc new-app node --dry-run -o yaml | oc set env --local -f - MYVAR=value' 'deployment.*.apps.*/node updated'
os::cmd::expect_success_and_text 'oc new-app node --dry-run -o yaml | oc set env --local -f - MYVAR=value -o yaml' 'apiVersion: apps.*/v1'
os::cmd::expect_success_and_text 'oc new-app node --dry-run -o yaml | oc set env --local -f - MYVAR=value -o json' '"apiVersion": "apps.*/v1"'
echo "resource printer: ok"
os::test::junit::declare_suite_end
