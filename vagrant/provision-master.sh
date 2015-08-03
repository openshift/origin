#!/bin/bash

set -ex
source $(dirname $0)/provision-config.sh

OPENSHIFT_SDN=$4

NETWORK_CONF_PATH=/etc/sysconfig/network-scripts/
sed -i 's/^NM_CONTROLLED=no/#NM_CONTROLLED=no/' ${NETWORK_CONF_PATH}ifcfg-eth1

systemctl restart network

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
if ! grep ${MASTER_IP} /etc/hosts; then
  echo "${MASTER_IP} ${MASTER_NAME}" >> /etc/hosts
fi
node_list=${node_list:1}

# Install the required packages
yum install -y docker-io git golang e2fsprogs hg net-tools bridge-utils which

# Build openshift
echo "Building openshift"
pushd /vagrant
  ./hack/build-go.sh
  cp _output/local/go/bin/openshift /usr/bin
  ./hack/install-etcd.sh
popd

# Initialize certificates
echo "Generating certs"
pushd /vagrant
  SERVER_CONFIG_DIR="`pwd`/origin.local.config"
  VOLUMES_DIR="/var/lib/origin.local.volumes"
  MASTER_CONFIG_DIR="${SERVER_CONFIG_DIR}/master"
  CERT_DIR="${MASTER_CONFIG_DIR}"

  # Master certs
  /usr/bin/openshift admin ca create-master-certs \
    --overwrite=false \
    --cert-dir=${CERT_DIR} \
    --master=https://${MASTER_IP}:8443 \
    --hostnames=${MASTER_IP},${MASTER_NAME}

  # Certs for nodes
  for (( i=0; i<${#MINION_NAMES[@]}; i++)); do
    minion=${MINION_NAMES[$i]}
    ip=${minion_ip_array[$i]}

    /usr/bin/openshift admin create-node-config \
      --node-dir="${SERVER_CONFIG_DIR}/node-${minion}" \
      --node="${minion}" \
      --hostnames="${minion},${ip}" \
      --master="https://${MASTER_IP}:8443" \
      --network-plugin="redhat/openshift-ovs-subnet" \
      --node-client-certificate-authority="${CERT_DIR}/ca.crt" \
      --certificate-authority="${CERT_DIR}/ca.crt" \
      --signer-cert="${CERT_DIR}/ca.crt" \
      --signer-key="${CERT_DIR}/ca.key" \
      --signer-serial="${CERT_DIR}/ca.serial.txt" \
      --volume-dir="${VOLUMES_DIR}"
  done

popd

# Start docker
systemctl enable docker.service
systemctl start docker.service

# Create systemd service
cat <<EOF > /usr/lib/systemd/system/openshift-master.service
[Unit]
Description=OpenShift Master
Requires=docker.service network.service
After=network.service

[Service]
ExecStart=/usr/bin/openshift start master --master=https://${MASTER_IP}:8443 --nodes=${node_list} --network-plugin=redhat/openshift-ovs-subnet
WorkingDirectory=/vagrant/

[Install]
WantedBy=multi-user.target
EOF

# Start the service
systemctl daemon-reload
systemctl start openshift-master.service

# if SDN requires service on master, then set it up
if [ "${OPENSHIFT_SDN}" != "ovs-gre" ]; then
  export ETCD_CAFILE=/vagrant/origin.local.config/master/ca.crt
  export ETCD_CERTFILE=/vagrant/origin.local.config/master/master.etcd-client.crt
  export ETCD_KEYFILE=/vagrant/origin.local.config/master/master.etcd-client.key
  $(dirname $0)/provision-master-sdn.sh $@
fi

# Set up the KUBECONFIG environment variable for use by oc
echo 'export KUBECONFIG=/vagrant/origin.local.config/master/admin.kubeconfig' >> /root/.bash_profile
echo 'export KUBECONFIG=/vagrant/origin.local.config/master/admin.kubeconfig' >> /home/vagrant/.bash_profile
