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
#  Tests if an IPv4 address is valid
#  Echos true (0) if valid, false (1) if invalid 
#
#  Examples:
#     validate_ipv4 192.0.2.3
#         # -> 0 
#
#     validate_ipv4 192.0.3.4.0 
#         # -> 1 
#
#     validate_ipv4 192.0.2 
#         # -> 1 
#
function validate_ipv4() {
  local IPv4_GROUP="[0-2]?[0-9]{1,2}"
  local IPv4_SHAPE="^(${IPv4_GROUP}\.){3,3}(${IPv4_GROUP})$"
  local is_valid=0
  local i

  if [[ ${1} =~ ${IPv4_SHAPE} ]]; then
    for i in $(echo ${1} | tr "." " "); do
      if [ $i -gt 255 ]; then
        is_valid=1
      fi
    done
  else
    is_valid=1
  fi
  return ${is_valid}
}

#
#  Tests if an IPv6 address is valid
#  Returns true (0) if valid, false (1) if invalid 
#
#  Examples:
#     validate_ipv6 2001:DB8:1:E32:FFFF:3:19:39FB 
#         # -> 0 
#
#     validate_ipv6 2001:DB8::39FB 
#         # -> 0 
#
#     validate_ipv6 2001::DB8::39FB 
#         # -> 1 
#
function validate_ipv6() {
  local IPv6_GROUP="[[:xdigit:]]{1,4}"
  local IPv6_SHAPE1="^(${IPv6_GROUP}:){7,7}(${IPv6_GROUP})$"
  local IPv6_SHAPE2="^::(${IPv6_GROUP}:){0,6}(${IPv6_GROUP})$"
  local IPv6_SHAPE3="^(${IPv6_GROUP}:){0,6}:(${IPv6_GROUP}:){0,6}(${IPv6_GROUP})$"
  local VALID_SHAPE3="^(${IPv6_GROUP}:+){1,6}(${IPv6_GROUP})$"

  local is_valid=1

  if [[ ${1} =~ ${IPv6_SHAPE1} ]]; then
    is_valid=0
  fi

  if [[ ${1} =~ ${IPv6_SHAPE2} ]]; then
    is_valid=0
  fi
   
  if [[ ${1} =~ ${IPv6_SHAPE3} ]]; then
    if [[ ${1} =~ ${VALID_SHAPE3} ]]; then
      is_valid=0
    fi
  fi
  return ${is_valid}
}

#
#  Expands list of IPv4 addresses. List elements can be an IP address
#  range or an IP address.
#
#  Examples:
#     expand_ipv4_range "3.3.3.3-4"
#         # -> 3.3.3.3 3.3.3.4
#
#     expand_ipv4_range "10.1.1.100-100"
#         # -> 10.1.1.100
#
#     expand_ipv4_range "10.1.1.100"
#         # -> 10.1.1.100
#
function expand_ipv4_range() {
  local expandedset=()
  local ip1=$(echo "$1" | awk '{print $1}' FS='-')
  local ip2=$(echo "$1" | awk '{print $2}' FS='-')
  local n 

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
  echo "${expandedset[@]}"
}

#
#  Expands list of IPv6 addresses. List elements can be an IP address
#  range or an IP address.
#
#  Examples:
#     expand_ipv6_range "2001:DB8:1ABC::1F39-1F3B"
#         # -> 2001:DB8:1ABC::1F39 2001:DB8:1ABC::1F3A 2001:DB8:1ABC::1F3B
#
function expand_ipv6_range() {
  local expandedset=()
  local ip1=$(echo "$1" | awk '{print $1}' FS='-')
  local ip2=$(echo "$1" | awk '{print $2}' FS='-')
  local n
  if [ -z "$ip2" ]; then
    expandedset=(${expandedset[@]} "$ip1")
  else
    local start=${ip1##*:}
    local decstart=`echo "ibase=16; ${start}" | bc`
    local base=${ip1%%${start}}
    local decend=`echo "ibase=16; ${ip2}" | bc`
    for n in `seq $decstart $decend`; do
      end=`echo "obase=16; ${n}" | bc`
      expandedset=(${expandedset[@]} "${base}${end}")
    done
  fi
  echo "${expandedset[@]}"
}

#
#  Returns the IP address family (IPv4 or IPv6)
#  Returns "4" or "6" respectively 
#  
#  Examples: 
#    get_address_family "192.168.3.1"
#         # -> 4
#
#    get_address_family "2001:DB8:1ABC::1F3A"
#         # -> 6
#
function get_address_family() {
  if validate_ipv4 ${1}; then
    return 4
  elif validate_ipv6 ${1}; then
    return 6
  else
    return 1
  fi
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
  local iprange
  local newip

  for iprange in $(echo "$vips" | sed 's/[^0-9a-fA-F:\.,-]//g' | tr "," " "); do
    local ip1=$(echo "$iprange" | awk '{print $1}' FS='-')
    get_address_family ${ip1}
    local family=$?
    if [ ${family} == "4" ]; then
      for newip in $(expand_ipv4_range ${iprange}); do
        expandedset=(${expandedset[@]} ${newip})
      done
    elif [ ${family} == "6" ]; then
      for newip in $(expand_ipv6_range ${iprange}); do
        expandedset=(${expandedset[@]} ${newip})
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
