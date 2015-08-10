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

function setup_required() {
    ip=$(echo `ip a s lbr0 2>/dev/null|awk '/inet / {print $2}'`)
    if [ "$ip" != "${subnet_gateway}/${subnet_mask_len}" ]; then
        return 0
    fi
    if ! grep -q lbr0 /run/openshift-sdn/docker-network; then
        return 0
    fi
    return 1
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

    ip link del vlinuxbr || true
    ip link add vlinuxbr type veth peer name vovsbr
    ip link set vlinuxbr up
    ip link set vovsbr up
    ip link set vlinuxbr txqueuelen 0
    ip link set vovsbr txqueuelen 0

    ovs-vsctl del-port br0 vovsbr || true
    ovs-vsctl add-port br0 vovsbr -- set Interface vovsbr ofport_request=9

    # Table 0; learn MAC addresses and continue with table 1
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=0, actions=learn(table=7, priority=200, hard_timeout=900, NXM_OF_ETH_DST[]=NXM_OF_ETH_SRC[], load:NXM_NX_TUN_IPV4_SRC[]->NXM_NX_TUN_IPV4_DST[], output:NXM_OF_IN_PORT[]), goto_table:1"

    # Table 1; initial dispatch
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=1, arp, actions=goto_table:7"
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=1, in_port=1, actions=goto_table:2" # vxlan0
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=1, in_port=2, actions=goto_table:4" # tun0
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=1, in_port=9, actions=goto_table:4" # vovsbr
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=1, actions=goto_table:3"            # container

    # Table 2; incoming from vxlan
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=2, arp, actions=goto_table:7"
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=2, priority=200, ip, nw_dst=${subnet_gateway}, actions=output:2"
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=2, tun_id=0, actions=goto_table:4"
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=2, priority=100, ip, nw_dst=${subnet}, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[], goto_table:5"

    # Table 3; incoming from container; filled in by openshift-ovs-multitenant

    # Table 4; general routing
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=4, priority=200, ip, nw_dst=${subnet_gateway}, actions=output:2"
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=4, priority=150, ip, nw_dst=${subnet}, actions=goto_table:5"
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=4, priority=100, ip, nw_dst=${cluster_subnet}, actions=goto_table:6"
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=4, priority=0, ip, actions=output:2"

    # Table 5; to local container; mostly filled in by openshift-ovs-multitenant
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=5, priority=200, ip, reg0=0, actions=goto_table:7"

    # Table 6; to remote container; filled in by multitenant.go

    # Table 7; MAC dispatch / ARP, filled in by Table 0's learn() rule
    # and with per-node vxlan ARP rules by multitenant.go
    ovs-ofctl -O OpenFlow13 add-flow br0 "table=7, priority=0, arp, actions=flood"

    ## linux bridge
    ip link set lbr0 down || true
    brctl delbr lbr0 || true
    brctl addbr lbr0
    ip addr add ${subnet_gateway}/${subnet_mask_len} dev lbr0
    ip link set lbr0 up
    brctl addif lbr0 vlinuxbr

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

    systemctl daemon-reload
    systemctl restart docker.service

    # disable iptables for lbr0
    # for kernel version 3.18+, module br_netfilter needs to be loaded upfront
    # for older ones, br_netfilter may not exist, but is covered by bridge (bridge-utils)
    modprobe br_netfilter || true 
    sysctl -w net.bridge.bridge-nf-call-iptables=0

    # enable IP forwarding for ipv4 packets
    sysctl -w net.ipv4.ip_forward=1
    sysctl -w net.ipv4.conf.${TUN}.forwarding=1

    # delete the subnet routing entry created because of lbr0
    ip route del ${subnet} dev lbr0 proto kernel scope link src ${subnet_gateway} || true

    mkdir -p /etc/openshift-sdn
    echo "export OPENSHIFT_CLUSTER_SUBNET=${cluster_subnet}" >> "/etc/openshift-sdn/config.env"
}

set +e
if ! setup_required; then
    echo "SDN setup not required."
    exit 140
fi
set -e

lockwrap setup
