#!/bin/bash -x

# This NetworkManager dispatcher script replicates the functionality of
# NetworkManager's dns=dnsmasq  however, rather than hardcoding the listening
# address and /etc/resolv.conf to 127.0.0.1 it pulls the IP address from the
# interface that owns the default route. This enables us to then configure pods
# to use this IP address as their only resolver, where as using 127.0.0.1 inside
# a pod would fail.
#
# To use this,
# Drop this script in /etc/NetworkManager/dispatcher.d/
# systemctl restart NetworkManager
# Configure node-config.yaml to set dnsIP: to the ip address of this
# node
#
# Test it:
# host kubernetes.default.svc.cluster.local
# host google.com
#
# TODO: I think this would be easy to add as a config option in NetworkManager
# natively, look at hacking that up

cd /etc/sysconfig/network-scripts
. ./network-functions

[ -f ../network ] && . ../network

if [[ $2 =~ ^(up|dhcp4-change)$ ]]; then
  # couldn't find an existing method to determine if the interface owns the 
  # default route
  def_route=$(/sbin/ip route list match 0.0.0.0/0 | awk '{print $3 }')
  def_route_int=$(/sbin/ip route get to ${def_route} | awk '{print $3}')
  def_route_ip=$(/sbin/ip route get to ${def_route} | awk '{print $5}')
  if [[ ${DEVICE_IFACE} == ${def_route_int} && \
       -n "${IP4_NAMESERVERS}" ]]; then
    if [ ! -f /etc/dnsmasq.d/origin-dns.conf ]; then
      cat << EOF > /etc/dnsmasq.d/origin-dns.conf
strict-order
no-resolv
domain-needed
server=/cluster.local/172.30.0.1
server=/30.172.in-addr.arpa/172.30.0.1
EOF
    fi
    # zero out our upstream servers list and feed it into dnsmasq
    echo -n > /etc/dnsmasq.d/origin-upstream-dns.conf
    for ns in ${IP4_NAMESERVERS}; do
       echo "server=${ns}" >> /etc/dnsmasq.d/origin-upstream-dns.conf
    done
    systemctl restart dnsmasq

    sed -i 's/^nameserver.*$/nameserver '"${def_route_ip}"'/g' /etc/resolv.conf
    echo "# nameserver updated by /etc/NetworkManager/dispatcher.d/99-origin-dns.sh" >> /etc/resolv.conf
  fi
fi
