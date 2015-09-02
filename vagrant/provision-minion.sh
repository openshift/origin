#!/bin/bash
set -ex
source $(dirname $0)/provision-config.sh

MINION_IP=$5
MINION_INDEX=$6

NETWORK_CONF_PATH=/etc/sysconfig/network-scripts/
sed -i 's/^NM_CONTROLLED=no/#NM_CONTROLLED=no/' ${NETWORK_CONF_PATH}ifcfg-eth1

systemctl restart network

# get the minion name, index is 1-based
minion_name=${MINION_NAMES[$MINION_INDEX-1]}

# Setup hosts file to ensure name resolution to each member of the cluster
minion_ip_array=(${MINION_IPS//,/ })
os::util::setup-hosts-file "${MASTER_NAME}" "${MASTER_IP}" MINION_NAMES \
  minion_ip_array

# Install the required packages
yum install -y docker-io git golang e2fsprogs hg openvswitch net-tools bridge-utils which ethtool

# Build openshift
echo "Building openshift"
pushd "${ORIGIN_ROOT}"
  ./hack/build-go.sh
  os::util::install-cmds "${ORIGIN_ROOT}"
popd

# Copy over the certificates directory
cp -r "${ORIGIN_ROOT}/openshift.local.config" /
chown -R vagrant.vagrant /openshift.local.config

mkdir -p /openshift.local.volumes

# Setup SDN
$(dirname $0)/provision-sdn.sh

# Create systemd service
cat <<EOF > /usr/lib/systemd/system/openshift-node.service
[Unit]
Description=OpenShift Node
Requires=network.service
After=docker.service network.service

[Service]
ExecStart=/usr/bin/openshift start node --config=/openshift.local.config/node-${minion_name}/node-config.yaml
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
EOF

# Start the service
systemctl daemon-reload
systemctl enable openshift-node.service
systemctl start openshift-node.service

# Set up the KUBECONFIG environment variable for use by the client
os::util::set-oc-env / "/root/.bash_profile"
os::util::set-oc-env / "/home/vagrant/.bash_profile"

# Register with the master
#curl -X POST -H 'Accept: application/json' -d "{\"kind\":\"Minion\", \"id\":"${MINION_IP}", \"apiVersion\":\"v1beta1\", \"hostIP\":"${MINION_IP}" }" http://${MASTER_IP}:8080/api/v1beta1/minions
