#!/bin/bash

source $(dirname $0)/provision-config.sh

os::provision::base-provision

# openshift is assumed to have been built before node deployment
os::provision::install-cmds "${ORIGIN_ROOT}"

os::provision::install-sdn "${ORIGIN_ROOT}"

echo "Launching openshift daemon"
NODE_NAME=${NODE_NAMES[${NODE_INDEX}-1]}
os::provision::start-node-service "${NODE_NAME}"

os::provision::set-os-env "${ORIGIN_ROOT}" "${CONFIG_ROOT}"
