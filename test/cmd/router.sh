#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc adm policy remove-scc-from-user privileged -z router
  oc delete sa/router -n default
  exit 0
) &>/dev/null

defaultimage="openshift/origin-\${component}:latest"
USE_IMAGES=${USE_IMAGES:-$defaultimage}

# test ipfailover
os::cmd::expect_failure_and_text 'oc adm ipfailover --dry-run' 'service account "ipfailover" does not have sufficient privileges'
os::cmd::expect_failure_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --dry-run' 'error: ipfailover could not be created'
os::cmd::expect_success 'oc adm policy add-scc-to-user privileged -z ipfailover'
os::cmd::expect_failure_and_text 'oc adm ipfailover --dry-run' 'you must specify at least one virtual IP address'
os::cmd::expect_success_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --dry-run' 'serviceaccount/ipfailover created \(dry run\)'
os::cmd::expect_success_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --dry-run' 'deploymentconfig.apps.openshift.io/ipfailover created \(dry run\)'
os::cmd::expect_success_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --dry-run -o yaml' 'name: ipfailover'
os::cmd::expect_success_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --dry-run -o name' 'deploymentconfig.apps.openshift.io/ipfailover'
os::cmd::expect_success_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --dry-run -o yaml' '1.2.3.4'
os::cmd::expect_success_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --iptables-chain="MY_CHAIN" --dry-run -o yaml' 'value: MY_CHAIN'
os::cmd::expect_success_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --check-interval=1177 --dry-run -o yaml' 'value: "1177"'
os::cmd::expect_success_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --check-script="ChkScript.sh" --dry-run -o yaml' 'value: ChkScript.sh'
os::cmd::expect_success_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --notify-script="NotScript.sh" --dry-run -o yaml' 'value: NotScript.sh'
os::cmd::expect_success_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --preemption-strategy="nopreempt" --dry-run -o yaml' 'value: nopreempt'
os::cmd::expect_success_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --dry-run -o yaml --vrrp-id-offset=56' 'hostPort: 63056'
os::cmd::expect_failure_and_text 'oc adm ipfailover --virtual-ips="1.2.3.4" --dry-run -o yaml --vrrp-id-offset=255' 'error: The vrrp-id-offset must be in the range 0..254'
os::cmd::expect_success 'oc adm policy remove-scc-from-user privileged -z ipfailover'

# TODO add tests for normal ipfailover creation
# os::cmd::expect_success_and_text 'oc adm ipfailover' 'deploymentconfig.apps.openshift.io "ipfailover" created'
# os::cmd::expect_failure_and_text 'oc adm ipfailover' 'Error from server: deploymentconfig "ipfailover" already exists'
# os::cmd::expect_success_and_text 'oc adm ipfailover -o name --dry-run | xargs oc delete' 'deleted'
echo "ipfailover: ok"

os::test::junit::declare_suite_end
