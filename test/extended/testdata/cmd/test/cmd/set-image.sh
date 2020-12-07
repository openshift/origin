#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/oc/set/image"
os::cmd::expect_success 'oc create -f ${TEST_DATA}/test-deployment-config.yaml'
os::cmd::expect_success 'oc create -f ${TEST_DATA}/hello-openshift/hello-pod.json'
os::cmd::expect_success 'oc create -f ${TEST_DATA}/image-streams/image-streams-centos7.json'
os::cmd::try_until_success 'oc get imagestreamtags ruby:2.5'
os::cmd::try_until_success 'oc get imagestreamtags ruby:2.6'

# test --local flag
os::cmd::expect_failure_and_text 'oc set image dc/test-deployment-config ruby-helloworld=ruby:2.5 --local' 'you must specify resources by --filename when --local is set.'
# test --dry-run flag with -o formats
os::cmd::expect_success_and_text 'oc set image dc/test-deployment-config ruby-helloworld=ruby:2.5 --source=istag --dry-run' 'test-deployment-config'
os::cmd::expect_success_and_text 'oc set image dc/test-deployment-config ruby-helloworld=ruby:2.5 --source=istag --dry-run -o name' 'deploymentconfig.apps.openshift.io/test-deployment-config'
os::cmd::expect_success_and_text 'oc set image dc/test-deployment-config ruby-helloworld=ruby:2.5 --source=istag --dry-run' 'deploymentconfig.apps.openshift.io/test-deployment-config image updated \(dry run\)'

os::cmd::expect_success 'oc set image dc/test-deployment-config ruby-helloworld=ruby:2.6 --source=istag'
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o jsonpath='{.spec.template.spec.containers[0].image}'" 'image-registry.openshift-image-registry.svc:5000/cmd-set-image/ruby'

os::cmd::expect_success 'oc set image dc/test-deployment-config ruby-helloworld=ruby:2.5 --source=istag'
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o jsonpath='{.spec.template.spec.containers[0].image}'" 'image-registry.openshift-image-registry.svc:5000/cmd-set-image/ruby'

os::cmd::expect_failure 'oc set image dc/test-deployment-config ruby-helloworld=ruby:XYZ --source=istag'
os::cmd::expect_failure 'oc set image dc/test-deployment-config ruby-helloworld=ruby:XYZ --source=isimage'

os::cmd::expect_success 'oc set image dc/test-deployment-config ruby-helloworld=nginx'
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o jsonpath='{.spec.template.spec.containers[0].image}'" 'nginx'

os::cmd::expect_success 'oc set image pod/hello-openshift hello-openshift=nginx'
os::cmd::expect_success_and_text "oc get pod/hello-openshift -o jsonpath='{.spec.containers[0].image}'" 'nginx'

os::cmd::expect_success 'oc set image pod/hello-openshift hello-openshift=nginx:1.9.1'
os::cmd::expect_success_and_text "oc get pod/hello-openshift -o jsonpath='{.spec.containers[0].image}'" 'nginx:1.9.1'

os::cmd::expect_success 'oc set image pods,dc *=ruby:2.5 --all --source=imagestreamtag'
os::cmd::expect_success_and_text "oc get pod/hello-openshift -o jsonpath='{.spec.containers[0].image}'" 'image-registry.openshift-image-registry.svc:5000/cmd-set-image/ruby'
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o jsonpath='{.spec.template.spec.containers[0].image}'" 'image-registry.openshift-image-registry.svc:5000/cmd-set-image/ruby'

echo "set-image: ok"
os::test::junit::declare_suite_end
