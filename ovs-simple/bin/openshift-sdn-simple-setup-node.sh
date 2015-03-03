#!/bin/bash

set -ex

printf 'Container network is "%s"; local host has subnet "%s" and gateway "%s".\n' "$3" "$2" "$1"

## openvswitch
ovs-vsctl del-br br0 || true
ovs-vsctl add-br br0 -- set Bridge br0 fail-mode=secure
ovs-vsctl set bridge br0 protocols=OpenFlow13
ovs-vsctl del-port br0 vxlan0 || true
ovs-vsctl add-port br0 vxlan0 -- set Interface vxlan0 type=vxlan options:remote_ip="flow" options:key="flow" ofport_request=10
ip link del vlinuxbr || true
ip link add vlinuxbr type veth peer name vovsbr
ip link set vlinuxbr up
ip link set vovsbr up
ip link set vlinuxbr txqueuelen 0
ip link set vovsbr txqueuelen 0

ovs-vsctl del-port br0 vovsbr || true
ovs-vsctl add-port br0 vovsbr -- set Interface vovsbr ofport_request=9

## linux bridge
ip link set lbr0 down || true
brctl delbr lbr0 || true
brctl addbr lbr0
ip addr add ${1}/24 dev lbr0
ip link set lbr0 up
brctl addif lbr0 vlinuxbr
ip route del $2 dev lbr0 proto kernel scope link src $1 || true
ip route add $3 dev lbr0 proto kernel scope link src $1


## iptables
iptables -t nat -D POSTROUTING -s 10.1.0.0/16 ! -d 10.1.0.0/16 -j MASQUERADE || true
iptables -t nat -A POSTROUTING -s 10.1.0.0/16 ! -d 10.1.0.0/16 -j MASQUERADE
iptables -D INPUT -p udp -m multiport --dports 4789 -m comment --comment "001 vxlan incoming" -j ACCEPT || true
iptables -D INPUT -i lbr0 -m comment --comment "traffic from docker" -j ACCEPT || true
lineno=$(iptables -nvL INPUT --line-numbers | grep "state RELATED,ESTABLISHED" | awk '{print $1}')
iptables -I INPUT $lineno -p udp -m multiport --dports 4789 -m comment --comment "001 vxlan incoming" -j ACCEPT
iptables -I INPUT $((lineno+1)) -i lbr0 -m comment --comment "traffic from docker" -j ACCEPT


## docker
if [[ -z "${DOCKER_OPTIONS}" ]]
then
    DOCKER_OPTIONS='-b=lbr0 --mtu=1450 --selinux-enabled'
fi

if ! grep -q "^OPTIONS='${DOCKER_OPTIONS}'" /etc/sysconfig/docker
then
    cat <<EOF > /etc/sysconfig/docker
# This file has been modified by openshift-sdn. Please modify the
# DOCKER_OPTIONS variable in the /etc/sysconfig/openshift-sdn-node,
# /etc/sysconfig/openshift-sdn-master or /etc/sysconfig/openshift-sdn
# files (depending on your setup).

OPTIONS='${DOCKER_OPTIONS}'
EOF
fi
systemctl daemon-reload
systemctl restart docker.service

