#!/bin/bash

source $(dirname $0)/provision-config.sh

os::util::base-provision

# openshift is assumed to have been built before node deployment
os::util::install-cmds "${ORIGIN_ROOT}"

os::util::install-sdn "${ORIGIN_ROOT}"

echo "Launching openshift daemon"
NODE_NAME=${NODE_NAMES[${NODE_INDEX}-1]}
os::util::start-node-service "${NODE_NAME}"

os::util::set-os-env "${ORIGIN_ROOT}" "${CONFIG_ROOT}"
