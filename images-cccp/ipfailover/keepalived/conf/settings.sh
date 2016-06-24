#!/bin/bash


#  ========================================================================
#  Settings passed by the failover coordinator on OpenShift Origin.
#  ========================================================================

#  Name of this IP Failover config instance.
HA_CONFIG_NAME=${OPENSHIFT_HA_CONFIG_NAME:-"OpenShift-IPFailover"}

#  IP Failover config selector.
HA_SELECTOR=${OPENSHIFT_HA_SELECTOR:-""}


#  List of virtual IP addresses.
#
#  The value entries are comma-separated entries of the form:
#     <ipaddress-range|ipaddress>
#
#     where:  ipaddress-range = <start-ipaddress>-<endip>
#
#  Example:
#     OPENSHIFT_HA_VIRTUAL_IPS="10.42.42.42,10.100.1.20-24"
#
HA_VIPS=${OPENSHIFT_HA_VIRTUAL_IPS:-""}


#  Interface (ethernet) to use - bound by vrrp.
NETWORK_INTERFACE=${OPENSHIFT_HA_NETWORK_INTERFACE:-""}  # "enp0s8"


#  Service port to monitor for failover.
HA_MONITOR_PORT=${OPENSHIFT_HA_MONITOR_PORT:-"80"}

#  Number of initial replicas.
HA_REPLICA_COUNT=${OPENSHIFT_HA_REPLICA_COUNT:-"1"}


#  Offset value to use to set the virtual router ids. Using different offset
#  values allows multiple ipfailover configurations to exist within the
#  same cluster.
#     HA_VRRP_ID_OFFSET=30
#
HA_VRRP_ID_OFFSET=${OPENSHIFT_HA_VRRP_ID_OFFSET:-"0"}



#  ========================================================================
#  Default settings - not currently exposed or overriden on OpenShift.
#  ========================================================================

#  If your environment doesn't support multicast, you can send VRRP adverts
#  to a list of IPv{4,6} addresses using unicast.
#  Example:
#     UNICAST_PEERS="5.6.7.8,9.10.11.12,13.14.15.16"
UNICAST_PEERS=${OPENSHIFT_HA_UNICAST_PEERS:-""}


#  List of emails to send admin messages to. If the list of email ids is
#  too long, you can use a DL (distribution list) ala:
#   ADMIN_EMAILS=("ramr@redhat.com" "cops@acme.org")
ADMIN_EMAILS=(${OPENSHIFT_HA_ADMIN_EMAILS:-"root@localhost"})

#  Email sender - the from address in the email headers.
EMAIL_FROM="ipfailover@openshift.local"

#  IP address of the SMTP server.
SMTP_SERVER=${OPENSHIFT_HA_SMTP_SERVER:-"127.0.0.1"}

#  SMTP connect timeout (in seconds).
SMTP_CONNECT_TIMEOUT=30


#  VRRP will preempt a lower priority machine when a higher priority one
#  comes back online. You can change the preemption strategy to either:
#     "nopreempt"  - which allows the lower priority machine to maintain its
#                    'MASTER' status.
#     OR
#     "preempt_delay 300"  - waits 5 mins (in seconds) after startup to
#                            preempt lower priority MASTERs.
PREEMPTION="preempt_delay 300"


#  By default, the IP for binding vrrpd is the primary IP on the above
#  specified interface. If you want to hide the location of vrrpd, you can
#  specify a src_addr for multicast/unicast vrrp packets.
#     MULTICAST_SOURCE_IPADDRESS="1.2.3.4"
#     UNICAST_SOURCE_IPADDRESS="1.2.3.4"

