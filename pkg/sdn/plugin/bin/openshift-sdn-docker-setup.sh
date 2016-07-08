#!/bin/bash

set -ex

bridge=$1
mtu=$2

DOCKER_NETWORK_OPTIONS="-b=${bridge} --mtu=${mtu}"
conf=/run/openshift-sdn/docker-network

if grep -q -s "DOCKER_NETWORK_OPTIONS='${DOCKER_NETWORK_OPTIONS}'" $conf; then
    exit 0
fi

mkdir -p $(dirname $conf)
cat <<EOF > $conf
# This file has been modified by openshift-sdn.

DOCKER_NETWORK_OPTIONS='${DOCKER_NETWORK_OPTIONS}'
EOF

# Restart docker. "systemctl restart" will bail out (unnecessarily) in
# the OpenShift-in-a-container case, so we work around that by sending
# the messages by hand.
dbus-send --system --print-reply --reply-timeout=2000 --type=method_call --dest=org.freedesktop.systemd1 /org/freedesktop/systemd1 org.freedesktop.systemd1.Manager.Reload
dbus-send --system --print-reply --reply-timeout=2000 --type=method_call --dest=org.freedesktop.systemd1 /org/freedesktop/systemd1 org.freedesktop.systemd1.Manager.RestartUnit string:'docker.service' string:'replace'
