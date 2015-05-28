#!/bin/bash

set -ex

subnet_gateway=$1
subnet=$2
cluster_subnet=$3
subnet_mask_len=$4
tun_gateway=$5
printf 'Container network is "%s"; local host has subnet "%s" and gateway "%s".\n' "${cluster_subnet}" "${subnet}" "${subnet_gateway}"
TUN=tun0

## openvswitch
ovs-vsctl del-br br0 || true
ovs-vsctl add-br br0 -- set Bridge br0 fail-mode=secure
ovs-vsctl set bridge br0 protocols=OpenFlow13
ovs-vsctl del-port br0 vxlan0 || true
ovs-vsctl add-port br0 vxlan0 -- set Interface vxlan0 type=vxlan options:remote_ip="flow" options:key="flow" ofport_request=1
ovs-vsctl add-port br0 ${TUN} -- set Interface ${TUN} type=internal ofport_request=2

## linux bridge
ip link set lbr0 down || true
brctl delbr lbr0 || true
brctl addbr lbr0
ip addr add ${subnet_gateway}/${subnet_mask_len} dev lbr0
ip link set lbr0 up

# setup tun address
ip addr add ${tun_gateway}/${subnet_mask_len} dev ${TUN}
ip link set ${TUN} up
ip route add ${cluster_subnet} dev ${TUN} proto kernel scope link

## iptables
iptables -t nat -D POSTROUTING -s ${cluster_subnet} ! -d ${cluster_subnet} -j MASQUERADE || true
iptables -t nat -A POSTROUTING -s ${cluster_subnet} ! -d ${cluster_subnet} -j MASQUERADE
iptables -D INPUT -p udp -m multiport --dports 4789 -m comment --comment "001 vxlan incoming" -j ACCEPT || true
iptables -D INPUT -i ${TUN} -m comment --comment "traffic from docker for internet" -j ACCEPT || true
lineno=$(iptables -nvL INPUT --line-numbers | grep "state RELATED,ESTABLISHED" | awk '{print $1}')
iptables -I INPUT $lineno -p udp -m multiport --dports 4789 -m comment --comment "001 vxlan incoming" -j ACCEPT
iptables -I INPUT $((lineno+1)) -i ${TUN} -m comment --comment "traffic from docker for internet" -j ACCEPT
fwd_lineno=$(iptables -nvL FORWARD --line-numbers | grep "reject-with icmp-host-prohibited" tail -n 1 | awk '{print $1}')
iptables -I FORWARD $fwd_lineno -d ${cluster_subnet} -j ACCEPT
iptables -I FORWARD $fwd_lineno -s ${cluster_subnet} -j ACCEPT

## docker
if [[ -z "${DOCKER_NETWORK_OPTIONS}" ]]
then
    DOCKER_NETWORK_OPTIONS='-b=lbr0 --mtu=1450'
fi

if ! grep -q "^DOCKER_NETWORK_OPTIONS='${DOCKER_NETWORK_OPTIONS}'" /etc/sysconfig/docker-network
then
    cat <<EOF > /etc/sysconfig/docker-network
# This file has been modified by openshift-sdn. Please modify the
# DOCKER_NETWORK_OPTIONS variable in the /etc/sysconfig/openshift-sdn-node,
# /etc/sysconfig/openshift-sdn-master or /etc/sysconfig/openshift-sdn
# files (depending on your setup).

DOCKER_NETWORK_OPTIONS='${DOCKER_NETWORK_OPTIONS}'
EOF
fi
systemctl daemon-reload
systemctl restart docker.service

# delete the subnet routing entry created because of lbr0
ip route del ${subnet} dev lbr0 proto kernel scope link src ${subnet_gateway} || true

mkdir -p /etc/openshift-sdn
echo "export OPENSHIFT_SDN_TAP1_ADDR=${tun_gateway}" >& "/etc/openshift-sdn/config.env"
echo "export OPENSHIFT_CLUSTER_SUBNET=${cluster_subnet}" >> "/etc/openshift-sdn/config.env"
