#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh

function is-node-registered() {
  local config=$1
  local node_name=$2

  /usr/local/bin/oc --config="${config}" get nodes "${node_name}" &> /dev/null
}

function disable-node() {
  local config=$1
  local node_name=$2

  local msg="${node_name} to register with the master"
  local condition="is-node-registered ${config} ${node_name}"
  os::util::wait-for-condition "${msg}" "${condition}"

  echo "Disabling scheduling for node ${node_name}"
  /usr/local/bin/osadm --config="${config}" manage-node "${node_name}" --schedulable=false > /dev/null
}

disable-node /data/openshift.local.config/master/admin.kubeconfig "$(hostname)-node"
