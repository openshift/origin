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

# ensure deleting a var through --env does not result in an error message
os::cmd::expect_success_and_text 'oc set env dc/node key=value' 'deploymentconfig "node" updated'
os::cmd::expect_success_and_text 'oc set env dc --all --containers="node" --env=key-' 'deploymentconfig "node" updated'
# ensure deleting a var through --env actually deletes the env var
os::cmd::expect_success_and_not_text "oc get dc/node -o jsonpath='{ .spec.template.spec.containers[?(@.name==\"node\")].env }'" 'name\:key'

# check that env vars are not split at commas
os::cmd::expect_success_and_text 'oc set env -o yaml dc/node PASS=x,y=z' 'value: x,y=z'
os::cmd::expect_success_and_text 'oc set env -o yaml dc/node --env PASS=x,y=z' 'value: x,y=z'
# warning is printed when --env has comma in it
os::cmd::expect_success_and_text 'oc set env dc/node --env PASS=x,y=z' 'no longer accepts comma-separated list'
# warning is not printed for variables passed as positional arguments
os::cmd::expect_success_and_not_text 'oc set env dc/node PASS=x,y=z' 'no longer accepts comma-separated list'

# create a build-config object with the JenkinsPipeline strategy
os::cmd::expect_success 'oc process -p NAMESPACE=openshift -f examples/jenkins/jenkins-ephemeral-template.json | oc create -f -'
os::cmd::expect_success "echo 'apiVersion: v1
kind: BuildConfig
metadata:
  name: fake-pipeline
spec:
  source:
    git:
      uri: git://github.com/openshift/ruby-hello-world.git
  strategy:
    jenkinsPipelineStrategy: {}
' | oc create -f -"

# ensure build-config has been created and that its type is "JenkinsPipeline"
os::cmd::expect_success_and_text "oc get bc fake-pipeline -o jsonpath='{ .spec.strategy.type }'" 'JenkinsPipeline'
# attempt to set an environment variable
os::cmd::expect_success_and_text 'oc set env bc/fake-pipeline FOO=BAR' 'buildconfig "fake\-pipeline" updated'
# ensure environment variable was set
os::cmd::expect_success_and_text "oc get bc fake-pipeline -o jsonpath='{ .spec.strategy.jenkinsPipelineStrategy.env }'" 'name\:FOO'
os::cmd::expect_success 'oc delete bc fake-pipeline'

echo "oc set env: ok"
os::test::junit::declare_suite_end
