#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/deployments"
# This test validates deployments and the env command

os::cmd::expect_success 'oc get deploymentConfigs'
os::cmd::expect_success 'oc get dc'
os::cmd::expect_success 'oc create -f test/integration/testdata/test-deployment-config.yaml'
os::cmd::expect_success 'oc describe deploymentConfigs test-deployment-config'
os::cmd::expect_success_and_text 'oc get dc -o name' 'deploymentconfig/test-deployment-config'
os::cmd::try_until_success 'oc get rc/test-deployment-config-1'
os::cmd::expect_success_and_text 'oc describe dc test-deployment-config' 'deploymentconfig=test-deployment-config'
os::cmd::expect_success_and_text 'oc status' 'dc/test-deployment-config deploys docker.io/openshift/origin-pod:latest'
os::cmd::expect_success 'oc create -f examples/hello-openshift/hello-pod.json'
os::cmd::try_until_text 'oc status' 'pod/hello-openshift runs openshift/hello-openshift'

os::test::junit::declare_suite_start "cmd/deployments/env"
# Patch a nil list
os::cmd::expect_success 'oc env dc/test-deployment-config TEST=value'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'TEST=value'
# Remove only env in the list
os::cmd::expect_success 'oc env dc/test-deployment-config TEST-'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config --list' 'TEST=value'
# Add back to empty list
os::cmd::expect_success 'oc env dc/test-deployment-config TEST=value'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config TEST=foo --list' 'TEST=value'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config TEST=foo --list' 'TEST=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo --list' 'TEST=value'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config OTHER=foo -c ruby --list' 'OTHER=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo -c ruby*   --list' 'OTHER=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo -c *hello* --list' 'OTHER=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo -c *world  --list' 'OTHER=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo --list' 'OTHER=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo -o yaml' 'name: OTHER'
os::cmd::expect_success_and_text 'echo OTHER=foo | oc env dc/test-deployment-config -e - --list' 'OTHER=foo'
os::cmd::expect_success_and_not_text 'echo #OTHER=foo | oc env dc/test-deployment-config -e - --list' 'OTHER=foo'
os::cmd::expect_success 'oc env dc/test-deployment-config TEST=bar OTHER=baz BAR-'
os::cmd::expect_success_and_text 'oc env -f test/integration/testdata/test-deployment-config.yaml TEST=VERSION -o yaml' 'v1'
os::cmd::expect_success 'oc env dc/test-deployment-config A=a B=b C=c D=d E=e F=f G=g'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'A=a'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'B=b'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'C=c'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'D=d'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'E=e'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'F=f'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'G=g'
os::cmd::expect_success 'oc env dc/test-deployment-config H=h G- E=updated C- A-'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'B=b'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'D=d'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'E=updated'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'F=f'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'H=h'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config --list' 'A=a'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config --list' 'C=c'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config --list' 'G=g'
echo "env: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/deployments/config"
os::cmd::expect_success 'oc deploy test-deployment-config'
os::cmd::expect_success 'oc deploy dc/test-deployment-config'
os::cmd::expect_success 'oc delete deploymentConfigs test-deployment-config'
echo "deploymentConfigs: ok"
os::test::junit::declare_suite_end

os::cmd::expect_success 'oc delete all --all'
# TODO: remove, flake caused by deployment controller updating the following dc
sleep 1
os::cmd::expect_success 'oc delete all --all'

os::cmd::expect_success 'oc process -f examples/sample-app/application-template-dockerbuild.json -l app=dockerbuild | oc create -f -'
os::cmd::try_until_success 'oc get rc/database-1'

os::test::junit::declare_suite_start "cmd/deployments/get"
os::cmd::expect_success_and_text "oc get dc --show-labels" "app=dockerbuild,template=application-template-dockerbuild"
os::cmd::expect_success_and_text "oc get dc frontend --show-labels" "app=dockerbuild,template=application-template-dockerbuild"
os::cmd::expect_success_and_not_text "oc get dc" "app=dockerbuild,template=application-template-dockerbuild"
os::cmd::expect_success_and_not_text "oc get dc frontend" "app=dockerbuild,template=application-template-dockerbuild"
os::cmd::expect_success "oc process -f test/testdata/old-template.json | oc create -f -"
os::cmd::expect_success_and_text "oc get dc/eap-app -o yaml" ":latest"
echo "get: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/deployments/rollout"
os::cmd::try_until_success 'oc rollout pause dc/database'
os::cmd::try_until_text "oc get dc/database --template='{{.spec.paused}}'" "true"
os::cmd::try_until_success 'oc rollout resume dc/database'
os::cmd::try_until_text "oc get dc/database --template='{{.spec.paused}}'" "<no value>"
echo "rollout: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/deployments/rollback"
# should fail because there's no previous deployment
os::cmd::expect_failure 'oc rollback database --to-version=1 -o=yaml'
os::cmd::expect_failure 'oc rollback dc/database --to-version=1 -o=yaml'
os::cmd::expect_failure 'oc rollback dc/database --to-version=1 --dry-run'
os::cmd::expect_failure 'oc rollback database-1 -o=yaml'
os::cmd::expect_failure 'oc rollback rc/database-1 -o=yaml'
os::cmd::expect_failure 'oc rollback database -o yaml'
echo "rollback: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/deployments/stop"
os::cmd::expect_success 'oc get dc/database'
os::cmd::expect_success 'oc expose dc/database --name=fromdc'
# should be a service
os::cmd::expect_success 'oc get svc/fromdc'
os::cmd::expect_success 'oc delete svc/fromdc'
os::cmd::expect_failure_and_text 'oc stop dc/database' 'delete'
os::cmd::expect_success 'oc delete dc/database'
os::cmd::expect_failure 'oc get dc/database'
os::cmd::expect_failure 'oc get rc/database-1'
echo "stop: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/deployments/autoscale"
os::cmd::expect_success 'oc create -f test/integration/testdata/test-deployment-config.yaml'
os::cmd::expect_success 'oc autoscale dc/test-deployment-config --max 5'
os::cmd::expect_success_and_text "oc get hpa/test-deployment-config --template='{{.spec.maxReplicas}}'" "5"
os::cmd::expect_success 'oc delete dc/test-deployment-config'
os::cmd::expect_success 'oc delete hpa/test-deployment-config'
echo "autoscale: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/deployments/setimage"
os::cmd::expect_success 'oc create -f test/integration/testdata/test-deployment-config.yaml'
os::cmd::expect_success 'oc set image dc/test-deployment-config ruby-helloworld=myshinynewimage --source=docker'
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o jsonpath='{.spec.template.spec.containers[0].image}'" "myshinynewimage"
os::cmd::expect_success 'oc delete dc/test-deployment-config'
echo "set image: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/deployments/setdeploymenthook"
# Validate the set deployment-hook command
arg="-f test/integration/testdata/test-deployment-config.yaml"
os::cmd::expect_failure_and_text "oc set deployment-hook" "error: one or more deployment configs"
os::cmd::expect_failure_and_text "oc set deployment-hook ${arg}" "error: you must specify one of --pre, --mid, or --post"
os::cmd::expect_failure_and_text "oc set deployment-hook ${arg} -o yaml --pre -- mycmd" 'Error from server: deploymentconfigs'
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --local -o yaml --post -- mycmd" 'mycmd'
os::cmd::expect_success_and_not_text "oc set deployment-hook ${arg} --local -o yaml --post -- mycmd | oc set deployment-hook -f - --local -o yaml --post --remove" 'mycmd'
os::cmd::expect_success "oc create ${arg}"
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --pre  -o yaml -- echo 'hello world'" 'pre:'
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --pre  -o yaml -- echo 'hello world'" 'execNewPod:'
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --pre  -o yaml -- echo 'hello world'" '\- echo'
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --pre  -o yaml -- echo 'hello world'" '\- hello world'
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --post -o yaml -- echo 'hello world'" 'post:'
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --mid  -o yaml -- echo 'hello world'" 'mid:'
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --pre --failure-policy=ignore -o yaml -- echo 'hello world'" 'failurePolicy: Ignore'
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --pre --failure-policy=retry  -o yaml -- echo 'hello world'" 'failurePolicy: Retry'
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --pre --failure-policy=abort  -o yaml -- echo 'hello world'" 'failurePolicy: Abort'
# Non-existent container
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --pre --container=blah -o yaml -- echo 'hello world'" 'does not have a container named'
# Non-existent volume
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --pre --volumes=blah -o yaml -- echo 'hello world'" 'does not have a volume named'
# Existing container
os::cmd::expect_success_and_not_text "oc set deployment-hook ${arg} --pre --container=ruby-helloworld -o yaml -- echo 'hello world'" 'does not have a container named'
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --pre --container=ruby-helloworld -o yaml -- echo 'hello world'" 'containerName: ruby-helloworld'
# Existing volume
os::cmd::expect_success_and_not_text "oc set deployment-hook ${arg} --pre --volumes=vol1 -o yaml -- echo 'hello world'" 'does not have a volume named'
os::cmd::expect_success_and_text "oc set deployment-hook ${arg} --pre --volumes=vol1 -o yaml -- echo 'hello world'" '\- vol1'
os::cmd::expect_success 'oc delete dc/test-deployment-config'
# Server object tests
os::cmd::expect_success "oc create -f test/integration/testdata/test-deployment-config.yaml"
os::cmd::expect_failure_and_text "oc set deployment-hook dc/test-deployment-config --pre" "you must specify a command"
os::cmd::expect_success_and_text "oc set deployment-hook test-deployment-config --pre -- echo 'hello world'" "updated"
os::cmd::expect_success_and_text "oc set deployment-hook dc/test-deployment-config --pre -- echo 'hello world'" "was not changed"
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o yaml" "pre:"
os::cmd::expect_success_and_text "oc set deployment-hook dc/test-deployment-config --pre --failure-policy=abort -- echo 'test'" "updated"
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o yaml" "failurePolicy: Abort"
os::cmd::expect_success_and_text "oc set deployment-hook --all --pre -- echo 'all dc'" "updated"
os::cmd::expect_success_and_text "oc get dc -o yaml" "all dc"
os::cmd::expect_success_and_text "oc set deployment-hook dc/test-deployment-config --pre --remove" "updated"
os::cmd::expect_success_and_not_text "oc get dc/test-deployment-config -o yaml" "pre:"
# Environment handling
os::cmd::expect_success_and_text "oc set deployment-hook dc/test-deployment-config --pre -o yaml --environment X=Y,Z=W -- echo 'test'" "value: Y,Z=W"
os::cmd::expect_success_and_text "oc set deployment-hook dc/test-deployment-config --pre -o yaml --environment X=Y,Z=W -- echo 'test'" "no longer accepts comma-separated list"
os::cmd::expect_success_and_not_text "oc set deployment-hook dc/test-deployment-config --pre -o yaml --environment X=Y -- echo 'test'" "no longer accepts comma-separated list"

os::cmd::expect_success "oc delete dc/test-deployment-config"
echo "set deployment-hook: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
