#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/set-probe-liveness"
# This test setting a liveness probe, without warning about replication controllers whose deployment depends on deployment configs
os::cmd::expect_success_and_text 'oc create -f pkg/oc/graph/genericgraph/test/simple-deployment.yaml' 'deploymentconfig "simple-deployment" created'
os::cmd::expect_success_and_text 'oc status --suggest' 'dc/simple-deployment has no liveness probe'

# test --local flag
os::cmd::expect_failure_and_text 'oc set probe dc/simple-deployment --liveness --get-url=http://google.com:80 --local' 'You must provide one or more resources by argument or filename'
# test --dry-run flag with -o formats
os::cmd::expect_success_and_text 'oc set probe dc/simple-deployment --liveness --get-url=http://google.com:80 --dry-run' 'simple-deployment'
os::cmd::expect_success_and_text 'oc set probe dc/simple-deployment --liveness --get-url=http://google.com:80 --dry-run -o name' 'deploymentconfigs/simple-deployment'
# ensure backwards compatibility with -o formats acting as --dry-run (e.g. all commands after this one succeed if specifying -o without --dry-run does not mutate resources in server)
os::cmd::expect_success_and_text 'oc set probe dc/simple-deployment --liveness --get-url=http://google.com:80 -o yaml' 'name: simple-deployment'

os::cmd::expect_success_and_not_text 'oc status --suggest' 'rc/simple-deployment-1 has no liveness probe'
os::cmd::expect_success_and_text 'oc set probe dc/simple-deployment --liveness --get-url=http://google.com:80' 'deploymentconfig "simple-deployment" updated'
os::cmd::expect_success_and_not_text 'oc status --suggest' 'dc/simple-deployment has no liveness probe'
echo "set-probe-liveness: ok"
os::test::junit::declare_suite_end
