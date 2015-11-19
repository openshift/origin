#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source $(dirname "${BASH_SOURCE}")/init.sh

NETWORK_PLUGIN=$(os::util::get-network-plugin "${NETWORK_PLUGIN}")

# Running an openshift node on the master ensures connectivity between
# the openshift service and pods.  This supports kube API calls that
# query a service and require that the endpoints of the service be
# reachable from the master.
NODE_NAMES+=(${SDN_NODE_NAME})
NODE_IPS+=(127.0.0.1)

# Force the addition of a hosts entry for the sdn node.
os::util::add-to-hosts-file "${MASTER_IP}" "${SDN_NODE_NAME}" 1

os::util::setup-hosts-file "${MASTER_NAME}" "${MASTER_IP}" NODE_NAMES NODE_IPS

echo "Building and installing openshift"
${ORIGIN_ROOT}/hack/build-go.sh
os::util::install-cmds "${ORIGIN_ROOT}"
${ORIGIN_ROOT}/hack/install-etcd.sh
os::util::install-sdn "${ORIGIN_ROOT}"

os::util::init-certs "${DEPLOYED_CONFIG_ROOT}" "${NETWORK_PLUGIN}" \
  "${MASTER_NAME}" "${MASTER_IP}" NODE_NAMES NODE_IPS

NODE_NAME_LIST=$(os::util::join , ${NODE_NAMES[@]})
cat <<EOF >> "${SUPERVISORD_CONF}"

[program:openshift-master]
command=/usr/bin/openshift start master --loglevel=5 --master=https://${MASTER_IP}:8443 --nodes=${NODE_NAME_LIST} --network-plugin=${NETWORK_PLUGIN}
directory=${DEPLOYED_CONFIG_ROOT}
priority=10
startsecs=20
stderr_events_enabled=true
stdout_events_enabled=true

[program:openshift-master-sdn]
command=/usr/bin/openshift start node --loglevel=5 --config=${DEPLOYED_CONFIG_ROOT}/openshift.local.config/node-${SDN_NODE_NAME}/node-config.yaml
priority=20
startsecs=20
stderr_events_enabled=true
stdout_events_enabled=true
EOF

# Start openshift
supervisorctl update

os::dind::reload-docker

os::dind::set-dind-env "${ORIGIN_ROOT}" "${DEPLOYED_CONFIG_ROOT}"
