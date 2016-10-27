#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Should set OPENSHIFT_NETWORK_PLUGIN
source /data/network-plugin

function ensure-master-config() {
  local config_path="/data/openshift.local.config"
  local master_path="${config_path}/master"
  local config_file="${master_path}/master-config.yaml"

  if [[ -f "${config_file}" ]]; then
    # Config has already been generated
    return
  fi

  local ip_addr
  ip_addr="$(ip addr | grep inet | grep eth0 | awk '{print $2}' | sed -e 's+/.*++')"
  local name
  name="$(hostname)"

  /usr/local/bin/openshift admin ca create-master-certs \
    --overwrite=false \
    --cert-dir="${master_path}" \
    --master="https://${ip_addr}:8443" \
    --hostnames="${ip_addr},${name}"

  /usr/local/bin/openshift start master --write-config="${master_path}" \
    --master="https://${ip_addr}:8443" \
    --network-plugin="${OPENSHIFT_NETWORK_PLUGIN}"

  # ensure the configuration can be used outside of the container
  chmod -R ga+rX "${master_path}"
  chmod ga+w "${master_path}/admin.kubeconfig"
}

ensure-master-config
