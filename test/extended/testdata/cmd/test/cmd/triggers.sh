#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all --all
  exit 0
) &>/dev/null


url=":${API_PORT:-8443}"
project="$(oc project -q)"

os::test::junit::declare_suite_start "cmd/triggers"
# This test validates triggers

os::cmd::expect_success 'oc new-app centos/ruby-22-centos7~https://github.com/openshift/ruby-hello-world.git'
os::cmd::expect_success 'oc get bc/ruby-hello-world'
os::cmd::expect_success 'oc get dc/ruby-hello-world'

os::cmd::expect_success "oc new-build --name=scratch --docker-image=scratch --dockerfile='FROM scratch'"

os::test::junit::declare_suite_start "cmd/triggers/buildconfigs"
## Build configs

# error conditions
os::cmd::expect_failure_and_text 'oc set triggers bc/ruby-hello-world --remove --remove-all' 'specify either --remove or --remove-all'
os::cmd::expect_failure_and_text 'oc set triggers bc/ruby-hello-world --auto --manual' 'at most one of --auto or --manual'
# print
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'config.*true'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'image.*ruby-22-centos7:latest.*true'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'webhook'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'github'
# note, oc new-app currently does not set up gitlab or bitbucket webhooks by default
# remove all
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --remove-all' 'updated'
# add a new secretReference style webhook to the BC
os::cmd::expect_success "oc patch bc/ruby-hello-world -p '{\"spec\":{\"triggers\":[{\"github\":{\"secretReference\":{\"name\":\"mysecret\"}},\"type\":\"GitHub\"}]}}'"
os::cmd::expect_success_and_text 'oc describe buildConfigs ruby-hello-world' "Webhook GitHub"
# make sure we can still add/set other triggers
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --from-gitlab' 'updated'
os::cmd::expect_success_and_text 'oc describe buildConfigs ruby-hello-world' "Webhook GitHub"
os::cmd::expect_success_and_text 'oc describe buildConfigs ruby-hello-world' "Webhook GitLab"
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --remove-all' 'updated'

os::cmd::expect_success_and_not_text 'oc set triggers bc/ruby-hello-world' 'webhook|github'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'config.*false'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'image.*ruby-22-centos7:latest.*false'
# set github hook
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --from-github' 'updated'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'github'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --remove --from-github' 'updated'
os::cmd::expect_success_and_not_text 'oc set triggers bc/ruby-hello-world' 'github'
# set webhook
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --from-webhook' 'updated'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'webhook'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --remove --from-webhook' 'updated'
os::cmd::expect_success_and_not_text 'oc set triggers bc/ruby-hello-world' 'webhook'
# set webhook plus envvars
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --from-webhook-allow-env' 'updated'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'webhook'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --remove --from-webhook-allow-env' 'updated'
os::cmd::expect_success_and_not_text 'oc set triggers bc/ruby-hello-world' 'webhook'
# set gitlab hook
os::cmd::expect_success 'oc set triggers bc/ruby-hello-world --from-gitlab'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'gitlab'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --remove --from-gitlab' 'updated'
os::cmd::expect_success_and_not_text 'oc set triggers bc/ruby-hello-world' 'gitlab'
# set bitbucket hook
os::cmd::expect_success 'oc set triggers bc/ruby-hello-world --from-bitbucket'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'bitbucket'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --remove --from-bitbucket' 'updated'
os::cmd::expect_success_and_not_text 'oc set triggers bc/ruby-hello-world' 'bitbucket'
# set from-image
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --from-image=ruby-22-centos7:other' 'updated'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world' 'image.*ruby-22-centos7:other.*true'
# manual and remove both clear build configs
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --from-image=ruby-22-centos7:other --manual' 'updated'
os::cmd::expect_success_and_not_text 'oc set triggers bc/ruby-hello-world' 'image.*ruby-22-centos7:other.*false'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --from-image=ruby-22-centos7:other' 'updated'
os::cmd::expect_success_and_text 'oc set triggers bc/ruby-hello-world --from-image=ruby-22-centos7:other --remove' 'updated'
os::cmd::expect_success_and_not_text 'oc set triggers bc/ruby-hello-world' 'image.*ruby-22-centos7:other'
# test --all
os::cmd::expect_success_and_text 'oc set triggers bc --all' 'buildconfigs/ruby-hello-world.*image.*ruby-22-centos7:latest.*false'
os::cmd::expect_success_and_text 'oc set triggers bc --all --auto' 'updated'
os::cmd::expect_success_and_text 'oc set triggers bc --all' 'buildconfigs/ruby-hello-world.*image.*ruby-22-centos7:latest.*true'
# set a trigger on a build that doesn't have an imagestream strategy.from-image
os::cmd::expect_success_and_text 'oc set triggers bc/scratch --from-image=test:latest' 'updated'

os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/triggers/deploymentconfigs"
## Deployment configs

# error conditions
os::cmd::expect_failure_and_text 'oc set triggers dc/ruby-hello-world --from-github' 'deployment configs do not support GitHub web hooks'
os::cmd::expect_failure_and_text 'oc set triggers dc/ruby-hello-world --from-webhook' 'deployment configs do not support web hooks'
os::cmd::expect_failure_and_text 'oc set triggers dc/ruby-hello-world --from-gitlab' 'deployment configs do not support GitLab web hooks'
os::cmd::expect_failure_and_text 'oc set triggers dc/ruby-hello-world --from-bitbucket' 'deployment configs do not support Bitbucket web hooks'
os::cmd::expect_failure_and_text 'oc set triggers dc/ruby-hello-world --from-image=test:latest' 'you must specify --containers when setting --from-image'
os::cmd::expect_failure_and_text 'oc set triggers dc/ruby-hello-world --from-image=test:latest --containers=other' 'not all container names exist: other \(accepts: ruby-hello-world\)'
# print
os::cmd::expect_success_and_text 'oc set triggers dc/ruby-hello-world' 'config.*true'
os::cmd::expect_success_and_text 'oc set triggers dc/ruby-hello-world' 'image.*ruby-hello-world:latest \(ruby-hello-world\).*true'
os::cmd::expect_success_and_not_text 'oc set triggers dc/ruby-hello-world' 'webhook|github|gitlab|bitbucket'
os::cmd::expect_success_and_not_text 'oc set triggers dc/ruby-hello-world' 'gitlab'
os::cmd::expect_success_and_not_text 'oc set triggers dc/ruby-hello-world' 'bitbucket'
# remove all
os::cmd::expect_success_and_text 'oc set triggers dc/ruby-hello-world --remove-all' 'updated'
os::cmd::expect_success_and_not_text 'oc set triggers dc/ruby-hello-world' 'webhook|github|image|gitlab|bitbucket'
os::cmd::expect_success_and_text 'oc set triggers dc/ruby-hello-world' 'config.*false'
# auto
os::cmd::expect_success_and_text 'oc set triggers dc/ruby-hello-world --auto' 'updated'
os::cmd::expect_success_and_text 'oc set triggers dc/ruby-hello-world' 'config.*true'
os::cmd::expect_success_and_text 'oc set triggers dc/ruby-hello-world --from-image=ruby-hello-world:latest -c ruby-hello-world' 'updated'
os::cmd::expect_success_and_text 'oc set triggers dc/ruby-hello-world' 'image.*ruby-hello-world:latest \(ruby-hello-world\).*true'
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/triggers/annotations"
## Deployment configs

os::cmd::expect_success 'oc create deployment test --image=busybox'

# error conditions
os::cmd::expect_failure_and_text 'oc set triggers deploy/test --from-github' 'does not support GitHub web hooks'
os::cmd::expect_failure_and_text 'oc set triggers deploy/test --from-webhook' 'does not support web hooks'
os::cmd::expect_failure_and_text 'oc set triggers deploy/test --from-gitlab' 'does not support GitLab web hooks'
os::cmd::expect_failure_and_text 'oc set triggers deploy/test --from-bitbucket' 'does not support Bitbucket web hooks'
os::cmd::expect_failure_and_text 'oc set triggers deploy/test --from-image=test:latest' 'you must specify --containers when setting --from-image'
os::cmd::expect_failure_and_text 'oc set triggers deploy/test --from-image=test:latest --containers=other' 'not all container names exist: other \(accepts: busybox\)'
# print
os::cmd::expect_success_and_text 'oc set triggers deploy/test' 'config.*true'
os::cmd::expect_success_and_not_text 'oc set triggers deploy/test' 'webhook|github|gitlab|bitbucket'
os::cmd::expect_success_and_not_text 'oc set triggers deploy/test' 'gitlab'
os::cmd::expect_success_and_not_text 'oc set triggers deploy/test' 'bitbucket'
# remove all
os::cmd::expect_success_and_text 'oc set triggers deploy/test --remove-all' 'updated'
os::cmd::expect_success_and_not_text 'oc set triggers deploy/test' 'webhook|github|image|gitlab|bitbucket'
os::cmd::expect_success_and_text 'oc set triggers deploy/test' 'config.*false'
# auto
os::cmd::expect_success_and_text 'oc set triggers deploy/test --auto' 'updated'
os::cmd::expect_success_and_text 'oc set triggers deploy/test' 'config.*true'
os::cmd::expect_success_and_text 'oc set triggers deploy/test --from-image=ruby-hello-world:latest -c busybox' 'updated'
os::cmd::expect_success_and_text 'oc set triggers deploy/test' 'image.*ruby-hello-world:latest \(busybox\).*true'
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
