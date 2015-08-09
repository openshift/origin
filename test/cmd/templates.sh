#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

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
