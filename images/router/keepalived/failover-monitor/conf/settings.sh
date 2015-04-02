#!/bin/bash


#  ========================================================================
#  Settings passed by the router on OpenShift Origin.
#  ========================================================================

#  Name of this router instance.
ROUTER_NAME=${OPENSHIFT_ROUTER_NAME:-"OpenShift-Router"}

#  Number of router replicas.
NUM_REPLICAS=${OPENSHIFT_ROUTER_HA_REPLICA_COUNT:-"1"}


#  List of virtual IP addresses.
#
#  The value entries are comma-separated entries of the form:
#     <ipaddress-range|ipaddress>
#
#     where:  ipaddress-range = <start-ipaddress>-<endip>
#
#  Example:
#     OPENSHIFT_ROUTER_HA_VIRTUAL_IPS="10.42.42.42,10.100.1.20-24"
#
ROUTER_VIPS=${OPENSHIFT_ROUTER_HA_VIRTUAL_IPS:-""}


#  Interface (ethernet) to use - bound by vrrp.
NETWORK_INTERFACE=${OPENSHIFT_ROUTER_HA_NETWORK_INTERFACE:-""}  # "enp0s8"


#  If your environment doesn't support multicast, you can send VRRP adverts
#  to a list of IPv{4,6} addresses using unicast.
#  Example:
#     UNICAST_PEERS="5.6.7.8,9.10.11.12,13.14.15.16"
UNICAST_PEERS=${OPENSHIFT_ROUTER_HA_UNICAST_PEERS:-""}




#  ========================================================================
#  Default settings - not currently exposed or overriden on OpenShift.
#  ========================================================================

#  List of emails to send admin messages to. If the list of email ids is
#  too long, you can use a DL (distribution list) ala:
#   ADMIN_EMAILS=("ramr@redhat.com" "cops@acme.org")
ADMIN_EMAILS=("root@localhost")

#  Email sender - the from address in the email headers.
EMAIL_FROM="router-ha@localhost"

#  IP address of the SMTP server.
SMTP_SERVER="127.0.0.1"

#  SMTP connect timeout (in seconds).
SMTP_CONNECT_TIMEOUT=30

#  Password for accessing vrrpd - only first 8 characters are used.
VRRPD_PASS=${ROUTER_NAME:-"OS-RouteR"}


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

