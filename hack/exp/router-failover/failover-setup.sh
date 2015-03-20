#!/bin/bash

# Constants.
readonly SCRIPT_DIR=$(dirname "${BASH_SOURCE[0]}")
readonly PACKAGES="keepalived nc"

#  Sample and keepalived config files.
readonly SAMPLE_CONFIG="$SCRIPT_DIR/conf/settings.example"
readonly KEEPALIVED_CONFIG="/etc/keepalived/keepalived.conf"

#  Config values.
readonly OPENSHIFT_ROUTER="openshift-router"
readonly CHECK_SCRIPT_NAME="chk_${OPENSHIFT_ROUTER//-/_}"
readonly CHECK_INTERVAL_SECS=2
readonly VRRP_SLAVE_PRIORITY=42

readonly DEFAULT_SMTP_CONNECT_TIMEOUT=30
readonly DEFAULT_MAIL_FROM="router-ha@openshift.local"
readonly DEFAULT_SMTP_SERVER="127.0.0.1"
readonly DEFAULT_ROUTER_NAME="OpenShift-Router"
readonly DEFAULT_HA_ROUTER_ID=11
readonly DEFAULT_INTERFACE="eth0"
readonly DEFAULT_VRRPD_PASS="os3P4sS"
readonly DEFAULT_PREEMPTION_STRATEGY="preempt_delay 300"


function _install_packages() {
  echo "  - Checking/installing packages $PACKAGES ..."
  yum -y install $PACKAGES

}  #  End of function  _install_packages.


function _generate_global_config() {
  local emailfrom=${EMAIL_FROM:-"$DEFAULT_MAIL_FROM"}
  local smtpserver=${SMTP_SERVER:-"$DEFAULT_SMTP_SERVER"}
  local routername=${HA_ROUTER_NAME:-"$DEFAULT_ROUTER_NAME"}
  routername="${routername//-/_}"

  echo "global_defs {"

  echo "   notification_email {"

  for email in ${ADMIN_EMAILS[@]}; do
    echo "     $email"
  done

  echo "   }"
  echo ""
  echo "   notification_email_from $emailfrom"
  echo "   smtp_server $smtpserver"
  echo "   smtp_connect_timeout $DEFAULT_SMTP_CONNECT_TIMEOUT"
  echo "   router_id $routername"
  echo "}"

}  #  End of function  _generate_global_config.


function _generate_script_config() {
  echo "vrrp_script $CHECK_SCRIPT_NAME {"
  echo "   script \"pidof $OPENSHIFT_ROUTER\""
  echo "   interval $CHECK_INTERVAL_SECS"
  echo "}"

}  #  End of function  _generate_script_config.


function _generate_authentication_info() {
  local vrrpdpass=${VRRPD_PASS:-"$DEFAULT_VRRPD_PASS"}

  echo "   authentication {"
  echo "      auth_type PASS"
  echo "      auth_pass $vrrpdpass"
  echo "   }"

}  #  End of function  _generate_authentication_info.


function _generate_track_script() {
  echo "   track_script {"
  echo "      $CHECK_SCRIPT_NAME"
  echo "   }"

}  #  End of function  _generate_track_script.


function _get_mucast_options() {
  local ipopts=""

  [ -n "$MCAST_SRC_IP" ]   && echo "    mcast_src_ip $MCAST_SRC_IP"
  [ -n "$UNICAST_SRC_IP" ] && echo "    unicast_src_ip $UNICAST_SRC_IP"

  if [ -n "$UNICAST_PEER_IPS" ]; then
    echo ""
    echo "    unicast_peer {"

    for ip in $UNICAST_PEER_IPS; do
      echo "        $ip"
    done

    echo "    }"
  fi


}  #  End of function  _get_mucast_options.


function _get_virtual_ipaddresses() {
  local ips=$1
  local interface=${INTERFACE:-"$DEFAULT_INTERFACE"}

  echo "   virtual_ipaddress {"

  for ip in $ips; do
  echo "      $ip dev $interface"
  done

  echo "   }"

}  #  End of function  _get_virtual_ipaddresses.


function _generate_vrrpd_instance_config() {
  local instancetype=${1:-"master"}

  local routername=${HA_ROUTER_NAME:-"$DEFAULT_ROUTER_NAME"}
  local vipname="${routername//-/_}"
  local interface=${INTERFACE:-"$DEFAULT_INTERFACE"}
  local preempt=${PREEMPTION:-"$DEFAULT_PREEMPTION_STRATEGY"}

  local groups=${SLAVE_GROUPS[@]}
  local initialstate=""
  local priority="$VRRP_SLAVE_PRIORITY"

  if [ "$instancetype" = "master" ]; then
    groups=${PRIMARY_GROUPS[@]}
    initialstate="state MASTER"
    priority=$((priority * 2))
  fi


  for gid in $groups; do
    local ips=${ROUTER_VIPS[$gid]}

    echo "
vrrp_instance ${vipname}_VIP_$gid {
   interface $interface
   $initialstate
   virtual_router_id $gid
   priority $priority
   $preempt

$(_generate_authentication_info)

$(_generate_track_script)

$(_get_mucast_options)

$(_get_virtual_ipaddresses "$ips")
}
"
  done

}  #  End of function  _generate_vrrpd_instance_config.


function _generate_config() {
  echo "! Configuration File for keepalived

$(_generate_global_config)

$(_generate_script_config)

$(_generate_vrrpd_instance_config master)
$(_generate_vrrpd_instance_config slave)
"

}  #  End of function  _generate_config.


function _write_config() {
  echo "  - Generating and writing config to $KEEPALIVED_CONFIG"
  _generate_config > "$KEEPALIVED_CONFIG"
  
}  #  End of function  _write_config.


function _start_services() {
  echo "  - Starting keepalived service (+ enabling autostart) ..."
  service keepalived stop || :
  service keepalived start
  chkconfig keepalived on

}  #  End of function  _start_services.


#
#  main():
#
source "${1:-"$SAMPLE_CONFIG"}"

_install_packages

_write_config

_start_services
