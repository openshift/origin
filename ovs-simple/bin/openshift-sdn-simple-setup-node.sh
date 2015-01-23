#!/bin/bash

set -ex

echo $@

ovs-vsctl del-br br0 || true
ovs-vsctl add-br br0 -- set Bridge br0 fail-mode=secure
ovs-vsctl set bridge br0 stp_enable=true
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
cat <<EOF > /etc/sysconfig/network-scripts/ifcfg-lbr0
DEVICE=lbr0
ONBOOT=yes
TYPE=Bridge
BOOTPROTO=static
IPADDR=$1
NETMASK=255.255.255.0
STP=yes
EOF
service network restart || true
ip link set lbr0 up
brctl addif lbr0 vlinuxbr
ip route del $2 dev lbr0 proto kernel scope link src $1 || true
ip route add $3 dev lbr0 proto kernel scope link src $1
iptables -t nat -D POSTROUTING -s 10.1.0.0/16 ! -d 10.1.0.0/16 -j MASQUERADE || true
iptables -t nat -A POSTROUTING -s 10.1.0.0/16 ! -d 10.1.0.0/16 -j MASQUERADE

echo "OPTIONS='-b=lbr0 --iptables=false --selinux-enabled'" >/etc/sysconfig/docker
systemctl daemon-reload
systemctl restart docker.service

