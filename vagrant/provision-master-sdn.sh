#!/bin/bash
set -ex
source $(dirname $0)/provision-config.sh

pushd $HOME
# build openshift-sdn
if [ -d openshift-sdn ]; then
    cd openshift-sdn
    git fetch origin
    git reset --hard origin/master
else
    git clone https://github.com/openshift/openshift-sdn
    cd openshift-sdn
fi

make clean
make
make install
popd

# Create systemd service
cat <<EOF > /usr/lib/systemd/system/openshift-master-sdn.service
[Unit]
Description=OpenShift SDN Master
Requires=openshift-master.service
After=openshift-master.service

[Service]
ExecStart=/usr/bin/openshift-sdn -etcd-endpoints=http://${MASTER_IP}:4001 

[Install]
WantedBy=multi-user.target
EOF

# Start the service
systemctl daemon-reload
systemctl start openshift-master-sdn.service
