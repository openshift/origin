#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
# Should set OPENSHIFT_NETWORK_PLUGIN
source /data/dind-env

function have-token() {
  local master_dir=$1

  [[ -s "${master_dir}/openshift-sdn.token" ]]
}

function openshift-sdn-node() {
  local config_dir=$1
  local master_dir=$2
  local kube_config="${config_dir}/node.kubeconfig"
  local sdn_kube_config="${config_dir}/sdn-node.kubeconfig"

  os::util::wait-for-condition "kubernetes token" "have-token ${master_dir}" "120"
  token=$(cat ${master_dir}/openshift-sdn.token)

  # Take over network functions on the node
  rm -Rf /etc/cni/net.d/80-openshift-network.conf

  # use either the bootstrapped node kubeconfig or the static configuration
  if [[ ! -f "${kube_config}" ]]; then
    # use the static node config if it exists
    # TODO: remove when static node configuration is no longer supported
    for f in ${config_dir}/system*.kubeconfig; do
      echo "info: Using ${f} for node configuration" 1>&2
      kube_config="${f}"
      break
    done
  fi
  # Hard fail if we still don't have a kubeconfig
  [[ -f "${kube_config}" ]]

  if [[ ! -f "${sdn_kube_config}" ]]; then
    # Use the same config as the node, but with the service account token
    oc config --config=${kube_config} view --flatten > ${sdn_kube_config}
    oc config --config=${sdn_kube_config} set-credentials sa "--token=${token}"
    oc config --config=${sdn_kube_config} set-context "$( oc config --config=${sdn_kube_config} current-context )" --user=sa
  fi
  # Launch the network process
  exec openshift start network --config=${config_dir}/node-config.yaml --kubeconfig=${sdn_kube_config} --loglevel=${DEBUG_LOGLEVEL:-4}
}

if [[ "${OPENSHIFT_NETWORK_PLUGIN}" =~ ^"redhat/" ]]; then
  openshift-sdn-node /var/lib/origin/openshift.local.config/node /data/openshift.local.config/master
fi
