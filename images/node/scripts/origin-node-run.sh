#!/bin/sh

set -eu

conf=${CONFIG_FILE:-/etc/origin/node/node-config.yaml}
opts=${OPTIONS:---loglevel=2}

function quit {
    pkill openshift
    exit 0
}

trap quit SIGTERM

if [ ! -f ${HOST_ETC}/systemd/system/docker.service.d/docker-sdn-ovs.conf ]; then
    mkdir -p ${HOST_ETC}/systemd/system/docker.service.d
    cp /usr/lib/systemd/system/docker.service.d/docker-sdn-ovs.conf ${HOST_ETC}/systemd/system/docker.service.d
fi

/usr/bin/openshift start node "--config=${conf}" "${opts}"

while true; do sleep 5; done
