#!/bin/bash

set -ex

lock_file=/var/lock/openshift-sdn.lock
subnet_gateway=$1
subnet=$2
cluster_subnet=$3
subnet_mask_len=$4
tun_gateway=$5
mtu=$6
printf 'Container network is "%s"; local host has subnet "%s", mtu "%d" and gateway "%s".\n' "${cluster_subnet}" "${subnet}" "${mtu}" "${subnet_gateway}"
TUN=tun0

# Synchronize code execution with a file lock.
function lockwrap() {
    (
    flock 200
    "$@"
    ) 200>${lock_file}
}

function docker_network_config() {
    if [ -z "${DOCKER_NETWORK_OPTIONS}" ]; then
	DOCKER_NETWORK_OPTIONS="-b=lbr0 --mtu=${mtu}"
    fi

    case "$1" in
	check)
	    if [ -f /.dockerinit ]; then
		# Assume supervisord-managed docker for docker-in-docker deployments
		conf=/etc/supervisord.conf
		if ! grep -q -s "DOCKER_DAEMON_ARGS=\"${DOCKER_NETWORK_OPTIONS}\"" $conf; then
		    return 1
		fi
	    else
		# Otherwise assume systemd-managed docker
		conf=/run/openshift-sdn/docker-network
		if ! grep -q -s "DOCKER_NETWORK_OPTIONS='${DOCKER_NETWORK_OPTIONS}'" $conf; then
		    return 1
		fi
	    fi
	    return 0
	    ;;

	update)
	    if [ -f /.dockerinit ]; then
		conf=/etc/supervisord.conf
		if [ ! -f $conf ]; then
		    echo "Running in docker but /etc/supervisord.conf not found." >&2
		    exit 1
		fi

		echo "Docker networking options have changed; manual restart required." >&2
		sed -i.bak -e \
		    "s+\(DOCKER_DAEMON_ARGS=\)\"\"+\1\"${DOCKER_NETWORK_OPTIONS}\"+" \
		    $conf
	    else
		mkdir -p /run/openshift-sdn
		cat <<EOF > /run/openshift-sdn/docker-network
# This file has been modified by openshift-sdn.

DOCKER_NETWORK_OPTIONS='${DOCKER_NETWORK_OPTIONS}'
EOF

		systemctl daemon-reload
		systemctl restart docker.service

		# disable iptables for lbr0
		# for kernel version 3.18+, module br_netfilter needs to be loaded upfront
		# for older ones, br_netfilter may not exist, but is covered by bridge (bridge-utils)
		#
		# This operation is assumed to have been performed in advance
		# for docker-in-docker deployments.
		modprobe br_netfilter || true
		sysctl -w net.bridge.bridge-nf-call-iptables=0
	    fi
	    ;;
    esac
}

function setup_required() {
    ip=$(echo `ip a s lbr0 2>/dev/null|awk '/inet / {print $2}'`)
    if [ "$ip" != "${subnet_gateway}/${subnet_mask_len}" ]; then
        return 0
    fi
    if ! docker_network_config check; then
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
    docker_network_config update

    # Cleanup docker0 since docker won't do it
    ip link set docker0 down || true
    brctl delbr docker0 || true

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
