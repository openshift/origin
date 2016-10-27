#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oadm policy remove-scc-from-user privileged -z router
  oc delete sa/router -n default
  exit 0
) &>/dev/null

defaultimage="openshift/origin-\${component}:latest"
USE_IMAGES=${USE_IMAGES:-$defaultimage}

os::test::junit::declare_suite_start "cmd/router"
# Test running a router
os::cmd::expect_failure_and_text 'oadm router --dry-run' 'does not exist'
os::cmd::expect_failure_and_text 'oadm router --dry-run -o yaml' 'service account "router" is not allowed to access the host network on nodes'
os::cmd::expect_failure_and_text 'oadm router --dry-run -o yaml' 'name: router'
os::cmd::expect_failure_and_text 'oadm router --dry-run --stats-port=1937 -o yaml' 'containerPort: 1937'
os::cmd::expect_failure_and_text 'oadm router --dry-run --host-network=false -o yaml' 'service account "router" is not allowed to access host ports on nodes'
os::cmd::expect_failure_and_text 'oadm router --dry-run --host-network=false -o yaml' 'hostPort: 1936'
os::cmd::expect_success_and_not_text 'oadm router --dry-run --host-network=false --host-ports=false -o yaml' 'hostPort: 1936'
os::cmd::expect_failure_and_text 'oadm router --dry-run --host-network=false --stats-port=1937 -o yaml' 'hostPort: 1937'
os::cmd::expect_failure_and_text 'oadm router --dry-run --service-account=other -o yaml' 'service account "other" is not allowed to access the host network on nodes'
os::cmd::expect_failure_and_not_text 'oadm router --dry-run --host-network=false -o yaml --credentials=${KUBECONFIG}' 'ServiceAccount'
# set ports internally
os::cmd::expect_failure_and_text 'oadm router --dry-run --host-network=false -o yaml' 'containerPort: 80'
os::cmd::expect_failure_and_text 'oadm router --dry-run --host-network=false --ports=80:8080 -o yaml' 'port: 8080'
os::cmd::expect_failure_and_text 'oadm router --dry-run --host-network=false --ports=80,8443:443 -o yaml' 'targetPort: 8443'
os::cmd::expect_failure_and_text 'oadm router --dry-run --host-network=false -o yaml' 'hostPort'
os::cmd::expect_success_and_not_text 'oadm router --dry-run --host-network=false --host-ports=false -o yaml' 'hostPort'
# don't use localhost for liveness probe by default
os::cmd::expect_success_and_not_text "oadm router --dry-run --host-network=false --host-ports=false -o yaml" 'host: localhost'
# client env vars are optional
os::cmd::expect_success_and_not_text 'oadm router --dry-run --host-network=false --host-ports=false -o yaml' 'OPENSHIFT_MASTER'
os::cmd::expect_success_and_not_text 'oadm router --dry-run --host-network=false --host-ports=false --secrets-as-env -o yaml' 'OPENSHIFT_MASTER'
os::cmd::expect_success_and_text 'oadm router --dry-run --host-network=false --host-ports=false --secrets-as-env --credentials=${KUBECONFIG} -o yaml' 'OPENSHIFT_MASTER'
# mount tls crt as secret
os::cmd::expect_success_and_not_text 'oadm router --dry-run --host-network=false --host-ports=false -o yaml' 'value: /etc/pki/tls/private/tls.crt'
os::cmd::expect_failure_and_text "oadm router --dry-run --host-network=false --host-ports=false --default-cert=${KUBECONFIG} -o yaml" 'the default cert must contain a private key'
os::cmd::expect_success_and_text "oadm router --dry-run --host-network=false --host-ports=false --default-cert=images/router/haproxy-base/conf/default_pub_keys.pem -o yaml" 'value: /etc/pki/tls/private/tls.crt'
os::cmd::expect_success_and_text "oadm router --dry-run --host-network=false --host-ports=false --default-cert=images/router/haproxy-base/conf/default_pub_keys.pem -o yaml" 'tls.key:'
os::cmd::expect_success_and_text "oadm router --dry-run --host-network=false --host-ports=false --default-cert=images/router/haproxy-base/conf/default_pub_keys.pem -o yaml" 'tls.crt: '
os::cmd::expect_success_and_text "oadm router --dry-run --host-network=false --host-ports=false --default-cert=images/router/haproxy-base/conf/default_pub_keys.pem -o yaml" 'type: kubernetes.io/tls'
# upgrade the router to have access to host networks
os::cmd::expect_success "oadm policy add-scc-to-user privileged -z router"
# uses localhost for probes
os::cmd::expect_success_and_text "oadm router --dry-run -o yaml" 'host: localhost'
os::cmd::expect_success_and_text "oadm router --dry-run --host-network=false -o yaml" 'hostPort'
os::cmd::expect_failure_and_text "oadm router --ports=80,8443:443" 'container port 8443 and host port 443 must be equal'

os::cmd::expect_success_and_text "oadm router -o yaml --credentials=${KUBECONFIG}" 'image:.*-haproxy-router:'
os::cmd::expect_success "oadm router --credentials=${KUBECONFIG} --images='${USE_IMAGES}'"
os::cmd::expect_success_and_text 'oadm router' 'service exists'
os::cmd::expect_success_and_text 'oc get dc/router -o yaml' 'readinessProbe'

# only when using hostnetwork should we force the probes to use localhost
os::cmd::expect_success_and_not_text "oadm router -o yaml --credentials=${KUBECONFIG} --host-network=false" 'host: localhost'
os::cmd::expect_success "oc delete dc/router"
os::cmd::expect_success "oc delete service router"
echo "router: ok"

# test ipfailover
os::cmd::expect_failure_and_text 'oadm ipfailover --dry-run' 'you must specify at least one virtual IP address'
os::cmd::expect_failure_and_text 'oadm ipfailover --virtual-ips="1.2.3.4" --dry-run' 'error: ipfailover could not be created'
os::cmd::expect_success 'oadm policy add-scc-to-user privileged -z ipfailover'
os::cmd::expect_success_and_text 'oadm ipfailover --virtual-ips="1.2.3.4" --dry-run' 'Creating IP failover'
os::cmd::expect_success_and_text 'oadm ipfailover --virtual-ips="1.2.3.4" --dry-run' 'Success \(dry run\)'
os::cmd::expect_success_and_text 'oadm ipfailover --virtual-ips="1.2.3.4" --dry-run -o yaml' 'name: ipfailover'
os::cmd::expect_success_and_text 'oadm ipfailover --virtual-ips="1.2.3.4" --dry-run -o name' 'deploymentconfig/ipfailover'
os::cmd::expect_success_and_text 'oadm ipfailover --virtual-ips="1.2.3.4" --dry-run -o yaml' '1.2.3.4'
os::cmd::expect_success_and_text 'oadm ipfailover --virtual-ips="1.2.3.4" --iptables-chain="MY_CHAIN" --dry-run -o yaml' 'value: MY_CHAIN'
os::cmd::expect_success 'oadm policy remove-scc-from-user privileged -z ipfailover'
# TODO add tests for normal ipfailover creation
# os::cmd::expect_success_and_text 'oadm ipfailover' 'deploymentconfig "ipfailover" created'
# os::cmd::expect_failure_and_text 'oadm ipfailover' 'Error from server: deploymentconfig "ipfailover" already exists'
# os::cmd::expect_success_and_text 'oadm ipfailover -o name --dry-run | xargs oc delete' 'deleted'
echo "ipfailover: ok"

os::test::junit::declare_suite_end
