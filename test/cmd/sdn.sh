#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete namespace sdn-test
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
os::cmd::expect_success 'oc new-project sdn-test'
os::cmd::expect_success 'oc get namespace sdn-test'
os::cmd::try_until_success 'oc get netnamespace sdn-test'
os::cmd::expect_success_and_not_text 'oc get netnamespace sdn-test -o jsonpath="{.netid}"' '^0$'

os::cmd::expect_success "oc project '${orig_project}'"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/sdn/hostsubnets"
# test-cmd environment has no nodes, hence no hostsubnets
os::cmd::expect_success_and_not_text 'oc get hostsubnets' '.'
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/sdn/egressnetworkpolicies"
os::cmd::expect_success 'oc get egressnetworkpolicies'
os::cmd::expect_success 'oc create -f test/integration/testdata/test-egress-network-policy.json'
os::cmd::expect_success 'oc get egressnetworkpolicy default'
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
