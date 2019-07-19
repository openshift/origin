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
echo 'projects command ok'


os::test::junit::declare_suite_end
