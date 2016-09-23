#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
# Should set OPENSHIFT_NETWORK_PLUGIN
source /data/network-plugin

function ensure-node-config() {
  local deployed_config_path="/var/lib/origin/openshift.local.config/node"
  local deployed_config_file="${deployed_config_path}/node-config.yaml"

  if [[ -f "${deployed_config_file}" ]]; then
    # Config has already been deployed
    return
  fi

  local config_path="/data/openshift.local.config"
  local host
  host="$(hostname)"
  if [[ -f "/etc/systemd/system/openshift-master.service" ]]; then
    host="${host}-node"
  fi
  local node_config_path="${config_path}/node-${host}"
  local config_file="${node_config_path}/node-config.yaml"

  # If the node config has not been generated
  if [[ ! -f "${config_file}" ]]; then
    local master_config_path="${config_path}/master"

    # Wait for the master to generate its config
    local condition="test -f ${master_config_path}/admin.kubeconfig"
    os::util::wait-for-condition "admin config" "${condition}" "${OS_WAIT_FOREVER}"

    local master_host
    master_host="$(grep server "${master_config_path}/admin.kubeconfig" | grep -v localhost | awk '{print $2}')"

    local ip_addr
    ip_addr="$(ip addr | grep inet | grep eth0 | awk '{print $2}' | sed -e 's+/.*++')"

    /usr/local/bin/openshift admin create-node-config \
      --node-dir="${config_path}" \
      --node="${host}" \
      --master="${master_host}" \
      --hostnames="${host},${ip_addr}" \
      --network-plugin="${OPENSHIFT_NETWORK_PLUGIN}" \
      --node-client-certificate-authority="${master_config_path}/ca.crt" \
      --certificate-authority="${master_config_path}/ca.crt" \
      --signer-cert="${master_config_path}/ca.crt" \
      --signer-key="${master_config_path}/ca.key" \
      --signer-serial="${master_config_path}/ca.serial.txt"
  fi

  # Deploy the node config
  mkdir -p "${deployed_config_path}"
  cp -r "${config_path}"/* "${deployed_config_path}"
}

ensure-node-config
