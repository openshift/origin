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

os::test::junit::declare_suite_start "cmd/admin/manage-node"
# Test admin manage-node operations
os::cmd::expect_success_and_text 'openshift admin manage-node --help' 'Manage nodes'

# create a node object to mess with
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

os::cmd::expect_success_and_text 'oadm manage-node --selector= --schedulable=true' 'Ready'
os::cmd::expect_success_and_not_text 'oadm manage-node --selector= --schedulable=true' 'Sched'
echo "manage-node: ok"
os::test::junit::declare_suite_end
