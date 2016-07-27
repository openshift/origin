#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete node/fake-node
  exit 0
) &>/dev/null

defaultimage="openshift/origin-\${component}:latest"
USE_IMAGES=${USE_IMAGES:-$defaultimage}

os::test::junit::declare_suite_start "cmd/admin/registry"
os::cmd::expect_success "echo 'apiVersion: v1
kind: Node
metadata:
  labels:
      kubernetes.io/hostname: fake-node
  name: fake-node
spec:
  externalID: fake-node
status:
  conditions:
  - lastHeartbeatTime: 2015-09-08T16:58:02Z
    lastTransitionTime: 2015-09-04T11:49:06Z
    reason: kubelet is posting ready status
    status: \"True\"
    type: Ready
  allocatable:
    cpu: \"4\"
    memory: 8010948Ki
    pods: \"110\"
  capacity:
    cpu: \"4\"
    memory: 8010948Ki
    pods: \"110\"
' | oc create -f -"

# Test running a registry as a daemonset
os::cmd::expect_failure_and_text 'oadm registry --daemonset --dry-run' 'does not exist'
os::cmd::expect_success_and_text "oadm registry --daemonset -o yaml --credentials=${KUBECONFIG}" 'DaemonSet'
os::cmd::expect_success "oadm registry --daemonset --credentials=${KUBECONFIG} --images='${USE_IMAGES}'"
os::cmd::expect_success_and_text 'oadm registry --daemonset' 'service exists'
os::cmd::expect_success_and_text 'oc get ds/docker-registry --template="{{.status.desiredNumberScheduled}}"' '1'
# clean up so we can test non-daemonset
os::cmd::expect_success "oc delete ds/docker-registry svc/docker-registry"
echo "registry daemonset: ok"

# Test running a registry
os::cmd::expect_failure_and_text 'oadm registry --dry-run' 'does not exist'
os::cmd::expect_success_and_text "oadm registry -o yaml --credentials=${KUBECONFIG}" 'image:.*\-docker\-registry'
os::cmd::expect_success "oadm registry --credentials=${KUBECONFIG} --images='${USE_IMAGES}'"
os::cmd::expect_success_and_text 'oadm registry' 'service exists'
os::cmd::expect_success_and_text 'oc describe svc/docker-registry' 'Session Affinity:\s*ClientIP'
os::cmd::expect_success_and_text 'oc get dc/docker-registry -o yaml' 'readinessProbe'
os::cmd::expect_success_and_text 'oc env --list dc/docker-registry' 'REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ENFORCEQUOTA=false'
echo "registry: ok"
os::test::junit::declare_suite_end
