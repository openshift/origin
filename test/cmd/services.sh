#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/create-service-nodeport"
# This test validates the 'create service nodeport' command and its "--node-port" and "--tcp" options
os::cmd::expect_success_and_text 'oc create service nodeport mynodeport --tcp=8080:7777 --node-port=30000' 'service "mynodeport" created'
os::cmd::expect_failure_and_text 'oc create service nodeport mynodeport --tcp=8080:7777 --node-port=30000' 'provided port is already allocated'
os::cmd::expect_failure_and_text 'oc create service nodeport mynodeport --tcp=8080:7777 --node-port=300' 'provided port is not in the valid range. The range of valid ports is 30000-32767'
os::cmd::expect_success_and_text 'oc describe service mynodeport' 'NodePort\:.*30000'
os::cmd::expect_success_and_text 'oc describe service mynodeport' 'NodePort\:.*8080-7777'

echo "create-services-nodeport: ok"
os::test::junit::declare_suite_end
