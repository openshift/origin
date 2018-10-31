#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Should set OPENSHIFT_NETWORK_PLUGIN
source /data/dind-env

function ensure-master-config() {
  local config_path="/data/openshift.local.config"
  local master_path="${config_path}/master"
  local config_file="${master_path}/master-config.yaml"
  local sdn_config_file="${master_path}/sdn-config.yaml"

  if [[ -f "${config_file}" ]]; then
    # Config has already been generated
    return
  fi

  local name
  name="$(hostname)"

  local ip_addr1
  ip_addr1="$(ip addr | grep inet | grep eth0 | awk '{print $2}' | sed -e 's+/.*++')"

  local ip_addr2
  ip_addr2="$(ip addr | grep inet | (grep eth1 || true) | awk '{print $2}' | sed -e 's+/.*++')"

  local ip_addrs
  local serving_ip_addr
  if [[ -n "${ip_addr2}" ]]; then
    ip_addrs="${ip_addr1},${ip_addr2}"
    serving_ip_addr="${ip_addr2}"
  else
    ip_addrs="${ip_addr1}"
    serving_ip_addr="${ip_addr1}"
  fi

  local image_format=${OPENSHIFT_IMAGE_FORMAT:-}
  local image_format_str=""
  if [[ -n "${image_format}" ]]; then
    image_format_str="--images=${image_format}"
  fi

  mkdir -p "${config_path}"
  (flock 200;
   /usr/local/bin/oc adm ca create-master-certs \
     --overwrite=false \
     --cert-dir="${master_path}" \
     --master="https://${serving_ip_addr}:8443" \
     --hostnames="${ip_addrs},${name}"

   /usr/local/bin/openshift start master --write-config="${master_path}" \
     --master="https://${serving_ip_addr}:8443" \
     ${image_format_str} \
     ${OPENSHIFT_ADDITIONAL_ARGS}

   mv "${config_file}" "${config_file}.bak"
   oc patch --config="${master_path}/admin.kubeconfig" --local --type=json -o yaml -f "${config_file}.bak" --patch='[{"op": "add", "path": "/controllerConfig/controllers/0", "value": "-openshift.io/sdn"}]' > "${config_file}"

   cat > "${sdn_config_file}" <<EOF
kind: OpenShiftControllerManagerConfig
apiVersion: openshiftcontrolplane.config.openshift.io/v1
kubeClientConfig:
  kubeConfig: ${master_path}/admin.kubeconfig
network:
  networkPluginName: ${OPENSHIFT_NETWORK_PLUGIN}
  clusterNetworks:
  - cidr: 10.128.0.0/14
    hostSubnetLength: 9
  serviceNetworkCIDR: 172.30.0.0/16
  vxLANPort: 4789
  vxlanPort: 4789
EOF
  ) 200>"${config_path}"/.openshift-ca.lock

  # ensure the configuration can be used outside of the container
  chmod -R ga+rX "${master_path}"
  chmod ga+w "${master_path}/admin.kubeconfig"
}

ensure-master-config
