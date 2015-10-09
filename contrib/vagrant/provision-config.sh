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

# Passed as arguments to provisioning from Vagrantfile
MASTER_IP=${1:-""}
NUM_MINIONS=${2:-""}
MINION_IPS=${3:-""}
INSTANCE_PREFIX=${4:-${OS_INSTANCE_PREFIX:-openshift}}

MASTER_NAME="${INSTANCE_PREFIX}-master"
MINION_NAMES=($(eval echo ${INSTANCE_PREFIX}-minion-{1..${NUM_MINIONS}}))
