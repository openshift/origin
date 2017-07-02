#!/bin/bash


#  ========================================================================
#  Settings passed by the failover coordinator on OpenShift Origin.
#  ========================================================================

#  Name of this IP Failover config instance.
HA_CONFIG_NAME="${OPENSHIFT_HA_CONFIG_NAME:-"OpenShift-IPFailover"}"

#  IP Failover config selector.
HA_SELECTOR="${OPENSHIFT_HA_SELECTOR:-""}"


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
HA_VIPS="${OPENSHIFT_HA_VIRTUAL_IPS:-""}"


#  Interface (ethernet) to use - bound by vrrp.
NETWORK_INTERFACE="${OPENSHIFT_HA_NETWORK_INTERFACE:-""}"  # "enp0s8"


#  Service port to monitor for failover.
HA_MONITOR_PORT="${OPENSHIFT_HA_MONITOR_PORT:-"80"}"

#  Number of initial replicas.
HA_REPLICA_COUNT="${OPENSHIFT_HA_REPLICA_COUNT:-"1"}"


#  Offset value to use to set the virtual router ids. Using different offset
#  values allows multiple ipfailover configurations to exist within the
#  same cluster. Range 1..255
#     HA_VRRP_ID_OFFSET=30
#
HA_VRRP_ID_OFFSET="${OPENSHIFT_HA_VRRP_ID_OFFSET:-"0"}"

# When the DC supplies an (non null) iptables chain
# (OPENSHIFT_HA_IPTABLES_CHAIN) make sure the rule to pass keepalived
# multicast (224.0.0.18) traffic is in the table.
HA_IPTABLES_CHAIN="${OPENSHIFT_HA_IPTABLES_CHAIN:-""}"

# Optional external check script that is run every HA_CHECK_INTERVAL seconds
# The script can test whatever is needed to verify the application is running.
# Must return 0 -- OK, or 1 -- fail
# This script is in addition to the default check that the port is listening.
# The script must be accessible from inside the keepalived pod
HA_CHECK_SCRIPT="${OPENSHIFT_HA_CHECK_SCRIPT:-""}"

# Optional notify script is called when a state transition occurs
# Transition to MASTER, or to BACKUP, or to FAULT
# The parameters to the script are passed in by keepalived: 
#  $1 - "GROUP"|"INSTANCE"
#  $2 - name of group or instance
#  $3 - target state of transition ("MASTER"|"BACKUP"|"FAULT")
# The script must be accessible from inside the keepalived pod
HA_NOTIFY_SCRIPT="${OPENSHIFT_HA_NOTIFY_SCRIPT:-""}"

# The check script is run every HA_CHECK_INTERVAL seconds.
# Default is 2
HA_CHECK_INTERVAL="${OPENSHIFT_HA_CHECK_INTERVAL:-"2"}"

#  VRRP will preempt a lower priority machine when a higher priority one
#  comes back online. You can change the preemption strategy to either:
#     "nopreempt"  - which allows the lower priority machine to maintain its
#                    'MASTER' status.
#     OR
#     "preempt_delay 300"  - waits 5 mins (in seconds) after startup to
#                            preempt lower priority MASTERs.
PREEMPTION="${OPENSHIFT_HA_PREEMPTION:-"preempt_delay 300"}"


#  ========================================================================
#  Default settings - not currently exposed or overridden on OpenShift.
#  ========================================================================

#  If your environment doesn't support multicast, you can send VRRP adverts
#  to a list of IPv{4,6} addresses using unicast.
#  Example:
#     UNICAST_PEERS="5.6.7.8,9.10.11.12,13.14.15.16"
UNICAST_PEERS="${OPENSHIFT_HA_UNICAST_PEERS:-""}"


#  List of emails to send admin messages to. If the list of email ids is
#  too long, you can use a DL (distribution list) ala:
#   ADMIN_EMAILS=("ramr@redhat.com" "cops@acme.org")
ADMIN_EMAILS=(${OPENSHIFT_HA_ADMIN_EMAILS:-"root@localhost"})

#  Email sender - the from address in the email headers.
EMAIL_FROM="ipfailover@openshift.local"

#  IP address of the SMTP server.
SMTP_SERVER="${OPENSHIFT_HA_SMTP_SERVER:-"127.0.0.1"}"

#  SMTP connect timeout (in seconds).
SMTP_CONNECT_TIMEOUT=30

#  By default, the IP for binding vrrpd is the primary IP on the above
#  specified interface. If you want to hide the location of vrrpd, you can
#  specify a src_addr for multicast/unicast vrrp packets.
#     MULTICAST_SOURCE_IPADDRESS="1.2.3.4"
#     UNICAST_SOURCE_IPADDRESS="1.2.3.4"

