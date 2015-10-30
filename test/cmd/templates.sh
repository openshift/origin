#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# This test validates template commands

oc get templates
oc create -f examples/sample-app/application-template-dockerbuild.json 
oc get templates
oc get templates ruby-helloworld-sample
oc get template ruby-helloworld-sample -o json | oc process -f -
oc process ruby-helloworld-sample
oc describe templates ruby-helloworld-sample
[ "$(oc describe templates ruby-helloworld-sample | grep -E "BuildConfig.*ruby-sample-build")" ]
oc delete templates ruby-helloworld-sample
oc get templates
# TODO: create directly from template
echo "templates: ok"

oc process -f test/templates/fixtures/guestbook.json -l app=guestbook | oc create -f -
oc status
[ "$(oc status | grep frontend-service)" ]
echo "template+config: ok"

oc create -f examples/sample-app/application-template-dockerbuild.json -n openshift
oc policy add-role-to-user admin test-user
oc login -u test-user -p password
oc new-project test-template-project
oc create -f examples/sample-app/application-template-dockerbuild.json
oc process template/ruby-helloworld-sample >/dev/null
oc process templates/ruby-helloworld-sample > /dev/null
oc process openshift//ruby-helloworld-sample > /dev/null
oc process openshift/template/ruby-helloworld-sample >/dev/null
echo "processing templates in different namespace: ok"
