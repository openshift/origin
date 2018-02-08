#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"

# Uncomment this if the plugin does not implement NetworkPolicy:

# export NETWORKING_E2E_NETWORKPOLICY=false

# Uncomment this if the plugin implements isolation in the same manner as
# redhat/openshift-ovs-multitenant:

# export NETWORKING_E2E_ISOLATION=true

export NETWORKING_E2E_FOCUS="${NETWORKING_E2E_FOCUS:-\[Area:Networking\]}"
export NETWORKING_E2E_EXTERNAL=1

# Checking for a given kubeconfig
os::log::info "Starting 'networking' extended tests for cni plugin"
if [[ -n "${OPENSHIFT_TEST_KUBECONFIG:-}" ]]; then
  # Run tests against an existing cluster
  "${OS_ROOT}/test/extended/networking.sh" $@
else
  os::log::error "Please set env OPENSHIFT_TEST_KUBECONFIG to run the tests against an existing cluster"
  exit 1
fi
