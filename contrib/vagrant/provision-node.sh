#!/bin/bash

source $(dirname $0)/provision-config.sh

NODE_NAME=${NODE_NAMES[${NODE_INDEX}-1]}

os::provision::base-provision

# Waiting for node config to exist before deploying allows vm
# provisioning to safely execute in parallel.
if ! os::provision::in-container; then
  os::provision::wait-for-node-config "${CONFIG_ROOT}" "${NODE_NAME}"
fi

# openshift is assumed to have been built before node deployment
os::provision::install-cmds "${ORIGIN_ROOT}"

os::provision::install-sdn "${ORIGIN_ROOT}"

echo "Launching openshift daemon"
os::provision::start-node-service "${CONFIG_ROOT}" "${NODE_NAME}"

os::provision::set-os-env "${ORIGIN_ROOT}" "${CONFIG_ROOT}"
