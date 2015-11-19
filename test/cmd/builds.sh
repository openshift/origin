#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

url=":${API_PORT:-8443}"
project="$(oc project -q)"

# This test validates builds and build related commands

oc new-build openshift/ruby-20-centos7 https://github.com/openshift/ruby-hello-world.git
oc get bc/ruby-hello-world
cat "${OS_ROOT}/Dockerfile" | oc new-build -D - --name=test
oc get bc/test
oc new-build --dockerfile=$'FROM centos:7\nRUN yum install -y httpd'
oc get bc/centos
oc delete all --all

oc process -f examples/sample-app/application-template-dockerbuild.json -l build=docker | oc create -f -
oc get buildConfigs
oc get bc
oc get builds

REAL_OUTPUT_TO=$(oc get bc/ruby-sample-build --template='{{ .spec.output.to.name }}')
oc patch bc/ruby-sample-build -p '{"spec":{"output":{"to":{"name":"different:tag1"}}}}'
oc get bc/ruby-sample-build --template='{{ .spec.output.to.name }}' | grep 'different'
oc patch bc/ruby-sample-build -p "{\"spec\":{\"output\":{\"to\":{\"name\":\"${REAL_OUTPUT_TO}\"}}}}"
echo "patchAnonFields: ok"

[ "$(oc describe buildConfigs ruby-sample-build | grep --text "Webhook GitHub" | grep -F "${url}/oapi/v1/namespaces/${project}/buildconfigs/ruby-sample-build/webhooks/secret101/github")" ]
[ "$(oc describe buildConfigs ruby-sample-build | grep --text "Webhook Generic" | grep -F "${url}/oapi/v1/namespaces/${project}/buildconfigs/ruby-sample-build/webhooks/secret101/generic")" ]
oc start-build --list-webhooks='all' ruby-sample-build
[ "$(oc start-build --list-webhooks='all' bc/ruby-sample-build | grep --text "generic")" ]
[ "$(oc start-build --list-webhooks='all' ruby-sample-build | grep --text "github")" ]
[ "$(oc start-build --list-webhooks='github' ruby-sample-build | grep --text "secret101")" ]
[ ! "$(oc start-build --list-webhooks='blah')" ]
webhook=$(oc start-build --list-webhooks='generic' ruby-sample-build --api-version=v1 | head -n 1)
oc start-build --from-webhook="${webhook}"
oc get builds
oc delete all -l build=docker
echo "buildConfig: ok"

oc create -f test/integration/fixtures/test-buildcli.json
# a build for which there is not an upstream tag in the corresponding imagerepo, so
# the build should use the image field as defined in the buildconfig
started=$(oc start-build ruby-sample-build-invalidtag)
oc describe build ${started} | grep openshift/ruby-20-centos7$
frombuild=$(oc start-build --from-build="${started}")
oc describe build ${frombuild} | grep openshift/ruby-20-centos7$
echo "start-build: ok"

oc cancel-build "${started}" --dump-logs --restart
oc delete all --all
oc process -f examples/sample-app/application-template-dockerbuild.json -l build=docker | oc create -f -
tryuntil oc get build/ruby-sample-build-1
# Uses type/name resource syntax
oc cancel-build build/ruby-sample-build-1
oc delete all --all
echo "cancel-build: ok"
