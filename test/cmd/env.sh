#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/set-env"
# This test validates the value of --image for oc run
os::cmd::expect_success 'oc new-app node'
os::cmd::expect_failure_and_text 'oc set env dc/node' 'error: at least one environment variable must be provided'
os::cmd::expect_success_and_text 'oc set env dc/node key=value' 'deploymentconfig "node" updated'
os::cmd::expect_success_and_text 'oc set env dc/node --list' 'deploymentconfigs node, container node'
os::cmd::expect_success_and_text 'oc set env dc --all --containers="node" key-' 'deploymentconfig "node" updated'
os::cmd::expect_failure_and_text 'oc set env dc --all --containers="node"' 'error: at least one environment variable must be provided'
os::cmd::expect_failure_and_not_text 'oc set env --from=secret/mysecret dc/node' 'error: at least one environment variable must be provided'
os::cmd::expect_failure_and_text 'oc set env dc/node test#abc=1234' 'environment variables must be of the form key=value'

# check that env vars are not split at commas
os::cmd::expect_success_and_text 'oc set env -o yaml dc/node PASS=x,y=z' 'value: x,y=z'
os::cmd::expect_success_and_text 'oc set env -o yaml dc/node --env PASS=x,y=z' 'value: x,y=z'
# warning is printed when --env has comma in it
os::cmd::expect_success_and_text 'oc set env dc/node --env PASS=x,y=z' 'no longer accepts comma-separated list'
# warning is not printed for variables passed as positional arguments
os::cmd::expect_success_and_not_text 'oc set env dc/node PASS=x,y=z' 'no longer accepts comma-separated list'

echo "oc set env: ok"
os::test::junit::declare_suite_end
