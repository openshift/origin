#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ORIGIN_ROOT=$(
  unset CDPATH
  origin_root=$(dirname "${BASH_SOURCE}")/..
  cd "${origin_root}"
  pwd
)
source ${ORIGIN_ROOT}/vagrant/provision-util.sh

# Passed as arguments to provisioning from Vagrantfile
MASTER_IP=${1:-""}
NUM_MINIONS=${2:-""}
MINION_IPS=${3:-""}

INSTANCE_PREFIX=openshift
MASTER_NAME="${INSTANCE_PREFIX}-master"
MASTER_TAG="${INSTANCE_PREFIX}-master"
MINION_TAG="${INSTANCE_PREFIX}-minion"
MINION_NAMES=($(eval echo ${INSTANCE_PREFIX}-minion-{1..${NUM_MINIONS}}))
MINION_IP_RANGES=($(eval echo "10.245.{2..${NUM_MINIONS}}.2/24"))
MINION_SCOPES=""

MASTER_USER=vagrant
MASTER_PASSWD=vagrant
