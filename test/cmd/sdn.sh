#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete namespace sdn-test-1
  oc delete namespace sdn-test-2
  oc delete namespace sdn-test-3
  oc delete egressnetworkpolicy --all
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/sdn"

os::test::junit::declare_suite_start "cmd/sdn/clusternetworks"
os::cmd::expect_success 'oc get clusternetworks'
# Sanity check that the environment is as expected, or the NetNamespace tests will fail
os::cmd::expect_success_and_text 'oc get clusternetwork default -o jsonpath="{.pluginName}"' 'redhat/openshift-ovs-multitenant'
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/sdn/netnamespaces"
orig_project="$(oc project -q)"

os::cmd::expect_success 'oc get netnamespaces'
os::cmd::expect_success_and_text 'oc get netnamespace default -o jsonpath="{.netid}"' '^0$'

os::cmd::expect_success 'oc new-project sdn-test-1'
os::cmd::expect_success 'oc new-project sdn-test-2'
os::cmd::expect_success 'oc new-project sdn-test-3'
os::cmd::try_until_success 'oc get netnamespace sdn-test-1'
os::cmd::expect_success_and_not_text 'oc get netnamespace sdn-test-1 -o jsonpath="{.netid}"' '^0$'
orig_vnid1="$(oc get netnamespace sdn-test-1 -o jsonpath='{.netid}')"
os::cmd::try_until_success 'oc get netnamespace sdn-test-2'
os::cmd::try_until_success 'oc get netnamespace sdn-test-3'

os::cmd::expect_success 'oc adm pod-network join-projects --to=sdn-test-1 sdn-test-2'
os::cmd::expect_success_and_text 'oc get netnamespace sdn-test-2 -o jsonpath="{.netid}"' "^${orig_vnid1}\$"

os::cmd::expect_success 'oc adm pod-network make-projects-global sdn-test-1'
os::cmd::expect_success_and_text 'oc get netnamespace sdn-test-1 -o jsonpath="{.netid}"' '^0$'

os::cmd::expect_success 'oc adm pod-network isolate-projects sdn-test-1'
os::cmd::expect_success_and_not_text 'oc get netnamespace sdn-test-1 -o jsonpath="{.netid}"' '^0$'

os::cmd::expect_success "oc project '${orig_project}'"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/sdn/hostsubnets"
# test-cmd environment has no nodes, hence no hostsubnets
os::cmd::expect_success_and_text 'oc get hostsubnets' 'No resources found.'
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/sdn/egressnetworkpolicies"
os::cmd::expect_success 'oc get egressnetworkpolicies'
os::cmd::expect_success 'oc create -f test/integration/testdata/test-egress-network-policy.json'
os::cmd::expect_success 'oc get egressnetworkpolicy default'
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
