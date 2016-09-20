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
os::cmd::expect_success 'oc create -f test/integration/testdata/test-deployment-config.yaml'
os::cmd::expect_success 'oc create -f examples/hello-openshift/hello-pod.json'

os::cmd::expect_success 'oc set image dc/test-deployment-config ruby-helloworld=openshift/ruby:2.3'
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o jsonpath='{.spec.template.spec.containers[0].image}'" 'centos/ruby-23-centos7@sha256'

os::cmd::expect_success 'oc set image dc/test-deployment-config ruby-helloworld=openshift/ruby:2.0'
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o jsonpath='{.spec.template.spec.containers[0].image}'" 'openshift/ruby-20-centos7@sha256'

os::cmd::expect_success 'oc set image dc/test-deployment-config ruby-helloworld=openshift/ruby:2.0'
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o jsonpath='{.spec.template.spec.containers[0].image}'" 'openshift/ruby-20-centos7@sha256'

os::cmd::expect_failure 'oc set image dc/test-deployment-config ruby-helloworld=openshift/ruby:XYZ'
os::cmd::expect_failure 'oc set image dc/test-deployment-config ruby-helloworld=openshift/ruby:XYZ --source=isimage'

os::cmd::expect_success 'oc set image dc/test-deployment-config ruby-helloworld=nginx --source=docker'
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o jsonpath='{.spec.template.spec.containers[0].image}'" 'nginx'

os::cmd::expect_success 'oc set image pod/hello-openshift hello-openshift=nginx --source=docker'
os::cmd::expect_success_and_text "oc get pod/hello-openshift -o jsonpath='{.spec.containers[0].image}'" 'nginx'

os::cmd::expect_success 'oc set image pod/hello-openshift hello-openshift=nginx:1.9.1 --source=docker'
os::cmd::expect_success_and_text "oc get pod/hello-openshift -o jsonpath='{.spec.containers[0].image}'" 'nginx:1.9.1'

os::cmd::expect_success 'oc set image pods,dc *=openshift/ruby:2.3 --all'
os::cmd::expect_success_and_text "oc get pod/hello-openshift -o jsonpath='{.spec.containers[0].image}'" 'centos/ruby-23-centos7@sha256'
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o jsonpath='{.spec.template.spec.containers[0].image}'" 'centos/ruby-23-centos7@sha256'

echo "set-image: ok"
os::test::junit::declare_suite_end
