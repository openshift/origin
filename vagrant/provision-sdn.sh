#!/bin/bash
set -ex
source $(dirname $0)/provision-config.sh

pushd $HOME
# build openshift-sdn
if [ -d openshift-sdn ]; then
    cd openshift-sdn
    git fetch origin
    git reset --hard origin/master
    git checkout -b multitenant
else
    git clone https://github.com/openshift/openshift-sdn -b multitenant
    cd openshift-sdn
fi

make clean
make
make install
popd

systemctl enable openvswitch
systemctl start openvswitch

# no need to start openshift-sdn, as it is integrated with openshift binary
