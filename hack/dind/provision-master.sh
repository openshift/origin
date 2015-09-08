#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source $(dirname "${BASH_SOURCE}")/init.sh

NETWORK_PLUGIN=$(os::util::get-network-plugin "${NETWORK_PLUGIN}")

os::util::setup-hosts-file "${MASTER_NAME}" "${MASTER_IP}" NODE_NAMES NODE_IPS

echo "Building and installing openshift"
${ORIGIN_ROOT}/hack/build-go.sh
os::util::install-cmds "${ORIGIN_ROOT}"
${ORIGIN_ROOT}/hack/install-etcd.sh
os::util::install-sdn "${ORIGIN_ROOT}"

os::util::init-certs "${ORIGIN_ROOT}" "${NETWORK_PLUGIN}" "${MASTER_NAME}" \
  "${MASTER_IP}" NODE_NAMES NODE_IPS

NODE_NAME_LIST=$(os::util::join , ${NODE_NAMES[@]})
cat <<EOF >> /etc/supervisord.conf

[program:openshift-master]
command=/usr/bin/openshift start master --loglevel=5 --master=https://${MASTER_IP}:8443 --nodes=${NODE_NAME_LIST} --network-plugin=${NETWORK_PLUGIN}
directory=${ORIGIN_ROOT}
priority=10
startsecs=20
stderr_events_enabled=true
stdout_events_enabled=true
EOF

# Start openshift
supervisorctl update

os::dind::set-dind-env "${ORIGIN_ROOT}"
