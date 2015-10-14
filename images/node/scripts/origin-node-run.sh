#!/bin/sh

set -eu

function quit {
    pkill openshift
    exit 0
}

trap quit SIGTERM

if [ ! -f ${HOST_ETC}/systemd/system/docker.service.d/docker-sdn-ovs.conf ]; then
    mkdir -p ${HOST_ETC}/systemd/system/docker.service.d
    cp /usr/lib/systemd/system/docker.service.d/docker-sdn-ovs.conf ${HOST_ETC}/systemd/system/docker.service.d
fi

/usr/bin/openshift start node --config=${CONFIG_FILE} ${OPTIONS}

while true; do sleep 5; done
