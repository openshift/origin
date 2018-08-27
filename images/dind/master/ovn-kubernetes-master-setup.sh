#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
source /data/dind-env

function ensure-token() {
  local token_file="${1}/ovn.token"
  local kube_config=$2

  /usr/local/bin/oc --config="${kube_config}" sa get-token ovn > ${token_file}
  [[ -s "${token_file}" ]]
}

function ovn-kubernetes-master-setup() {
  local config_dir=$1
  local kube_config="${config_dir}/admin.kubeconfig"

  os::util::wait-for-apiserver "${kube_config}"

  systemctl enable ovn-northd
  systemctl start ovn-northd

  ln -sf /data/ovnkube /usr/local/bin/
  ln -sf /data/ovn-kube-util /usr/local/bin/
  ln -sf /data/ovn-k8s-cni-overlay /usr/local/bin/
  ln -sf /data/ovn-k8s-gateway-helper /usr/local/bin/
  ln -sf /data/ovn-k8s-util /usr/local/bin/
  ln -sf /data/ovn-k8s-watcher /usr/local/bin/
  mkdir -p /usr/lib/python2.7/site-packages
  ln -sf /data/ovn_k8s /usr/lib/python2.7/site-packages/

  # Create the service account for OVN stuff
  if ! /usr/local/bin/oc --config="${kube_config}" get serviceaccount ovn >/dev/null 2>&1; then
    /usr/local/bin/oc --config="${kube_config}" create serviceaccount ovn
    /usr/local/bin/oc --config="${kube_config}" adm policy add-cluster-role-to-user cluster-admin -z ovn

    # rhbz#1383707: need to add ovn SA to anyuid SCC to allow pod annotation updates
    os::util::wait-for-anyuid "${kube_config}"
    /usr/local/bin/oc --config="${kube_config}" adm policy add-scc-to-user anyuid -z ovn
  fi

  os::util::wait-for-condition "kubernetes token" "ensure-token ${config_dir} ${kube_config}" "120"
}

if [[ -n "${OPENSHIFT_OVN_KUBERNETES}" ]]; then
  ovn-kubernetes-master-setup /data/openshift.local.config/master
fi
