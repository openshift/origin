#!/bin/bash

source $(dirname $0)/provision-config.sh

os::provision::base-provision "${ORIGIN_ROOT}" true

os::provision::build-origin "${ORIGIN_ROOT}" "${SKIP_BUILD}"
os::provision::build-etcd "${ORIGIN_ROOT}" "${SKIP_BUILD}"

os::provision::base-install "${ORIGIN_ROOT}" "${CONFIG_ROOT}"

if [ "${SDN_NODE}" = "true" ]; then
  # Running an sdn node on the master when using an openshift sdn
  # plugin ensures connectivity between the openshift service and
  # pods.  This enables kube API calls that query a service and
  # require that the endpoints of the service be reachable from the
  # master.  This capability is used extensively in the kube e2e
  # tests.
  NODE_NAMES+=(${SDN_NODE_NAME})
  NODE_IPS+=(127.0.0.1)
  # Force the addition of a hosts entry for the sdn node.
  os::provision::add-to-hosts-file "${MASTER_IP}" "${SDN_NODE_NAME}" 1
fi

os::provision::init-certs "${CONFIG_ROOT}" "${NETWORK_PLUGIN}" \
  "${MASTER_NAME}" "${MASTER_IP}" NODE_NAMES NODE_IPS

echo "Launching openshift daemons"
NODE_LIST=$(os::provision::join , ${NODE_NAMES[@]})
cmd="/usr/bin/openshift start master --loglevel=${LOG_LEVEL} \
 --master=https://${MASTER_IP}:8443 \
 --network-plugin=${NETWORK_PLUGIN}"
os::provision::start-os-service "openshift-master" "OpenShift Master" "${cmd}"

if [ "${SDN_NODE}" = "true" ]; then
  os::provision::start-node-service "${CONFIG_ROOT}" "${SDN_NODE_NAME}"

  # Disable scheduling for the sdn node - its purpose is only to ensure
  # pod network connectivity on the master.
  #
  # This will be performed separately for dind to allow as much time
  # as possible for the node to register itself.  Vagrant can deploy
  # in parallel but dind deploys serially for simplicity.
  if ! os::provision::in-container; then
    os::provision::disable-sdn-node "${CONFIG_ROOT}" "${SDN_NODE_NAME}"
  fi
fi
