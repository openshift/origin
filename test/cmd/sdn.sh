#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/sdn"

os::cmd::expect_success 'oc get clusternetworks'
os::cmd::expect_success_and_text 'oc get clusternetwork default -o jsonpath="{.pluginName}"' 'redhat/openshift-ovs-multitenant'
os::cmd::expect_failure_and_text 'oc patch clusternetwork default -p "{\"network\": \"1.0.0.0/8\"}"' 'Invalid value'
os::cmd::expect_failure_and_text 'oc patch clusternetwork default -p "{\"hostsubnetlength\": 22}"' 'Invalid value'
os::cmd::expect_failure_and_text 'oc patch clusternetwork default -p "{\"serviceNetwork\": \"1.0.0.0/8\"}"' 'Invalid value'

orig_project=$(oc project -q)

os::cmd::expect_success 'oc get netnamespaces'
os::cmd::expect_success_and_text 'oc get netnamespace default -o jsonpath="{.netid}"' '^0$'
os::cmd::expect_failure 'oc get netnamespace sdn-test'
os::cmd::expect_success 'oc new-project sdn-test'
os::cmd::expect_success 'oc get namespace sdn-test'
os::cmd::try_until_success 'oc get netnamespace sdn-test'
os::cmd::expect_success_and_not_text 'oc get netnamespace sdn-test -o jsonpath="{.netid}"' '^0$'
os::cmd::expect_success 'oc delete namespace sdn-test'
os::cmd::try_until_failure 'oc get netnamespace sdn-test'

os::cmd::expect_success 'oc project ${orig_project}'

# test-cmd environment has no nodes, hence no hostsubnets
os::cmd::expect_success_and_not_text 'oc get hostsubnets' '.'

policy='{"kind": "EgressNetworkPolicy", "metadata": {"name": "default"}, "spec": {"egress": [{"type": "Allow", "to": {"cidrSelector": "192.168.0.0/16"}}, {"type": "Deny", "to": {"cidrSelector": "0.0.0.0/0"}}]}}'
os::cmd::expect_success 'oc get egressnetworkpolicies'
os::cmd::expect_failure 'oc get egressnetworkpolicy default'
os::cmd::expect_success 'echo "${policy}" | oc create -f -'
os::cmd::expect_success 'oc get egressnetworkpolicy default'
os::cmd::expect_success 'oc delete egressnetworkpolicy default'

os::test::junit::declare_suite_end
