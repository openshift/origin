#!/bin/bash
set -ex
source $(dirname $0)/provision-config.sh

MINION_IP=$4
OPENSHIFT_SDN=$6

# Setup hosts file to support ping by hostname to master
if [ ! "$(cat /etc/hosts | grep $MASTER_NAME)" ]; then
  echo "Adding $MASTER_NAME to hosts file"
  echo "$MASTER_IP $MASTER_NAME" >> /etc/hosts
fi

# Setup hosts file to support ping by hostname to each minion in the cluster
minion_ip_array=(${MINION_IPS//,/ })
for (( i=0; i<${#MINION_NAMES[@]}; i++)); do
  minion=${MINION_NAMES[$i]}
  ip=${minion_ip_array[$i]}  
  if [ ! "$(cat /etc/hosts | grep $minion)" ]; then
    echo "Adding $minion to hosts file"
    echo "$ip $minion" >> /etc/hosts
  fi  
done

# Install the required packages
yum install -y docker-io git golang e2fsprogs hg openvswitch net-tools bridge-utils

# Build openshift
echo "Building openshift"
pushd /vagrant
  ./hack/build-go.sh
  cp _output/local/go/bin/openshift /usr/bin
popd

if [ "${OPENSHIFT_SDN}" != "ovs-gre" ]; then
  $(dirname $0)/provision-node-sdn.sh $@
else
  # Setup default networking between the nodes
  $(dirname $0)/provision-gre-network.sh $@
fi

# Copy over the certificates directory and modify the kubeconfig file to use the master ip
cp -r /vagrant/openshift.local.certificates /
chown -R vagrant.vagrant /openshift.local.certificates
sed -ie "s/10.0.2.15/${MASTER_IP}/g" /openshift.local.certificates/admin/.kubeconfig

# Create systemd service
cat <<EOF > /usr/lib/systemd/system/openshift-node.service
[Unit]
Description=OpenShift Node
Requires=docker.service network.service
After=network.service

[Service]
ExecStart=/usr/bin/openshift start node --kubeconfig=/openshift.local.certificates/admin/.kubeconfig --master=https://${MASTER_IP}:8443
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
echo 'export KUBECONFIG=/openshift.local.certificates/admin/.kubeconfig' >> /root/.bash_profile
echo 'export KUBECONFIG=/openshift.local.certificates/admin/.kubeconfig' >> /home/vagrant/.bash_profile

# Register with the master
#curl -X POST -H 'Accept: application/json' -d "{\"kind\":\"Minion\", \"id\":"${MINION_IP}", \"apiVersion\":\"v1beta1\", \"hostIP\":"${MINION_IP}" }" http://${MASTER_IP}:8080/api/v1beta1/minions
