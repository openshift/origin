#!/bin/bash

set -ex
source $(dirname $0)/provision-config.sh

# Setup hosts file to support ping by hostname to each minion in the cluster from apiserver
node_list=""
minion_ip_array=(${MINION_IPS//,/ })
for (( i=0; i<${#MINION_NAMES[@]}; i++)); do
  minion=${MINION_NAMES[$i]}
  node_list="${node_list},${minion}"
  ip=${minion_ip_array[$i]}
  if [ ! "$(cat /etc/hosts | grep $minion)" ]; then
    echo "Adding $minion to hosts file"
    echo "$ip $minion" >> /etc/hosts
  fi
done
node_list=${node_list:1}

# Install the required packages
yum install -y docker-io git golang e2fsprogs hg net-tools bridge-utils

# Build openshift
echo "Building openshift"
pushd /vagrant
  ./hack/build-go.sh
  cp _output/local/go/bin/openshift /usr/bin
  ./hack/install-etcd.sh
popd

# Start docker
systemctl enable docker.service
systemctl start docker.service
usermod -a -G docker vagrant

# Create systemd service
cat <<EOF > /usr/lib/systemd/system/openshift-master.service
[Unit]
Description=openshift master
After=network.service

[Service]
ExecStart=/usr/bin/openshift start master --master=http://${MASTER_IP}:8080 --nodes=${node_list}

[Install]
WantedBy=multi-user.target
EOF

# Start the service
systemctl daemon-reload
systemctl enable openshift-master.service
systemctl start openshift-master.service
