#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
source /data/dind-env

function is-api-running() {
  local config=$1

  /usr/local/bin/oc --config="${kube_config}" get --raw /healthz/ready &> /dev/null
}

function ovn-kubernetes-node-setup() {
  local config_dir=$1
  local kube_config="${config_dir}/node.kubeconfig"

  local msg="apiserver to become alive"
  os::util::wait-for-condition "${msg}" "is-api-running ${kube_config}"

  ln -sf /data/ovnkube /usr/local/bin/
  ln -sf /data/ovn-kube-util /usr/local/bin/
  ln -sf /data/ovn-k8s-cni-overlay /opt/cni/bin/
  ln -sf /data/ovn-k8s-gateway-helper /usr/local/bin/
  ln -sf /data/ovn-k8s-overlay /usr/local/bin
  ln -sf /data/ovn-k8s-util /usr/local/bin/
  ln -sf /data/ovn-k8s-watcher /usr/local/bin/
  mkdir -p /usr/lib/python2.7/site-packages
  ln -sf /data/ovn_k8s /usr/lib/python2.7/site-packages/
}

if [[ -n "${OPENSHIFT_OVN_KUBERNETES}" ]]; then
  ovn-kubernetes-node-setup /var/lib/origin/openshift.local.config/node/
fi
