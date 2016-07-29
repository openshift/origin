#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/admin/start"
# Check failure modes of various system commands
os::cmd::expect_failure_and_text 'openshift start network' 'kubeconfig must be set'
os::cmd::expect_failure_and_text 'openshift start network --config=${NODECONFIG} --enable=kubelet' 'the following components are not recognized: kubelet'
os::cmd::expect_failure_and_text 'openshift start network --config=${NODECONFIG} --enable=kubelet,other' 'the following components are not recognized: kubelet, other'
os::cmd::expect_failure_and_text 'openshift start network --config=${NODECONFIG} --disable=other' 'the following components are not recognized: other'
os::cmd::expect_failure_and_text 'openshift start network --config=${NODECONFIG} --disable=dns,proxy,plugins' 'at least one node component must be enabled \(dns, plugins, proxy\)'
os::cmd::expect_failure_and_text 'openshift start node' 'kubeconfig must be set'
os::cmd::expect_failure_and_text 'openshift start node --config=${NODECONFIG} --disable=other' 'the following components are not recognized: other'
os::cmd::expect_failure_and_text 'openshift start node --config=${NODECONFIG} --disable=dns,kubelet,proxy,plugins' 'at least one node component must be enabled \(dns, kubelet, plugins, proxy\)'
os::cmd::expect_failure_and_text 'openshift start --write-config=/tmp/test --hostname=""' 'error: --hostname must have a value'
os::test::junit::declare_suite_end
