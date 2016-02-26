#!/bin/bash

source $(dirname $0)/provision-config.sh

# Copy configuration to local storage because each node's openshift
# service uses the configuration path as a working directory and it is
# not desirable for nodes to share a working directory.
DEPLOYED_CONFIG_ROOT="/"

# Provided index is 1-based, array is 0 based
NODE_NAME=${NODE_NAMES[${NODE_INDEX}-1]}

os::provision::base-provision "${ORIGIN_ROOT}"

# Waiting for node config to exist before deploying allows vm
# provisioning to safely execute in parallel.
if ! os::provision::in-container; then
  os::provision::wait-for-node-config "${CONFIG_ROOT}" "${NODE_NAME}"
fi

os::provision::copy-config "${CONFIG_ROOT}"

# openshift is assumed to have been built before node deployment
os::provision::install-cmds "${ORIGIN_ROOT}"

os::provision::copy-config "${CONFIG_ROOT}"

echo "Launching openshift daemon"
os::provision::start-node-service "${DEPLOYED_CONFIG_ROOT}" "${NODE_NAME}"

os::provision::install-networking "${NETWORK_PLUGIN}" "${MASTER_IP}" \
    "${ORIGIN_ROOT}" "${DEPLOYED_CONFIG_ROOT}"

os::provision::set-os-env "${ORIGIN_ROOT}" "${DEPLOYED_CONFIG_ROOT}"
