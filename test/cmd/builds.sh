#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

url=":${API_PORT:-8443}"
project="$(oc project -q)"

# This test validates builds and build related commands

os::cmd::expect_success 'oc new-build openshift/ruby-20-centos7 https://github.com/openshift/ruby-hello-world.git'
os::cmd::expect_success 'oc get bc/ruby-hello-world'
os::cmd::expect_success 'cat "${OS_ROOT}/Dockerfile" | oc new-build -D - --name=test'
os::cmd::expect_success 'oc get bc/test'
os::cmd::expect_success "oc new-build --dockerfile=\$'FROM centos:7\nRUN yum install -y httpd'"
os::cmd::expect_success 'oc get bc/centos'
os::cmd::expect_success 'oc delete all --all'

os::cmd::expect_success 'oc process -f examples/sample-app/application-template-dockerbuild.json -l build=docker | oc create -f -'
os::cmd::expect_success 'oc get buildConfigs'
os::cmd::expect_success 'oc get bc'
os::cmd::expect_success 'oc get builds'

# make sure the imagestream has the latest tag before starting a build or the build will immediately fail.
os::cmd::expect_success "tryuntil 'oc get is ruby-20-centos7 | grep latest'"

REAL_OUTPUT_TO=$(oc get bc/ruby-sample-build --template='{{ .spec.output.to.name }}')
os::cmd::expect_success "oc patch bc/ruby-sample-build -p '{\"spec\":{\"output\":{\"to\":{\"name\":\"different:tag1\"}}}}'"
os::cmd::expect_success "oc get bc/ruby-sample-build --template='{{ .spec.output.to.name }}' | grep 'different'"
os::cmd::expect_success "oc patch bc/ruby-sample-build -p '{\"spec\":{\"output\":{\"to\":{\"name\":\"${REAL_OUTPUT_TO}\"}}}}'"
echo "patchAnonFields: ok"

os::cmd::expect_success_and_text 'oc describe buildConfigs ruby-sample-build' "Webhook GitHub.+${url}/oapi/v1/namespaces/${project}/buildconfigs/ruby-sample-build/webhooks/secret101/github"
os::cmd::expect_success_and_text 'oc describe buildConfigs ruby-sample-build' "Webhook Generic.+${url}/oapi/v1/namespaces/${project}/buildconfigs/ruby-sample-build/webhooks/secret101/generic"
os::cmd::expect_success 'oc start-build --list-webhooks='all' ruby-sample-build'
os::cmd::expect_success_and_text 'oc start-build --list-webhooks=all bc/ruby-sample-build' 'generic'
os::cmd::expect_success_and_text 'oc start-build --list-webhooks=all ruby-sample-build' 'github'
os::cmd::expect_success_and_text 'oc start-build --list-webhooks=github ruby-sample-build' 'secret101'
os::cmd::expect_failure 'oc start-build --list-webhooks=blah'
webhook=$(oc start-build --list-webhooks='generic' ruby-sample-build --api-version=v1 | head -n 1)
os::cmd::expect_success "oc start-build --from-webhook=${webhook}" 
os::cmd::expect_success 'oc get builds'
os::cmd::expect_success 'oc delete all -l build=docker'
echo "buildConfig: ok"

os::cmd::expect_success 'oc create -f test/integration/fixtures/test-buildcli.json'
# a build for which there is not an upstream tag in the corresponding imagerepo, so
# the build should use the image field as defined in the buildconfig
started=$(oc start-build ruby-sample-build-invalidtag)
os::cmd::expect_success_and_text "oc describe build ${started}" 'openshift/ruby-20-centos7$'
frombuild=$(oc start-build --from-build="${started}")
os::cmd::expect_success_and_text "oc describe build ${frombuild}" 'openshift/ruby-20-centos7$'
echo "start-build: ok"

os::cmd::expect_success "oc cancel-build ${started} --dump-logs --restart"
os::cmd::expect_success 'oc delete all --all'
os::cmd::expect_success 'oc process -f examples/sample-app/application-template-dockerbuild.json -l build=docker | oc create -f -'
os::cmd::expect_success "tryuntil 'oc get build/ruby-sample-build-1'"
# Uses type/name resource syntax
os::cmd::expect_success 'oc cancel-build build/ruby-sample-build-1'
os::cmd::expect_success 'oc delete all --all'
echo "cancel-build: ok"
