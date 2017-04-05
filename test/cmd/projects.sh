#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/projects"

os::test::junit::declare_suite_start "cmd/projects/lifecycle"
# resourceaccessreview
os::cmd::expect_success 'oc policy who-can get pods -n missing-ns'
# selfsubjectaccessreview
os::cmd::expect_success 'oc policy can-i get pods -n missing-ns'
# selfsubjectrulesreivew
os::cmd::expect_success 'oc policy can-i --list -n missing-ns'
# subjectaccessreview
os::cmd::expect_success 'oc policy can-i get pods --user=bob -n missing-ns'
# subjectrulesreview
os::cmd::expect_success 'oc policy can-i --list  --user=bob -n missing-ns'
echo 'project lifecycle ok'
os::test::junit::declare_suite_end

os::cmd::expect_failure_and_text 'oc projects test_arg' 'no arguments'
# log in as a test user and expect no projects
os::cmd::expect_success 'oc login -u test -p test'
os::cmd::expect_success_and_text 'oc projects' 'You are not a member of any projects'
# add a project and expect text for a single project
os::cmd::expect_success_and_text 'oc new-project test4' 'Now using project "test4" on server '
os::cmd::try_until_text 'oc projects' 'You have one project on this server: "test4".'
os::cmd::expect_success_and_text 'oc new-project test5' 'Now using project "test5" on server '
os::cmd::try_until_text 'oc projects' 'You have access to the following projects and can switch between them with '
os::cmd::expect_success_and_text 'oc projects' 'test4'
os::cmd::expect_success_and_text 'oc projects' 'test5'
# test --skip-config-write
os::cmd::expect_success_and_text 'oc new-project test6 --skip-config-write' 'To switch to this project and start adding applications, use'
os::cmd::expect_success_and_not_text 'oc config view -o jsonpath="{.contexts[*].context.namespace}"' '\btest6\b'
os::cmd::try_until_text 'oc projects' 'test6'
os::cmd::expect_success_and_text 'oc project test6' 'Now using project "test6"'
os::cmd::expect_success_and_text 'oc config view -o jsonpath="{.contexts[*].context.namespace}"' '\btest6\b'

# test if namespace is updated when the user tries to edit immutable fields of a project but has rights to edit the namespace
os::test::junit::declare_suite_start "cmd/projects/namespace_update"
os::cmd::expect_success_and_text 'oc login -u nstest -p nstest' 'Login successful.' # login standard user
os::cmd::expect_success_and_text 'oc new-project nstestproj' 'Now using project "nstestproj" on server ' # make project
# test that standard user cannot edit immutable fields
os::cmd::expect_failure_and_text 'oc annotate project nstestproj key1=val1 --overwrite' 'The Project "nstestproj" is invalid: metadata.annotations\[key1\]: Invalid value: "val1": field is immutable, try updating the namespace'
os::cmd::expect_failure_and_text 'oc annotate namespace nstestproj key2=val2 --overwrite' 'Error from server \(Forbidden\): User "nstest" cannot "patch" "namespaces" with name "nstestproj" in project "nstestproj"'
os::cmd::expect_success_and_text 'oc login -u system:admin -n default' 'Login successful.|system:admin' # login as admin
# test that admin user can edit all fields regardless of which endpoint they go through
os::cmd::expect_success_and_text 'oc annotate project nstestproj key3=val3 --overwrite' 'project "nstestproj" annotated'
os::cmd::expect_success_and_text 'oc annotate namespace nstestproj key4=val4 --overwrite' 'namespace "nstestproj" annotated'
os::cmd::expect_success_and_text 'oc login -u nstest -p nstest' 'Login successful.' # login standard user again
# test that standard user still cannot edit immutable fields
os::cmd::expect_failure_and_text 'oc annotate project nstestproj key1=val1 --overwrite' 'The Project "nstestproj" is invalid: metadata.annotations\[key1\]: Invalid value: "val1": field is immutable, try updating the namespace'
os::cmd::expect_failure_and_text 'oc annotate namespace nstestproj key2=val2 --overwrite' 'Error from server \(Forbidden\): User "nstest" cannot "patch" "namespaces" with name "nstestproj" in project "nstestproj"'
# this will now be successful because there is no diff between the old and new project
os::cmd::expect_success_and_text 'oc annotate project nstestproj key3=val3 --overwrite' 'project "nstestproj" annotated'
# this fails early at the authorization stage even though there is no diff
os::cmd::expect_failure_and_text 'oc annotate namespace nstestproj key4=val4 --overwrite' 'Error from server \(Forbidden\): User "nstest" cannot "patch" "namespaces" with name "nstestproj" in project "nstestproj"'
# same tests as above but with labels instead of annotations
os::cmd::expect_failure_and_text 'oc label project nstestproj keyA=valA --overwrite' 'The Project "nstestproj" is invalid: metadata.labels\[keyA\]: Invalid value: "valA": field is immutable, try updating the namespace'
os::cmd::expect_failure_and_text 'oc label namespace nstestproj keyB=valB --overwrite' 'Error from server \(Forbidden\): User "nstest" cannot "patch" "namespaces" with name "nstestproj" in project "nstestproj"'
os::cmd::expect_success_and_text 'oc login -u system:admin -n default' 'Login successful.|system:admin'
os::cmd::expect_success_and_text 'oc label project nstestproj keyC=valC --overwrite' 'project "nstestproj" labeled'
os::cmd::expect_success_and_text 'oc label namespace nstestproj keyD=valD --overwrite' 'namespace "nstestproj" labeled'
os::cmd::expect_success_and_text 'oc login -u nstest -p nstest' 'Login successful.'
os::cmd::expect_failure_and_text 'oc label project nstestproj keyA=valA --overwrite' 'The Project "nstestproj" is invalid: metadata.labels\[keyA\]: Invalid value: "valA": field is immutable, try updating the namespace'
os::cmd::expect_failure_and_text 'oc label namespace nstestproj keyB=valB --overwrite' 'Error from server \(Forbidden\): User "nstest" cannot "patch" "namespaces" with name "nstestproj" in project "nstestproj"'
os::cmd::expect_success_and_text 'oc label project nstestproj keyC=valC --overwrite' 'project "nstestproj" not labeled'
os::cmd::expect_failure_and_text 'oc label namespace nstestproj keyD=valD --overwrite' 'Error from server \(Forbidden\): User "nstest" cannot "patch" "namespaces" with name "nstestproj" in project "nstestproj"'
os::test::junit::declare_suite_end

echo 'projects command ok'

os::test::junit::declare_suite_end
