#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete route foo bar testroute test-route new-route
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/routes"

os::cmd::expect_success 'oc get routes'
os::cmd::expect_success 'oc create -f test/integration/testdata/test-route.json'
os::cmd::expect_success_and_text 'oc get routes testroute --show-labels' 'rtlabel1'
os::cmd::expect_success 'oc delete routes testroute'
os::cmd::expect_success 'oc create -f test/integration/testdata/test-service.json'
os::cmd::expect_success 'oc create route passthrough --service=svc/frontend'
os::cmd::expect_success 'oc delete routes frontend'
os::cmd::expect_success 'oc create route edge --path /test --service=services/non-existent --port=80'
os::cmd::expect_success 'oc delete routes non-existent'
os::cmd::expect_success 'oc create route edge test-route --service=frontend'
os::cmd::expect_success 'oc delete routes test-route'
os::cmd::expect_failure 'oc create route edge new-route'
os::cmd::expect_success 'oc delete services frontend'
os::cmd::expect_success 'oc create route edge --insecure-policy=Allow --service=foo --port=80'
os::cmd::expect_success_and_text 'oc get route foo -o jsonpath="{.spec.tls.insecureEdgeTerminationPolicy}"' 'Allow'
os::cmd::expect_success 'oc delete routes foo'

os::cmd::expect_success_and_text 'oc create route edge --service foo --port=8080' 'created'
os::cmd::expect_success_and_text 'oc create route edge --service bar --port=9090' 'created'

os::cmd::expect_success_and_text 'oc set route-backends foo' 'routes/foo'
os::cmd::expect_success_and_text 'oc set route-backends foo' 'Service'
os::cmd::expect_success_and_text 'oc set route-backends foo' '100'
os::cmd::expect_failure_and_text 'oc set route-backends foo --zero --equal' 'error: --zero and --equal may not be specified together'
os::cmd::expect_failure_and_text 'oc set route-backends foo --zero --adjust' 'error: --adjust and --zero may not be specified together'
os::cmd::expect_failure_and_text 'oc set route-backends foo a=' 'expected NAME=WEIGHT'
os::cmd::expect_failure_and_text 'oc set route-backends foo =10' 'expected NAME=WEIGHT'
os::cmd::expect_failure_and_text 'oc set route-backends foo a=a' 'WEIGHT must be a number'
os::cmd::expect_success_and_text 'oc set route-backends foo a=10' 'updated'
os::cmd::expect_success_and_text 'oc set route-backends foo a=100' 'updated'
os::cmd::expect_success_and_text 'oc set route-backends foo a=0' 'updated'
os::cmd::expect_success_and_text 'oc set route-backends foo' '0'
os::cmd::expect_success_and_text 'oc get routes foo' 'a'
os::cmd::expect_success_and_text 'oc set route-backends foo a1=0 b2=0' 'updated'
os::cmd::expect_success_and_text 'oc set route-backends foo' 'a1'
os::cmd::expect_success_and_text 'oc set route-backends foo' 'b2'
os::cmd::expect_success_and_text 'oc set route-backends foo a1=100 b2=50 c3=0' 'updated'
os::cmd::expect_success_and_text 'oc get routes foo' 'a1\(66%\),b2\(33%\),c3\(0%\)'
os::cmd::expect_success_and_text 'oc set route-backends foo a1=100 b2=0 c3=0' 'updated'
os::cmd::expect_success_and_text 'oc set route-backends foo --adjust b2=+10%' 'updated'
os::cmd::expect_success_and_text 'oc get routes foo' 'a1\(90%\),b2\(10%\),c3\(0%\)'
os::cmd::expect_success_and_text 'oc set route-backends foo --adjust b2=+25%' 'updated'
os::cmd::expect_success_and_text 'oc get routes foo' 'a1\(65%\),b2\(35%\),c3\(0%\)'
os::cmd::expect_success_and_text 'oc set route-backends foo --adjust b2=+99%' 'updated'
os::cmd::expect_success_and_text 'oc get routes foo' 'a1\(0%\),b2\(100%\),c3\(0%\)'
os::cmd::expect_success_and_text 'oc set route-backends foo --adjust b2=-51%' 'updated'
os::cmd::expect_success_and_text 'oc get routes foo' 'a1\(51%\),b2\(49%\),c3\(0%\)'
os::cmd::expect_success_and_text 'oc set route-backends foo --adjust a1=20%' 'updated'
os::cmd::expect_success_and_text 'oc get routes foo' 'a1\(20%\),b2\(80%\),c3\(0%\)'
os::cmd::expect_success_and_text 'oc set route-backends foo --adjust c3=50%' 'updated'
os::cmd::expect_success_and_text 'oc get routes foo' 'a1\(10%\),b2\(80%\),c3\(10%\)'
os::cmd::expect_success_and_text 'oc describe routes foo' '25 \(10%\)'
os::cmd::expect_success_and_text 'oc describe routes foo' '200 \(80%\)'
os::cmd::expect_success_and_text 'oc describe routes foo' '25 \(10%\)'
os::cmd::expect_success_and_text 'oc describe routes foo' '<error: endpoints "c3" not found>'
os::cmd::expect_success_and_text 'oc set route-backends foo --adjust c3=1' 'updated'
os::cmd::expect_success_and_text 'oc describe routes foo' '1 \(0%\)'
os::cmd::expect_success_and_text 'oc set route-backends foo --equal' 'updated'
os::cmd::expect_success_and_text 'oc get routes foo' 'a1\(33%\),b2\(33%\),c3\(33%\)'
os::cmd::expect_success_and_text 'oc describe routes foo' '100 \(33%\)'
os::cmd::expect_success_and_text 'oc set route-backends foo --zero' 'updated'
os::cmd::expect_success_and_text 'oc get routes foo' 'a1\(0%\),b2\(0%\),c3\(0%\)'
os::cmd::expect_success_and_text 'oc describe routes foo' '0'

os::test::junit::declare_suite_end
