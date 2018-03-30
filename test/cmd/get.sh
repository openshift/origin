#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/get"
os::cmd::expect_success_and_text 'oc create service loadbalancer testsvc1  --tcp=8080' "service \"testsvc1\" created"
# mixed resource output should print resource kind
# prefix even when only one type of resource is present
os::cmd::expect_success_and_text 'oc get all' "svc/testsvc1"
# ensure that getting mixed resource types still returns prefixed resources, if there are at most resources of one type
os::cmd::expect_success_and_text 'oc get svc,pod' "svc/testsvc1"
os::cmd::expect_failure_and_text 'oc get svc,pod testsvc1' "svc/testsvc1"
# create second resource type and ensure that prefixed resource names are returned for both
os::cmd::expect_success_and_text 'oc create imagestream testimg1' "imagestream \"testimg1\" created"
os::cmd::expect_success_and_text 'oc get svc,is' "svc/testsvc1"
# create second service and expect `get all` to still append resource kind to multiple of one type of resource
os::cmd::expect_success_and_text 'oc create service loadbalancer testsvc2  --tcp=8081' "service \"testsvc2\" created"
os::cmd::expect_success_and_text 'oc get all' "svc/testsvc2"
# test tuples of same and different resource kinds (tuples of same resource kind should not return prefixed items).
os::cmd::expect_success_and_not_text 'oc get svc/testsvc1 svc/testsvc2' "svc/testsvc1"
os::cmd::expect_success_and_text 'oc get svc/testsvc1 is/testimg1' "svc/testsvc1"
os::cmd::expect_success_and_text 'oc get --v=8 svc/testsvc1 is/testimg1' "round_trippers.go"
# specific resources should not have their kind prefixed
os::cmd::expect_success_and_text 'oc get svc' "testsvc1"
# test --show-labels displays labels for users
os::cmd::expect_success 'oc create user test-user-1'
os::cmd::expect_success 'oc label user/test-user-1 customlabel=true'
os::cmd::expect_success_and_text 'oc get users test-user-1 --show-labels' "customlabel=true"
os::cmd::expect_success_and_not_text 'oc get users test-user-1' "customlabel=true"
# test structured and unstructured resources print generically without panic
os::cmd::expect_success_and_text 'oc get projectrequests -o yaml' 'status: Success'
os::cmd::expect_success_and_text 'oc get projectrequests,svc,pod -o yaml' 'kind: List'
# test --wacth does not result in an error when a resource list is served in multiple chunks
os::cmd::expect_success 'oc create cm cmone'
os::cmd::expect_success 'oc create cm cmtwo'
os::cmd::expect_success 'oc create cm cmthree'
os::cmd::expect_success_and_not_text 'oc get configmap --chunk-size=1 --watch --request-timeout=1s' 'watch is only supported on individual resources'
os::cmd::expect_success_and_not_text 'oc get configmap --chunk-size=1 --watch-only --request-timeout=1s' 'watch is only supported on individual resources'
echo "oc get: ok"
os::test::junit::declare_suite_end
