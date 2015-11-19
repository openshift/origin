#!/bin/sh

set -eu

function quit {
    /usr/share/openvswitch/scripts/ovs-ctl stop
    exit 0
}

trap quit SIGTERM

/usr/share/openvswitch/scripts/ovs-ctl start --system-id=random

while true; do sleep 5; done

