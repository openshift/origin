#!/bin/bash


#  Constants.
LIB_DIR=$(dirname "${BASH_SOURCE[0]}")
VBOX_INTERFACES="enp0s3 enp0s8 eth1"


#
#  Returns "scrubbed" name - removes characters that are not alphanumeric or
#  underscore and replacing dashes with underscores.
#
#  Examples:
#      scrub "config\!@#@$%$^&*()-+=1_{}|[]\\:;'<>?,./ipfailover"
#         # -> config_1_ipfailover
#
#      scrub "ha-1"  # -> ha_1
#
function scrub() {
  local val=$(echo "$1" | tr -dc '[:alnum:]\-_')
  echo "${val//-/_}"
}


#
#  Expands list of virtual IP addresses. List elements can be an IP address
#  range or an IP address and elements can be space or comma separated.
#
#  Examples:
#     expand_ip_ranges "1.1.1.1, 2.2.2.2,3.3.3.3-4  4.4.4.4"
#         # -> 1.1.1.1 2.2.2.2 3.3.3.3 3.3.3.4 4.4.4.4
#
#     expand_ip_ranges "10.1.1.100-102 10.1.1.200-200 10.42.42.42"
#         # -> 10.1.1.100 10.1.1.101 10.1.1.102 10.1.1.200 10.42.42.42
#
function expand_ip_ranges() {
  local vips=${1:-""}
  local expandedset=()

  for iprange in $(echo "$vips" | sed 's/[^0-9\.\,-]//g' | tr "," " "); do
    local ip1=$(echo "$iprange" | awk '{print $1}' FS='-')
    local ip2=$(echo "$iprange" | awk '{print $2}' FS='-')
    if [ -z "$ip2" ]; then
      expandedset=(${expandedset[@]} "$ip1")
    else
      local base=$(echo "$ip1" | cut -f 1-3 -d '.')
      local start=$(echo "$ip1" | awk '{print $NF}' FS='.')
      local end=$(echo "$ip2" | awk '{print $NF}' FS='.')
      for n in `seq $start $end`; do
        expandedset=(${expandedset[@]} "${base}.$n")
      done
    fi
  done

  echo "${expandedset[@]}"
}


#
#  Generate base name for the VRRP instance.
#
#  Examples:
#     vrrp_instance_basename "arp"   # -> arp_VIP
#
#     vrrp_instance_basename "ha-1"  # -> ha_1_VIP
#
function vrrp_instance_basename() {
  echo "$(scrub "$1")_VIP"
}


#
#  Generate VRRP instance name.
#
#  Examples:
#     generate_vrrp_instance_name arp 42  # -> arp_VIP_42
#
#     generate_vrrp_instance_name ha-1    # -> ha_1_VIP_0
#
function generate_vrrp_instance_name() {
  local iid=${2:-0}
  echo "$(vrrp_instance_basename "$1")_${iid}"
}


#
#  Returns the network device name to use for VRRP.
#
#  Examples:
#     get_network_device
#
#     get_network_device  "eth0"
#
function get_network_device() {
  for dev in $1 ${VBOX_INTERFACES}; do
    if ip addr show dev "$dev" &> /dev/null; then
      echo "$dev"
      return
    fi
  done

  ip route get 8.8.8.8 | awk '/dev/ { f=NR }; f && (NR-1 == f)' RS=" "
}


#
#  Returns the IP address associated with a network device.
#
#  Examples:
#     get_device_ip_address
#
#     get_device_ip_address  "docker0"
#
function get_device_ip_address() {
  local dev=${1:-"$(get_network_device)"}
  ifconfig "$dev" | awk '/inet / { print $2 }'
}
