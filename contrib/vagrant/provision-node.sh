#!/bin/bash

source $(dirname $0)/provision-config.sh

# Provided index is 1-based, array is 0 based
NODE_NAME=${NODE_NAMES[${NODE_INDEX}-1]}

os::provision::base-provision "${ORIGIN_ROOT}"

# Waiting for node config to exist before deploying allows vm
# provisioning to safely execute in parallel.
if ! os::provision::in-container; then
  os::provision::wait-for-node-config "${CONFIG_ROOT}" "${NODE_NAME}"
fi

# Binaries are expected to have been built by the time node
# configuration is available.
os::provision::base-install "${ORIGIN_ROOT}" "${CONFIG_ROOT}"

echo "Launching openshift daemon"
os::provision::start-node-service "${CONFIG_ROOT}" "${NODE_NAME}"
