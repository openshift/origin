#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/edit"
# This test validates the edit command

os::cmd::expect_success 'oc create -f examples/hello-openshift/hello-pod.json'

os::cmd::expect_success_and_text 'OC_EDITOR=cat oc edit pod/hello-openshift' 'Edit cancelled'
os::cmd::expect_success_and_text 'OC_EDITOR=cat oc edit pod/hello-openshift' 'name: hello-openshift'
os::cmd::expect_success_and_text 'OC_EDITOR=cat oc edit --windows-line-endings pod/hello-openshift | file -' 'CRLF'
os::cmd::expect_success_and_not_text 'OC_EDITOR=cat oc edit --windows-line-endings=false pod/hello-openshift | file -' 'CRFL'

os::cmd::expect_success 'oc create -f test/testdata/services.yaml'
os::cmd::expect_success_and_text 'OC_EDITOR=cat oc edit svc' 'kind: List'
echo "edit: ok"
os::test::junit::declare_suite_end
