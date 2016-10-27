#!/bin/bash


#  Includes.
mydir=$(dirname "${BASH_SOURCE[0]}")
source "$mydir/../conf/settings.sh"
source "$mydir/utils.sh"
source "$mydir/config-generators.sh"

#  Constants.
readonly KEEPALIVED_CONFIG="/etc/keepalived/keepalived.conf"
readonly KEEPALIVED_DEFAULTS="/etc/sysconfig/keepalived"


function cleanup() {
  echo "  - Cleaning up ... "
  [ -n "$1" ] && kill -TERM $1

  local interface=$(get_network_device "$NETWORK_INTERFACE")
  local vips=$(expand_ip_ranges "$HA_VIPS")
  echo "  - Releasing VIPs ${vips} (interface ${interface}) ... "

  local regex='^.*?/[0-9]+$'

  for vip in ${vips}; do
    echo "  - Releasing VIP ${vip} ... "
    if [[ ${vip} =~ ${regex} ]] ; then
      ip addr del ${vip} dev ${interface} || :
    else
      ip addr del ${vip}/32 dev ${interface} || :
    fi
  done

  exit 0
}


function setup_failover() {
  echo "  - Loading ip_vs module ..."
  modprobe ip_vs

  echo "  - Checking if ip_vs module is available ..."
  if lsmod | grep '^ip_vs'; then
    echo "  - Module ip_vs is loaded."
  else
    echo "ERROR: Module ip_vs is NOT available."
  fi

  # When the DC supplies an (non null) iptables chain
  # (OPENSHIFT_HA_IPTABLES_CHAIN) make sure the rule to pass keepalived
  # multicast (224.0.0.18) is in the table.
  chain="${OPENSHIFT_HA_IPTABLES_CHAIN:-""}"
  if [[ -n ${chain} ]]; then
    echo "  - check for iptables rule for keepalived multicast (224.0.0.18) ..."
    if ! iptables -S | grep 224.0.0.18 > /dev/null 2>&1 ; then
      # Add the rule to the beginning of the chain.
      echo "  - adding iptables rule to $chain to access 224.0.0.18."
      iptables -I ${chain} 1 -d 224.0.0.18/32 -j ACCEPT
    fi
  fi

  echo "  - Generating and writing config to $KEEPALIVED_CONFIG"
  generate_failover_config > "$KEEPALIVED_CONFIG"
}


function start_failover_services() {
  echo "  - Starting failover services ..."

  [ -f "$KEEPALIVED_DEFAULTS" ] && source "$KEEPALIVED_DEFAULTS"

  killall -9 /usr/sbin/keepalived &> /dev/null || :
  /usr/sbin/keepalived $KEEPALIVED_OPTIONS -n --log-console &
  local pid=$!

  trap "cleanup ${pid}" SIGHUP SIGINT SIGTERM
  wait ${pid}
}

