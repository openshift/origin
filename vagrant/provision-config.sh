#!/bin/bash
set -e

# Passed as arguments to provisioning from Vagrantfile
MASTER_IP=$1
NUM_MINIONS=$2
MINION_IPS=$3

INSTANCE_PREFIX=openshift
MASTER_NAME="${INSTANCE_PREFIX}-master"
MINION_NAMES=($(eval echo ${INSTANCE_PREFIX}-minion-{1..${NUM_MINIONS}}))
