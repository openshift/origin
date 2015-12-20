#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

# This test validates template commands

os::cmd::expect_success 'oc get templates'
os::cmd::expect_success 'oc create -f examples/sample-app/application-template-dockerbuild.json'
os::cmd::expect_success 'oc get templates'
os::cmd::expect_success 'oc get templates ruby-helloworld-sample'
os::cmd::expect_success 'oc get template ruby-helloworld-sample -o json | oc process -f -'
os::cmd::expect_success 'oc process ruby-helloworld-sample'
os::cmd::expect_success_and_text 'oc describe templates ruby-helloworld-sample' "BuildConfig.*ruby-sample-build"
os::cmd::expect_success 'oc delete templates ruby-helloworld-sample'
os::cmd::expect_success 'oc get templates'
# TODO: create directly from template
echo "templates: ok"

os::cmd::expect_success 'oc process -f test/templates/fixtures/guestbook.json -l app=guestbook | oc create -f -'
os::cmd::expect_success_and_text 'oc status' 'frontend-service'
echo "template+config: ok"

# Joined parameter values are honored
os::cmd::expect_success_and_text 'oc process -f test/templates/fixtures/guestbook.json -v ADMIN_USERNAME=myuser,ADMIN_PASSWORD=mypassword'    '"myuser"'
os::cmd::expect_success_and_text 'oc process -f test/templates/fixtures/guestbook.json -v ADMIN_USERNAME=myuser,ADMIN_PASSWORD=mypassword'    '"mypassword"'
# Individually specified parameter values are honored
os::cmd::expect_success_and_text 'oc process -f test/templates/fixtures/guestbook.json -v ADMIN_USERNAME=myuser -v ADMIN_PASSWORD=mypassword' '"myuser"'
os::cmd::expect_success_and_text 'oc process -f test/templates/fixtures/guestbook.json -v ADMIN_USERNAME=myuser -v ADMIN_PASSWORD=mypassword' '"mypassword"'
echo "template+parameters: ok"

# Run as cluster-admin to allow choosing any supplemental groups we want
# Ensure large integers survive unstructured JSON creation
os::cmd::expect_success 'oc create -f test/fixtures/template-type-precision.json'
# ... and processing
os::cmd::expect_success_and_text 'oc process template-type-precision' '1000030003'
os::cmd::expect_success_and_text 'oc process template-type-precision' '2147483647'
os::cmd::expect_success_and_text 'oc process template-type-precision' '9223372036854775807'
# ... and re-encoding as structured resources
os::cmd::expect_success 'oc process template-type-precision | oc create -f -'
# ... and persisting
os::cmd::expect_success_and_text 'oc get pod/template-type-precision -o json' '1000030003'
os::cmd::expect_success_and_text 'oc get pod/template-type-precision -o json' '2147483647'
os::cmd::expect_success_and_text 'oc get pod/template-type-precision -o json' '9223372036854775807'
# Ensure patch computation preserves data
patch='{"metadata":{"annotations":{"comment":"patch comment"}}}'
os::cmd::expect_success "oc patch pod template-type-precision -p '${patch}'"
os::cmd::expect_success_and_text 'oc get pod/template-type-precision -o json' '9223372036854775807'
os::cmd::expect_success_and_text 'oc get pod/template-type-precision -o json' 'patch comment'
os::cmd::expect_success 'oc delete template/template-type-precision'
os::cmd::expect_success 'oc delete pod/template-type-precision'
echo "template data precision: ok"


os::cmd::expect_success 'oc create -f examples/sample-app/application-template-dockerbuild.json -n openshift'
os::cmd::expect_success 'oc policy add-role-to-user admin test-user'
os::cmd::expect_success 'oc login -u test-user -p password'
os::cmd::expect_success 'oc new-project test-template-project'
# make sure the permissions on the new project are set up
os::cmd::try_until_success 'oc get templates'
os::cmd::expect_success 'oc create -f examples/sample-app/application-template-dockerbuild.json'
os::cmd::expect_success 'oc process template/ruby-helloworld-sample'
os::cmd::expect_success 'oc process templates/ruby-helloworld-sample'
os::cmd::expect_success 'oc process openshift//ruby-helloworld-sample'
os::cmd::expect_success 'oc process openshift/template/ruby-helloworld-sample'
echo "processing templates in different namespace: ok"
