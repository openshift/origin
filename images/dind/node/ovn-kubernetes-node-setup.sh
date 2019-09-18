#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
source /data/dind-env

function ovn-kubernetes-node-setup() {
  local config_dir=$1
  local kube_config="${config_dir}/node.kubeconfig"

  os::util::wait-for-apiserver "${kube_config}"

  ln -sf /data/ovnkube /usr/local/bin/
  ln -sf /data/ovn-kube-util /usr/local/bin/
  ln -sf /data/ovn-k8s-cni-overlay /opt/cni/bin/
}

if [[ -n "${OPENSHIFT_OVN_KUBERNETES}" ]]; then
  ovn-kubernetes-node-setup /var/lib/origin/openshift.local.config/node/
fi
