#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ORIGIN_ROOT=$(
  unset CDPATH
  origin_root=$(dirname "${BASH_SOURCE}")/../..
  cd "${origin_root}"
  pwd
)
source ${ORIGIN_ROOT}/contrib/vagrant/provision-util.sh

# Passed as arguments to provisioning script
MASTER_IP=${1:-""}
NODE_COUNT=${2:-${OPENSHIFT_NUM_MINIONS:-2}}
NODE_IPS=${3:-""}
INSTANCE_PREFIX=${4:-${OPENSHIFT_INSTANCE_PREFIX:-openshift}}

# Set defaults for optional arguments
FIXUP_NET_UDEV=false
NETWORK_PLUGIN=${OPENSHIFT_NETWORK_PLUGIN:-""}
NODE_INDEX=0
CONFIG_ROOT=${ORIGIN_ROOT}

# Parse optional arguments
# Skip the positional arguments
OPTIND=5
while getopts ":i:n:c:f" opt; do
  case $opt in
    f)
      FIXUP_NET_UDEV=true
      ;;
    i)
      NODE_INDEX=${OPTARG}
      ;;
    n)
      NETWORK_PLUGIN=${OPTARG}
      ;;
    c)
      CONFIG_ROOT=${OPTARG}
      ;;
    \?)
      echo "Invalid option: -${OPTARG}" >&2
      exit 1
      ;;
    :)
      echo "Option -${OPTARG} requires an argument." >&2
      exit 1
      ;;
  esac
done

LOG_LEVEL=${OPENSHIFT_LOG_LEVEL:-5}

NODE_IPS=(${NODE_IPS//,/ })
if [ "${CONFIG_ROOT}" = "/" ]; then
  CONFIG_ROOT=""
fi
NETWORK_PLUGIN=$(os::provision::get-network-plugin "${NETWORK_PLUGIN}" \
  "${DIND_MANAGEMENT_SCRIPT:-false}")
MASTER_NAME="${INSTANCE_PREFIX}-master"
NODE_PREFIX="${INSTANCE_PREFIX}-node-"
NODE_NAMES=( $(eval echo ${NODE_PREFIX}{1..${NODE_COUNT}}) )
SDN_NODE_NAME="${INSTANCE_PREFIX}-master-sdn"
