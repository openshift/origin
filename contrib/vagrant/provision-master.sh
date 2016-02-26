#!/bin/bash

source $(dirname $0)/provision-config.sh

os::provision::base-provision "${ORIGIN_ROOT}" true

os::provision::build-origin "${ORIGIN_ROOT}" "${SKIP_BUILD}"
os::provision::build-etcd "${ORIGIN_ROOT}" "${SKIP_BUILD}"

echo "Installing openshift"
os::provision::install-cmds "${ORIGIN_ROOT}"

# TODO(marun) Should only deploy sdn node when openshift-sdn is configured
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

if [ "${NETWORK_PLUGIN}" = "flannel" ]; then
  # Avoid setting a network plugin in the openshift config when
  # flannel is used.
  CONFIG_NETWORK_PLUGIN=""
  NETWORK_PLUGIN_OPT=""
else
  CONFIG_NETWORK_PLUGIN="${NETWORK_PLUGIN}"
  NETWORK_PLUGIN_OPT="--network-plugin=${NETWORK_PLUGIN}"
fi

os::provision::init-certs "${CONFIG_ROOT}" "${CONFIG_NETWORK_PLUGIN}" \
    "${MASTER_NAME}" "${MASTER_IP}" NODE_NAMES NODE_IPS

# Copy configuration to local storage when the configuration path is
# mounted over nfs to prevent etcd from experiencing nfs-related
# locking errors.
CONFIG_MOUNT_TYPE=$(df -P -T "${CONFIG_ROOT}" | tail -n +2 | awk '{print $2}')
if [[ "${CONFIG_MOUNT_TYPE}" = "nfs" ]]; then
  DEPLOYED_CONFIG_ROOT="/"
  os::provision::copy-config "${CONFIG_ROOT}"
else
  DEPLOYED_CONFIG_ROOT="${CONFIG_ROOT}"
fi

echo "Launching openshift daemons"
cmd="/usr/bin/openshift start master --loglevel=${LOG_LEVEL} \
 --master=https://${MASTER_IP}:8443 \
 ${NETWORK_PLUGIN_OPT}"
os::provision::start-os-service "openshift-master" "OpenShift Master" \
    "${cmd}" "${DEPLOYED_CONFIG_ROOT}"

# Install networking after starting openshift to ensure that plugins
# like flannel can write configuration to openshift's etcd server.
os::provision::install-networking "${NETWORK_PLUGIN}" "${MASTER_IP}" \
    "${ORIGIN_ROOT}" "${DEPLOYED_CONFIG_ROOT}" true

if [ "${SDN_NODE}" = "true" ]; then
  os::provision::start-node-service "${DEPLOYED_CONFIG_ROOT}" \
      "${SDN_NODE_NAME}"

  # Disable scheduling for the sdn node - it's purpose is only to ensure
  # pod network connectivity on the master.
  os::provision::disable-sdn-node "${DEPLOYED_CONFIG_ROOT}" "${SDN_NODE_NAME}"
fi

os::provision::set-os-env "${ORIGIN_ROOT}" "${DEPLOYED_CONFIG_ROOT}"
