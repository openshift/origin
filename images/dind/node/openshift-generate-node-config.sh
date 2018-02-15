#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
# Should set OPENSHIFT_NETWORK_PLUGIN
source /data/dind-env

function ensure-node-config() {
  local deployed_config_path="/var/lib/origin/openshift.local.config/node"
  local deployed_config_file="${deployed_config_path}/node-config.yaml"

  local config_path="/data/openshift.local.config"
  local host
  host="$(hostname)"
  if os::util::is-master; then
    host="${host}-node"
  fi
  local node_config_path="${config_path}/node-${host}"
  local node_config_file="${node_config_path}/node-config.yaml"

  if [[ -f "${deployed_config_file}" && -f "${node_config_file}" ]]; then
    # Config has already been deployed and they have not removed the node config to indicate a regen is needed
    return
  fi

  # If the node config has not been generated
  if [[ ! -f "${node_config_file}" ]]; then
    local master_config_path="${config_path}/master"
    local master_config_file="${master_config_path}/admin.kubeconfig"

    # Wait for the master to generate its config
    local condition="test -f ${master_config_file}"
    os::util::wait-for-condition "admin config" "${condition}"

    local master_host
    master_host="$(grep server "${master_config_file}" | grep -v localhost | awk '{print $2}')"

    local ip_addr1
    ip_addr1="$(ip addr | grep inet | grep eth0 | awk '{print $2}' | sed -e 's+/.*++')"

    local ip_addr2
    ip_addr2="$(ip addr | grep inet | (grep eth1 || true) | awk '{print $2}' | sed -e 's+/.*++')"

    local ip_addrs
    if [[ -n "${ip_addr2}" ]]; then
      ip_addrs="${ip_addr1},${ip_addr2}"
    else
      ip_addrs="${ip_addr1}"
    fi

    # Hold a lock on the shared volume to ensure cert generation is
    # performed serially.  Cert generation is not compatible with
    # concurrent execution since the file passed to --signer-serial
    # needs to be incremented by each invocation.
    (flock 200;
     /usr/local/bin/oc adm create-node-config \
       --node-dir="${node_config_path}" \
       --node="${host}" \
       --master="${master_host}" \
       --hostnames="${host},${ip_addrs}" \
       --network-plugin="${OPENSHIFT_NETWORK_PLUGIN}" \
       --node-client-certificate-authority="${master_config_path}/ca.crt" \
       --certificate-authority="${master_config_path}/ca.crt" \
       --signer-cert="${master_config_path}/ca.crt" \
       --signer-key="${master_config_path}/ca.key" \
       --signer-serial="${master_config_path}/ca.serial.txt"
    ) 200>"${config_path}"/.openshift-generate-node-config.lock

    cat >> "${node_config_file}" <<EOF
kubeletArguments:
  cgroups-per-qos: ["false"]
  enforce-node-allocatable: [""]
  fail-swap-on: ["false"]
  eviction-soft: [""]
  eviction-hard: [""]
EOF

    if [[ "${OPENSHIFT_CONTAINER_RUNTIME}" != "dockershim" ]]; then
      cat >> "${node_config_file}" <<EOF
  container-runtime: ["remote"]
  container-runtime-endpoint: ["${OPENSHIFT_REMOTE_RUNTIME_ENDPOINT}"]
  image-service-endpoint: ["${OPENSHIFT_REMOTE_RUNTIME_ENDPOINT}"]
EOF
    fi

  fi

  # Ensure the configuration is readable outside of the container
  chmod -R ga+rX "${node_config_path}"

  # Remove any old config in case we are reloading
  if [[ -d "${deployed_config_path}" ]]; then
      rm -rf "${deployed_config_path}"
  fi

  # Deploy the node config
  mkdir -p "${deployed_config_path}"
  cp -r "${node_config_path}"/* "${deployed_config_path}/"
}

ensure-node-config
