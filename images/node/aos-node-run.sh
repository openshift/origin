#!/bin/sh

set -eu

function quit {
    pkill openshift
    /usr/share/openvswitch/scripts/ovs-ctl stop
    rm -f ${HOST_ETC}/systemd/system/docker.service.d/docker-sdn-ovs.conf
    exit 0
}

trap quit SIGTERM

if [ ! -f ${HOST_ETC}/systemd/system/docker.service.d/docker-sdn-ovs.conf ]; then
    mkdir -p ${HOST_ETC}/systemd/system/docker.service.d
    cp /usr/lib/systemd/system/docker.service.d/docker-sdn-ovs.conf ${HOST_ETC}/systemd/system/docker.service.d
fi

/usr/share/openvswitch/scripts/ovs-ctl start --system-id=random

/usr/bin/openshift start node --config=/etc/origin/node/node-config.yaml --loglevel=2 &

while true; do sleep 5; done

