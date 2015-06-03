#!/bin/bash

#  TODO: This follows the initial demo pieces and uses a bash script to
#        generate the keepalived config - rework this into a template
#        similar to how it is done for the haproxy configuration.

#  Includes.
source "$(dirname "${BASH_SOURCE[0]}")/utils.sh"


# Constants.
readonly CHECK_SCRIPT_NAME="chk_${HA_CONFIG_NAME//-/_}"
readonly CHECK_INTERVAL_SECS=2
readonly VRRP_SLAVE_PRIORITY=42

readonly DEFAULT_PREEMPTION_STRATEGY="preempt_delay 300"


#
#  Generate global config section.
#
#  Example:
#     generate_global_config  arparp
#
function generate_global_config() {
  local routername=$(scrub "$1")

  echo "global_defs {"
  echo "   notification_email {"

  for email in ${ADMIN_EMAILS[@]}; do
    echo "     $email"
  done

  echo "   }"
  echo ""
  echo "   notification_email_from ${EMAIL_FROM:-"ipfailover@openshift.local"}"
  echo "   smtp_server ${SMTP_SERVER:-"127.0.0.1"}"
  echo "   smtp_connect_timeout ${SMTP_CONNECT_TIMEOUT:-"30"}"
  echo "   router_id $routername"
  echo "}"
}


#
#  Generate VRRP checker script configuration section.
#
#  Example:
#      generate_script_config
#      generate_script_config "10.1.2.3" 8080
#
function generate_script_config() {
  local serviceip=${1:-"127.0.0.1"}
  local port=${2:-80}

  echo ""
  echo "vrrp_script $CHECK_SCRIPT_NAME {"

  if [ "$port" = "0" ]; then
    echo "   script \"true\""
  else
    echo "   script \"</dev/tcp/${serviceip}/${port}\""
  fi

  echo "   interval $CHECK_INTERVAL_SECS"
  echo "}"
}


#
#  Generate authentication information section.
#
#  Example:
#      generate_authentication_info
#
function generate_authentication_info() {
  local creds=${1:-"R0ut3r"}
  echo ""
  echo "   authentication {"
  echo "      auth_type PASS"
  echo "      auth_pass $creds"
  echo "   }"
}


#
#  Generate track script section.
#
#  Example:
#      generate_track_script
#
function generate_track_script() {
  echo ""
  echo "   track_script {"
  echo "      $CHECK_SCRIPT_NAME"
  echo "   }"
}


#
#  Generate multicast + unicast options section based on the values of the
#  MULTICAST_SOURCE_IPADDRESS, UNICAST_SOURCE_IPADDRESS and UNICAST_PEERS
#  environment variables.
#
#  Examples:
#      generate_mucast_options
#
#      UNICAST_SOURCE_IPADDRESS=10.1.1.1 UNICAST_PEERS="10.1.1.2,10.1.1.3" \
#          generate_mucast_options
#
function generate_mucast_options() {
  echo ""

  if [ -n "$MULTICAST_SOURCE_IPADDRESS" ]; then
    echo "    mcast_src_ip $MULTICAST_SOURCE_IPADDRESS"
  fi

  if [ -n "$UNICAST_SOURCE_IPADDRESS" ]; then
    echo "    unicast_src_ip $UNICAST_SOURCE_IPADDRESS"
  fi

  if [ -n "$UNICAST_PEERS" ]; then
    echo ""
    echo "    unicast_peer {"

    for ip in $(echo "$UNICAST_PEERS" | tr "," " "); do
      echo "        $ip"
    done

    echo "    }"
  fi
}


#
#  Generate VRRP sync groups section.
#
#  Examples:
#      generate_vrrp_sync_groups "ipf-1" "10.1.1.1 10.1.2.2"
#
#      generate_vrrp_sync_groups "arparp" "10.42.42.42-45, 10.9.1.1"
#
function generate_vrrp_sync_groups() {
  local servicename=$(scrub "$1")

  echo ""
  echo "vrrp_sync_group group_${servicename} {"
  echo "   group {"

  local prefix="$(vrrp_instance_basename "$1")"
  local counter=1

  for ip in $(expand_ip_ranges "$2"); do
    echo "      ${prefix}_${counter}   # VIP $ip"
    counter=$((counter + 1))
  done

  echo "   }"
  echo "}"
}


#
#  Generate virtual ip address section.
#
#  Examples:
#      generate_vip_section "10.245.2.3" "enp0s8"
#
#      generate_vip_section "10.1.1.1 10.1.2.2" "enp0s8"
#
#      generate_vip_section "10.42.42.42-45, 10.9.1.1"
#
function generate_vip_section() {
  local interface=${2:-"$(get_network_device)"}

  echo ""
  echo "   virtual_ipaddress {"

  for ip in $(expand_ip_ranges "$1"); do
    echo "      ${ip} dev $interface"
  done

  echo "   }"
}


#
#  Generate vrrpd instance configuration section.
#
#  Examples:
#      generate_vrrpd_instance_config arp 1 "10.1.2.3" enp0s8 "252" "master"
#
#      generate_vrrpd_instance_config arp 1 "10.1.2.3" enp0s8 "3" "slave"
#
#      generate_vrrpd_instance_config ipf-1 4 "10.1.2.3-4" enp0s8 "7"
#
function generate_vrrpd_instance_config() {
  local servicename=$1
  local iid=${2:-"0"}
  local vips=$3
  local interface=$4
  local priority=${5:-"10"}
  local instancetype=${6:-"slave"}

  local vipname=$(scrub "$1")
  local initialstate=""
  local preempt=${PREEMPTION:-"$DEFAULT_PREEMPTION_STRATEGY"}

  [ "$instancetype" = "master" ] && initialstate="state MASTER"

  local instance_name=$(generate_vrrp_instance_name "$servicename" "$iid")

  local auth_section=$(generate_authentication_info "$servicename")
  local vip_section=$(generate_vip_section "$vips" "$interface")
  echo "
vrrp_instance ${instance_name} {
   interface ${interface}
   ${initialstate}
   virtual_router_id ${iid}
   priority ${priority}
   ${preempt}
   ${auth_section}
   $(generate_track_script)
   $(generate_mucast_options)
   ${vip_section}
}
"

}


#
#  Generate failover configuration.
#
#  Examples:
#      generate_failover_configuration
#
function generate_failover_config() {
  local vips=$(expand_ip_ranges "$HA_VIPS")
  local interface=$(get_network_device "$NETWORK_INTERFACE")
  local ipaddr=$(get_device_ip_address "$interface")
  local port=$(echo "$HA_MONITOR_PORT" | sed 's/[^0-9]//g')

  echo "! Configuration File for keepalived

$(generate_global_config "$HA_CONFIG_NAME")
$(generate_script_config "$ipaddr" "$port")
$(generate_vrrp_sync_groups "$HA_CONFIG_NAME" "$vips")
"

  local ipkey=$(echo "$ipaddr" | cut -f 4 -d '.')
  local ipslot=$((ipkey % 128))

  local nodecount=$(($HA_REPLICA_COUNT > 0 ? $HA_REPLICA_COUNT : 1))
  local idx=$((ipslot % $nodecount))
  idx=$((idx + 1))

  local counter=1
  local previous="none"

  for vip in ${vips}; do
    local offset=$((RANDOM % 32))
    local priority=$(($((ipslot % 64)) + $offset))
    local instancetype="slave"
    local n=$((counter % $idx))

    if [ $n -eq 0 ]; then
      instancetype="master"
      if [ "$previous" = "master" ]; then
        #  Inverse priority + reset, so that we can flip-flop priorities.
        priority=$((ipslot + 1))
        previous="flip-flop"
      else
        priority=$((255 - $ipslot))
        previous=$instancetype
      fi
    fi

    generate_vrrpd_instance_config "$HA_CONFIG_NAME" "$counter" "$vip"  \
        "$interface" "$priority" "$instancetype"

    counter=$((counter + 1))
  done
}

