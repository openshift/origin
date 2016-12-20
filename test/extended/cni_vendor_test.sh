#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"

# Set this to false if namespaces are not isolated
export OPENSHIFT_NETWORK_ISOLATION="${OPENSHIFT_NETWORK_ISOLATION:-true}"
export NETWORKING_E2E_FOCUS="${NETWORKING_E2E_FOCUS:-\[networking\]}"
export NETWORKING_E2E_MINIMAL=1

# Checking for a given kubeconfig
os::log::info "Starting 'networking' extended tests for cni plugin"
if [[ -n "${OPENSHIFT_TEST_KUBECONFIG:-}" ]]; then
  # Run tests against an existing cluster
  "${OS_ROOT}/test/extended/networking.sh" $@
else
  os::log::error "Please set env OPENSHIFT_TEST_KUBECONFIG to run the tests against an existing cluster"
  exit 1
fi
