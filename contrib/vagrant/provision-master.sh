#!/bin/bash

set -ex
source $(dirname $0)/provision-config.sh

FIXUP_NET_UDEV=$5

NETWORK_PLUGIN=$(os::util::get-network-plugin ${6:-""})

if [ "${FIXUP_NET_UDEV}" == "true" ]; then
  NETWORK_CONF_PATH=/etc/sysconfig/network-scripts/
  rm -f ${NETWORK_CONF_PATH}ifcfg-enp*
  if [[ -f "${NETWORK_CONF_PATH}ifcfg-eth1" ]]; then
    sed -i 's/^NM_CONTROLLED=no/#NM_CONTROLLED=no/' ${NETWORK_CONF_PATH}ifcfg-eth1
    if ! grep -q "NAME=" ${NETWORK_CONF_PATH}ifcfg-eth1; then
      echo "NAME=openshift" >> ${NETWORK_CONF_PATH}ifcfg-eth1
    fi
    nmcli con reload
    nmcli dev disconnect eth1
    nmcli con up "openshift"
  fi
fi

# Setup hosts file to ensure name resolution to each member of the cluster
minion_ip_array=(${MINION_IPS//,/ })
os::util::setup-hosts-file "${MASTER_NAME}" "${MASTER_IP}" MINION_NAMES \
  minion_ip_array

# Install the required packages
yum install -y docker-io git golang e2fsprogs hg net-tools bridge-utils which

# Build openshift
echo "Building openshift"
pushd "${ORIGIN_ROOT}"
  ./hack/build-go.sh
  os::util::install-cmds "${ORIGIN_ROOT}"
  ./hack/install-etcd.sh
popd

os::util::init-certs "${ORIGIN_ROOT}" "${NETWORK_PLUGIN}" "${MASTER_NAME}" \
  "${MASTER_IP}" MINION_NAMES minion_ip_array

# Start docker
systemctl enable docker.service
systemctl start docker.service

# Create systemd service
node_list=$(os::util::join , ${MINION_NAMES[@]})
cat <<EOF > /usr/lib/systemd/system/openshift-master.service
[Unit]
Description=OpenShift Master
Requires=docker.service network.service
After=network.service

[Service]
ExecStart=/usr/bin/openshift start master --master=https://${MASTER_IP}:8443 --nodes=${node_list} --network-plugin=${NETWORK_PLUGIN}
WorkingDirectory=${ORIGIN_ROOT}/

[Install]
WantedBy=multi-user.target
EOF

# Start the service
systemctl daemon-reload
systemctl start openshift-master.service

# setup SDN
$(dirname $0)/provision-sdn.sh

# Set up the KUBECONFIG environment variable for use by oc
os::util::set-oc-env "${ORIGIN_ROOT}" "/root/.bash_profile"
os::util::set-oc-env "${ORIGIN_ROOT}" "/home/vagrant/.bash_profile"
