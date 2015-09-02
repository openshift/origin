#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source $(dirname "${BASH_SOURCE}")/../../vagrant/provision-config.sh

NUM_NODES=${NUM_MINIONS:-2}
NODE_IPS=(${MINION_IPS//,/ })
HOST_NAME=${5:-""}
NETWORK_PLUGIN=${6:-${OPENSHIFT_SDN:-""}}

NODE_PREFIX="${INSTANCE_PREFIX}-node-"
NODE_NAMES=( $(eval echo ${NODE_PREFIX}{1..${NUM_NODES}}) )

DOCKER_CMD=${DOCKER_CMD:-"sudo docker"}

DEPLOYED_ROOT="/data"
SCRIPT_ROOT="${DEPLOYED_ROOT}/hack/dind"

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
