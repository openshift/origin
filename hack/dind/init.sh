#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source $(dirname "${BASH_SOURCE}")/../../contrib/vagrant/provision-config.sh

NUM_NODES=${NUM_MINIONS:-2}
NODE_IPS=(${MINION_IPS//,/ })
HOST_NAME=${5:-""}
NETWORK_PLUGIN=${6:-${OPENSHIFT_SDN:-""}}

NODE_PREFIX="${INSTANCE_PREFIX}-node-"
NODE_NAMES=( $(eval echo ${NODE_PREFIX}{1..${NUM_NODES}}) )
SDN_NODE_NAME="${INSTANCE_PREFIX}-master-sdn"

DOCKER_CMD=${DOCKER_CMD:-"sudo docker"}

DEPLOYED_ROOT="/data"
SCRIPT_ROOT="${DEPLOYED_ROOT}/hack/dind"
SUPERVISORD_CONF="/etc/supervisord.conf"

CONFIG_ROOT=${OS_DIND_CONFIG_ROOT:-/tmp/openshift-dind-cluster/${INSTANCE_PREFIX}}
DEPLOYED_CONFIG_ROOT="/config"

os::dind::set-dind-env() {
  # Set up the KUBECONFIG environment variable for use by oc
  local deployed_root=$1
  local config_root=$2

  # Target .bashrc by default instead of .bash_profile because a
  # 'docker exec' invocation will not run .bash_profile
  local target=${3:-"/root/.bashrc"}

  local log_target='/var/log/supervisor/openshift-*-stderr-*'
  os::util::set-oc-env "${config_root}" "${target}"
  cat <<EOF >> "${target}"
alias oc-less-log="less ${log_target}"
alias oc-tail-log="tail -f ${log_target}"
alias oc-create-hello="oc create -f ${deployed_root}/examples/hello-openshift/hello-pod.json"
EOF
}

os::dind::reload-docker() {
  # Ensure that openshift-sdn has written configuration for docker
  # before triggering a docker restart.
  echo "Waiting for openshift-sdn to update supervisord.conf with docker config"
  local counter=0
  local timeout=30
  while grep -q 'DOCKER_DAEMON_ARGS=\"\"' "${SUPERVISORD_CONF}"; do
    if [[ "${counter}" -lt "${timeout}" ]]; then
      counter=$((counter + 1))
      echo -n '.'
      sleep 1
    else
      echo -e "\n[ERROR] Timeout waiting for openshift-sdn to update supervisord.conf"
      exit 1
    fi
  done
  echo -e '\nDone'

  # Stop docker gracefully
  ${SCRIPT_ROOT}/kill-docker.sh

  # Restart docker
  supervisorctl update
}

os::dind::is-sdn-node-registered() {
  local master_cid=$1
  local node_name=$2

  ${DOCKER_CMD} exec -t "${master_cid}" bash -ci \
    "oc get nodes ${node_name} &> /dev/null"
}

os::dind::disable-sdn-node() {
  local master_cid=$1
  local node_name=$2

  local sdn_msg="for sdn node to register with the master"
  local start_msg="Waiting ${sdn_msg}"
  local error_msg="[ERROR] Timeout waiting ${sdn_msg}"
  local condition="os::dind::is-sdn-node-registered ${master_cid} ${node_name}"
  local timeout=30
  os::util::wait-for-condition "${start_msg}" "${error_msg}" "${condition}" \
    "${timeout}"

  echo "Disabling scheduling for the sdn node"
  # Disable scheduling outside of the master provision script to give
  # the node time to register itself to the master.
  ${DOCKER_CMD} exec -t "${master_cid}" bash -ci \
    "osadm manage-node ${node_name} --schedulable=false > /dev/null"
}
