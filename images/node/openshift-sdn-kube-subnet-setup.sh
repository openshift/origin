#!/bin/bash

set -ex

lock_file=/var/lock/openshift-sdn.lock
subnet_gateway=$1
subnet=$2
cluster_subnet=$3
subnet_mask_len=$4
tun_gateway=$5
printf 'Container network is "%s"; local host has subnet "%s" and gateway "%s".\n' "${cluster_subnet}" "${subnet}" "${subnet_gateway}"
TUN=tun0

# Synchronize code execution with a file lock.
function lockwrap() {
    (
    flock 200
    "$@"
    ) 200>${lock_file}
}

function docker_setup_required() {
    ip=$(echo `ip a s lbr0 2>/dev/null|awk '/inet / {print $2}'`)
    if [ "$ip" != "${subnet_gateway}/${subnet_mask_len}" ]; then
        return 0
    fi
    if ! grep -q lbr0 /run/openshift-sdn/docker-network; then
        return 0
    fi
    return 1
}

function docker_setup() {
    ip link del vlinuxbr || true
    ip link add vlinuxbr type veth peer name vovsbr
    ip link set vlinuxbr up
    ip link set vovsbr up
    ip link set vlinuxbr txqueuelen 0
    ip link set vovsbr txqueuelen 0

    ## linux bridge
    ip link set lbr0 down || true
    brctl delbr lbr0 || true
    brctl addbr lbr0
    ip addr add ${subnet_gateway}/${subnet_mask_len} dev lbr0
    ip link set lbr0 up
    brctl addif lbr0 vlinuxbr

    ## docker
    if [[ -z "${DOCKER_NETWORK_OPTIONS}" ]]
    then
        DOCKER_NETWORK_OPTIONS='-b=lbr0 --mtu=1450'
    fi

    mkdir -p /run/openshift-sdn
    cat <<EOF > /run/openshift-sdn/docker-network
# This file has been modified by openshift-sdn. Please modify the
# DOCKER_NETWORK_OPTIONS variable in /etc/sysconfig/openshift-node if this
# is an integrated install or /etc/sysconfig/openshift-sdn-node if this is a
# standalone install.

DOCKER_NETWORK_OPTIONS='${DOCKER_NETWORK_OPTIONS}'
EOF
}

function setup() {
    # clear config file
    rm -f /etc/openshift-sdn/config.env

    ## openvswitch
    ovs-vsctl del-br br0 || true
    ovs-vsctl add-br br0 -- set Bridge br0 fail-mode=secure
    ovs-vsctl set bridge br0 protocols=OpenFlow13
    ovs-vsctl del-port br0 vxlan0 || true
    ovs-vsctl add-port br0 vxlan0 -- set Interface vxlan0 type=vxlan options:remote_ip="flow" options:key="flow" ofport_request=1
    ovs-vsctl add-port br0 ${TUN} -- set Interface ${TUN} type=internal ofport_request=2

    ovs-vsctl del-port br0 vovsbr || true
    ovs-vsctl add-port br0 vovsbr -- set Interface vovsbr ofport_request=9

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
    fwd_lineno=$(iptables -nvL FORWARD --line-numbers | grep "reject-with icmp-host-prohibited" | tail -n 1 | awk '{print $1}')
    iptables -I FORWARD $fwd_lineno -d ${cluster_subnet} -j ACCEPT
    iptables -I FORWARD $fwd_lineno -s ${cluster_subnet} -j ACCEPT

    # disable iptables for lbr0
    # for kernel version 3.18+, module br_netfilter needs to be loaded upfront
    # for older ones, br_netfilter may not exist, but is covered by bridge (bridge-utils)
    modprobe br_netfilter || true 
    sysctl -w net.bridge.bridge-nf-call-iptables=0

    # delete the subnet routing entry created because of lbr0
    ip route del ${subnet} dev lbr0 proto kernel scope link src ${subnet_gateway} || true

    mkdir -p /etc/openshift-sdn
    echo "export OPENSHIFT_SDN_TAP1_ADDR=${tun_gateway}" >& "/etc/openshift-sdn/config.env"
    echo "export OPENSHIFT_CLUSTER_SUBNET=${cluster_subnet}" >> "/etc/openshift-sdn/config.env"
}

set +e
if docker_setup_required; then
    lockwrap docker_setup
    systemctl daemon-reload
    systemctl restart docker.service
fi
set -e

lockwrap setup

