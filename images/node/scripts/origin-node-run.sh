#!/bin/sh

set -eu

hostetc=${HOST_ETC:-/rootfs/etc}
conf=${CONFIG_FILE:-/etc/origin/node/node-config.yaml}
opts=${OPTIONS:---loglevel=2}
if [ "$#" -ne 0 ]; then
  opts=""
fi

if [ ! -f ${hostetc}/systemd/system/docker.service.d/docker-sdn-ovs.conf ]; then
    mkdir -p ${hostetc}/systemd/system/docker.service.d
    cp /usr/lib/systemd/system/docker.service.d/docker-sdn-ovs.conf ${hostetc}/systemd/system/docker.service.d
fi

exec /usr/bin/openshift start node "--config=${conf}" "${opts}" $@
