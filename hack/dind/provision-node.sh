#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source $(dirname "${BASH_SOURCE}")/init.sh

os::util::setup-hosts-file ${MASTER_NAME} ${MASTER_IP} NODE_NAMES NODE_IPS

echo "Installing openshift"
os::util::install-cmds "${ORIGIN_ROOT}"
os::util::install-sdn "${ORIGIN_ROOT}"

cat <<EOF >> "${SUPERVISORD_CONF}"

[program:openshift-node]
command=/usr/bin/openshift start node --loglevel=5 --config=${DEPLOYED_CONFIG_ROOT}/openshift.local.config/node-${HOST_NAME}/node-config.yaml
priority=20
startsecs=20
stderr_events_enabled=true
stdout_events_enabled=true
EOF

# Start openshift
supervisorctl update

os::dind::reload-docker

os::dind::set-dind-env "${ORIGIN_ROOT}" "${DEPLOYED_CONFIG_ROOT}"
