#!/bin/bash
#
# This library holds utility functions used by dind deployment and images.  Since
# it is intended to be distributed standalone in dind images, it cannot depend
# on any functions outside of this file.

# os::util::wait-for-condition blocks until the provided condition becomes true
#
# Globals:
#  None
# Arguments:
#  - 1: message indicating what conditions is being waited for (e.g. 'config to be written')
#  - 2: a string representing an eval'able condition.  When eval'd it should not output
#       anything to stdout or stderr.
#  - 3: optional timeout in seconds.  If not provided, waits forever.
# Returns:
#  1 if the condition is not met before the timeout
function os::util::wait-for-condition() {
  local msg=$1
  # condition should be a string that can be eval'd.
  local condition=$2
  local timeout=${3:-}

  local start_msg="Waiting for ${msg}"
  local error_msg="[ERROR] Timeout waiting for ${msg}"

  local counter=0
  while ! ${condition} >& /dev/null; do
    if [[ "${counter}" = "0" ]]; then
      echo "${start_msg}"
    fi

    if [[ -z "${timeout}" || "${counter}" -lt "${timeout}" ]]; then
      counter=$((counter + 1))
      if [[ -n "${timeout}" ]]; then
        echo -n '.'
      fi
      sleep 1
    else
      echo -e "\n${error_msg}"
      return 1
    fi
  done

  if [[ "${counter}" != "0" && -n "${timeout}" ]]; then
    echo -e '\nDone'
  fi
}
readonly -f os::util::wait-for-condition

# os::util::is-master indicates whether the host is configured to be an OpenShift master
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  1 if host is a master, 0 otherwise
function os::util::is-master() {
   test -f "/etc/systemd/system/openshift-master.service"
}
readonly -f os::util::is-master

# os::util::ensure-ipsec-config configures or deconfigures IPsec for the host
#
# Globals:
#  None
# Arguments:
#  - 1: the local machine hostname
#  - 2: the CA certificate for IPsec authentication
#  - 3: the client certificate for IPsec authentication
#  - 4: the client key for IPsec authentication
# Returns:
#  1 on error, 0 on success
function os::util::ensure-ipsec-config() {
  local host=$1
  local ca_file=$2
  local cert_file=$3
  local key_file=$4

  local private_policy_file="/etc/ipsec.d/policies/private"
  local clear_policy_file="/etc/ipsec.d/policies/clear"
  local conf_file="/etc/ipsec.d/openshift-cluster.conf"
  local ip_addr="$(ip addr | grep inet | grep eth0 | awk '{print $2}' | sed -e 's+/.*++')"
  local node_subnet="$(ip route | grep eth0 | grep ${ip_addr} | awk '{print $1}')"
  local defroute="$(ip route | grep default | awk '{print $3}')"

  # Should set OPENSHIFT_CLUSTER_IPSEC
  source /data/openshift-cluster-ipsec

  # Remove any existing configuration
  rm -f "${conf_file}"
  sed -i 's,'"${node_subnet}"',,g' "${private_policy_file}"
  sed -i '/^$/d' "${private_policy_file}"
  sed -i 's,'"${defroute}"'/32,,g' "${clear_policy_file}"
  sed -i '/^$/d' "${clear_policy_file}"

  if [[ "${OPENSHIFT_CLUSTER_IPSEC}" != "yes" ]]; then
    systemctl enable ipsec
    systemctl restart ipsec
    return
  fi

  # libreswan/NSS only import PKCS#12 files so create one
  local auth_dirname="$(dirname ${cert_file})"
  local p12_file="${auth_dirname}/master-certs.p12"
  openssl pkcs12 -export \
    -in "${cert_file}" \
    -inkey "${key_file}" \
    -certfile "${ca_file}" \
    -passout pass: \
    -out "${p12_file}"

  # libreswan uses the certificate subject Common Name (CN) to reference the
  # certificate in configuration
  local cert_nickname="$(openssl x509 -in ${cert_file} -subject -noout | sed -n 's/.*CN=\(.*\)/\1/p')"

  # import the p12 into the libreswan NSS database
  mkdir -p /etc/ipsec.d
  ipsec initnss
  pk12util -i "${p12_file}" -d "sql:/etc/ipsec.d" -W ""

  cat > "${conf_file}" <<EOF
conn private
	left=%defaultroute
	leftid=%fromcert
	# our certificate
	leftcert="NSS Certificate DB:${cert_nickname}"
	right=%opportunisticgroup
	rightid=%fromcert
	# their certificate transmitted via IKE
	rightca=%same
	ikev2=insist
	authby=rsasig
	failureshunt=drop
	negotiationshunt=hold
	auto=ondemand

conn clear
	left=%defaultroute
	right=%group
	authby=never
	type=passthrough
	auto=route
	priority=100
EOF

  # Add the node's subnet to the list of subnets to require encryption for
  echo "${node_subnet}" >> "${private_policy_file}"
  # But also exclude the docker bridge so clients can manage the cluster from the host
  echo "${defroute}/32" >> "${clear_policy_file}"

  systemctl enable ipsec
  systemctl restart ipsec
}
readonly -f os::util::ensure-ipsec-config
