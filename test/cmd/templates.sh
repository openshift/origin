#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  oc delete template/ruby-helloworld-sample -n openshift
  oc delete project test-template-project
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/templates"
# This test validates template commands

os::test::junit::declare_suite_start "cmd/templates/basic"
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
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/templates/config"
os::cmd::expect_success 'oc process -f test/templates/testdata/guestbook.json -l app=guestbook | oc create -f -'
os::cmd::expect_success_and_text 'oc status' 'frontend-service'
echo "template+config: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/templates/parameters"
# Individually specified parameter values are honored
os::cmd::expect_success_and_text 'oc process -f test/templates/testdata/guestbook.json -v ADMIN_USERNAME=myuser -v ADMIN_PASSWORD=mypassword' '"myuser"'
os::cmd::expect_success_and_text 'oc process -f test/templates/testdata/guestbook.json -v ADMIN_USERNAME=myuser -v ADMIN_PASSWORD=mypassword' '"mypassword"'
# Argument values are honored
os::cmd::expect_success_and_text 'oc process ADMIN_USERNAME=myuser ADMIN_PASSWORD=mypassword -f test/templates/testdata/guestbook.json'       '"myuser"'
os::cmd::expect_success_and_text 'oc process -f test/templates/testdata/guestbook.json ADMIN_USERNAME=myuser ADMIN_PASSWORD=mypassword'       '"mypassword"'
# Argument values with commas are honored
os::cmd::expect_success 'oc create -f examples/sample-app/application-template-stibuild.json'
os::cmd::expect_success_and_text 'oc process ruby-helloworld-sample MYSQL_USER=myself MYSQL_PASSWORD=my,1%pa=s'        '"myself"'
os::cmd::expect_success_and_text 'oc process MYSQL_USER=myself MYSQL_PASSWORD=my,1%pa=s ruby-helloworld-sample'        '"my,1%pa=s"'
os::cmd::expect_success_and_text 'oc process ruby-helloworld-sample -v MYSQL_USER=myself -v MYSQL_PASSWORD=my,1%pa=s'  '"myself"'
os::cmd::expect_success_and_text 'oc process -v MYSQL_USER=myself -v MYSQL_PASSWORD=my,1%pa=s ruby-helloworld-sample'  '"my,1%pa=s"'
os::cmd::expect_success 'oc delete template ruby-helloworld-sample'
echo "template+parameters: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/templates/data-precision"
# Run as cluster-admin to allow choosing any supplemental groups we want
# Ensure large integers survive unstructured JSON creation
os::cmd::expect_success 'oc create -f test/testdata/template-type-precision.json'
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
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/templates/different-namespaces"
os::cmd::expect_success 'oc create -f examples/sample-app/application-template-dockerbuild.json -n openshift'
os::cmd::expect_success 'oc policy add-role-to-user admin test-user'
new="$(mktemp -d)/tempconfig"
os::cmd::expect_success "oc config view --raw > ${new}"
export KUBECONFIG=${new}
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
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end