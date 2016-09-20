#!/bin/bash

source $(dirname $0)/provision-config.sh

os::provision::base-provision "${OS_ROOT}" true

os::provision::build-origin "${OS_ROOT}" "${SKIP_BUILD}"
os::provision::build-etcd "${OS_ROOT}" "${SKIP_BUILD}"

os::provision::base-install "${OS_ROOT}" "${CONFIG_ROOT}"

if [[ "${SDN_NODE}" = "true" ]]; then
  # Running an sdn node on the master when using an openshift sdn
  # plugin ensures connectivity between the openshift service and
  # pods.  This enables kube API calls that query a service and
  # require that the endpoints of the service be reachable from the
  # master.  This capability is used extensively in the kube e2e
  # tests.
  NODE_NAMES+=(${SDN_NODE_NAME})
  NODE_IPS+=(${MASTER_IP})
  # Force the addition of a hosts entry for the sdn node.
  os::provision::add-to-hosts-file "${MASTER_IP}" "${SDN_NODE_NAME}" 1
fi

os::provision::init-certs "${CONFIG_ROOT}" "${NETWORK_PLUGIN}" \
  "${MASTER_NAME}" "${MASTER_IP}" NODE_NAMES NODE_IPS

# Copy configuration to local storage when the configuration path is
# mounted over nfs to prevent etcd from experiencing nfs-related
# locking errors.
CONFIG_MOUNT_TYPE=$(df -P -T "${CONFIG_ROOT}" | tail -n +2 | awk '{print $2}')
if [[ "${CONFIG_MOUNT_TYPE}" = "nfs" ]]; then
  DEPLOYED_CONFIG_ROOT="/"
  echo "WARNING: NFS detected. Cluster state will not be retained if the cluster is redeployed."
  os::provision::copy-config "${CONFIG_ROOT}"
else
  DEPLOYED_CONFIG_ROOT="${CONFIG_ROOT}"
fi

echo "Launching openshift daemons"
NODE_LIST="$(os::provision::join , ${NODE_NAMES[@]})"
cmd="/usr/bin/openshift start master --loglevel=${LOG_LEVEL} \
 --master=https://${MASTER_IP}:8443 \
 --network-plugin=${NETWORK_PLUGIN}"
os::provision::start-os-service "openshift-master" "OpenShift Master" \
    "${cmd}" "${DEPLOYED_CONFIG_ROOT}"

if [[ "${SDN_NODE}" = "true" ]]; then
  os::provision::start-node-service "${DEPLOYED_CONFIG_ROOT}" \
      "${SDN_NODE_NAME}"

  # Disable scheduling for the sdn node - its purpose is only to ensure
  # pod network connectivity on the master.
  #
  # This will be performed separately for dind to allow as much time
  # as possible for the node to register itself.  Vagrant can deploy
  # in parallel but dind deploys serially for simplicity.
  if ! os::provision::in-container; then
    os::provision::disable-node "${OS_ROOT}" "${DEPLOYED_CONFIG_ROOT}" \
        "${SDN_NODE_NAME}"
  fi
fi
