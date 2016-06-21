#!/bin/bash

source $(dirname $0)/provision-config.sh

# Provided index is 1-based, array is 0 based
NODE_NAME=${NODE_NAMES[${NODE_INDEX}-1]}

os::provision::base-provision "${OS_ROOT}"

# Waiting for node config to exist before deploying allows vm
# provisioning to safely execute in parallel.
if ! os::provision::in-container; then
  os::provision::wait-for-node-config "${CONFIG_ROOT}" "${NODE_NAME}"
fi

# Copy configuration to local storage because each node's openshift
# service uses the configuration path as a working directory and it is
# not desirable for nodes to share a working directory.
DEPLOYED_CONFIG_ROOT="/"
os::provision::copy-config "${CONFIG_ROOT}"

# Binaries are expected to have been built by the time node
# configuration is available.
os::provision::base-install "${OS_ROOT}" "${DEPLOYED_CONFIG_ROOT}"

echo "Launching openshift daemon"
os::provision::start-node-service "${DEPLOYED_CONFIG_ROOT}" "${NODE_NAME}"
